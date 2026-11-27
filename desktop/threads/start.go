package threads

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/term"
	"github.com/nednella/orchestrator/internal/thread"
	"github.com/nednella/orchestrator/internal/util"
)

// StartState says how far the start of a new thread has got, for a thread whose row came before its agent.
type StartState string

const (
	Starting       StartState = "starting"  // the worktree, the host or the first message is still on its way
	StartFailed    StartState = "failed"    // a step failed; RetryStart goes on from it
	StartCancelled StartState = "cancelled" // the owner stopped it; RetryStart goes on from the step it stopped at
)

// starting is a thread being started: its worktree, its host and its first message come after its row, which is in
// the list from the first moment. The fields under "progress" belong to the goroutine that runs the start; the
// fields under "shown" are read by the list and the refresh, under mu.
type starting struct {
	proj            project.Project
	name            thread.Name
	rawTitle        string // the title as asked for, which names the branch
	title           string
	text            string
	send            bool
	issue           int
	brief, handover string
	action          string
	fork            forkStart
	o               StartOptions
	p               agent.Provider
	model           project.Model
	created         time.Time
	app             context.Context // the app's, which the first message and the watchers outlive a retry on

	// progress
	work       workPlace
	branch     string // the branch a no-issue start picked
	placed     bool
	making     string // the folder git began to make the worktree in, which a quit may have cut short before it was recorded
	note       string // what the owner should know of how the worktree was made, which goes into the chat before the agent starts
	noted      bool   // the note is in the chat
	setupTried bool   // the set-up command was looked at in this try: a retry looks again
	restore    func()
	delivered  bool
	saved      []byte // what the start's record on disk holds
	sending    string // what a try handed to the host and has no reply for: sendingFirst or sendingHeld, else ""
	sentBefore int    // how many such messages the host's chat log held then

	// shown, under mu
	info      term.Info
	status    StartState
	failure   string
	running   bool
	launched  bool
	aborted   bool
	cancelled bool // the owner stopped the start
	cancel    context.CancelFunc
	done      chan struct{}
	step      string         // the step that runs, for the row
	queue     []host.Message // what the owner sent while it started, oldest first
}

// shownText is the first message as the thread shows it while it starts: what the owner sent, and nothing when it is
// only left as a draft.
func (st *starting) shownText() string {
	if !st.send {
		return ""
	}
	return strings.TrimPrefix(st.text, AppPrompt)
}

func (st *starting) placeholder() term.Info {
	in := term.Info{Name: st.name, Title: st.title, Model: st.model.Model, Effort: st.model.Effort, Fast: st.model.Fast, Access: st.o.Access, Action: st.action, Fork: st.fork.of, Side: st.fork.side, Parent: st.o.Parent, Created: st.created}
	if st.issue > 0 {
		in.Issue = strconv.Itoa(st.issue)
	}
	if st.o.Place == InWorktree {
		if st.issue > 0 {
			in.Branch = st.proj.BranchFor(st.issue)
		}
	} else {
		in.Path = cmp.Or(st.o.dir, st.proj.Dir)
	}
	return in
}

// begin makes the record of a thread to start: its title, its model and its name.
func (s *Threads) begin(proj project.Project, title, text string, send bool, issue int, brief, handover, action string, fork forkStart, o StartOptions) (*starting, error) {
	p := s.Provider()
	if err := s.agents.CheckInstalled(p); err != nil {
		return nil, err
	}
	name, app := s.reserve(proj)
	return &starting{
		proj: proj, name: name, rawTitle: title, title: cmp.Or(strings.TrimSpace(title), defaultTitle), text: text, send: send, issue: issue,
		brief: brief, handover: handover, action: action, fork: fork, o: o, p: p, model: fill(p, proj.Dir, o.Model), created: time.Now(), app: app,
		restore: func() {},
	}, nil
}

// startNow starts the thread before it returns, as a child, a fork and a side chat need.
func (s *Threads) startNow(st *starting) (Thread, error) {
	s.createMu.Lock()
	defer s.createMu.Unlock()
	ctx := st.app
	if err := s.execute(ctx, st); err != nil {
		return Thread{}, err
	}
	return s.viewOf(st.name.String())
}

// startLater lists the thread at once and starts it on a goroutine: the row says it starts, and shows the first
// message as sent. It returns the row.
func (s *Threads) startLater(st *starting) Thread {
	s.mu.Lock()
	defer s.mu.Unlock()
	st.info = st.placeholder()
	st.status, st.running = Starting, true
	st.step = "Starting Claude"
	if st.o.Place == InWorktree {
		st.step = "Setting up the worktree"
	}
	st.done = make(chan struct{})
	var run context.Context
	run, st.cancel = context.WithCancel(st.app)
	s.starting[st.name.String()] = st
	s.info = append(s.info, st.info)
	s.number()
	s.sync()
	go s.runStart(run, st)
	for _, v := range s.list() {
		if v.ID == st.name.String() {
			return v
		}
	}
	return Thread{}
}

// runStart runs the steps of a start that is listed, and tells the thread's row how it went.
func (s *Threads) runStart(ctx context.Context, st *starting) {
	defer close(st.done)
	for {
		err := s.execute(ctx, st)
		s.mu.Lock()
		if err == nil && len(st.queue) > 0 {
			// A message that came after the last delivery.
			s.mu.Unlock()
			continue
		}
		cancelled := err != nil && st.cancelled && !st.aborted
		kept := ""
		if cancelled {
			s.mu.Unlock()
			kept = s.undoCancelled(st)
			s.mu.Lock()
		}
		// The start ends under the lock that found no message held, so one that comes next is not lost: it is told
		// that the thread runs, and goes to its agent.
		st.running = false
		switch {
		case err == nil:
			delete(s.starting, st.name.String())
			s.dropStartRecord(st.name)
		// A start the app's quit stopped is saved as it was, and listed as failed when the app starts again.
		case st.aborted || st.app.Err() != nil:
			s.mu.Unlock()
			return
		case cancelled:
			st.status, st.failure = StartCancelled, "Start cancelled"
			if kept != "" {
				st.failure += ". The worktree stays: " + kept
			}
		default:
			log.Printf("orchestrator: thread %s did not start: %v", st.name, err)
			st.status, st.failure = StartFailed, startFailure(err)
		}
		s.sync()
		s.mu.Unlock()
		return
	}
}

// undoCancelled stops the set-up command of a start the owner stopped, and removes the worktree it made when its agent
// does not run and the checks of a clean-up pass. It says why the worktree stays, "" when it went or there was none.
func (s *Threads) undoCancelled(st *starting) string {
	s.stopSetup(st.name.String())
	if st.launched {
		return ""
	}
	if reason := s.dropWorktreeOf(context.WithoutCancel(st.app), st); reason != "" {
		return reason
	}
	s.progress(st, func() {
		st.work, st.placed = workPlace{}, false
		st.info = st.placeholder()
		if i := slices.IndexFunc(s.info, func(in term.Info) bool { return in.Name == st.name }); i >= 0 && !st.launched {
			s.info[i] = st.info
		}
	})
	return ""
}

// dropWorktreeOf removes the worktree the start made, and says why it stays: see dropStartWorktree.
func (s *Threads) dropWorktreeOf(ctx context.Context, st *starting) string {
	s.mu.Lock()
	branch, making := st.branch, st.making
	s.mu.Unlock()
	if st.issue > 0 {
		branch = st.proj.BranchFor(st.issue)
	}
	return s.life.dropStartWorktree(ctx, st.proj, st.name, st.title, branch, making)
}

// startFailure is why a start failed, in a sentence for the thread's row: the host's own reason when it refused, else
// the first line of the error.
func startFailure(err error) string {
	if refused := (*host.Refused)(nil); errors.As(err, &refused) {
		return refused.Reason
	}
	return util.FirstLine(err.Error())
}

// What a try of a start was sending when it ended without the host's reply.
const (
	sendingFirst = "first"
	sendingHeld  = "held"
)

// errNotDelivered is the error of a first message the agent did not take.
var errNotDelivered = errors.New("the agent did not take the first message in time")

// execute does each step of a start that has not been done yet, so a retry goes on where the start stopped. A start
// that is listed keeps its worktree when a step fails, for the retry; one that is not has no retry, so its worktree goes.
func (s *Threads) execute(ctx context.Context, st *starting) error {
	if st.placed && st.o.Later && st.work.made && gone(st.work.dir) {
		s.progress(st, func() { st.work, st.placed = workPlace{}, false })
	}
	if !st.placed {
		if st.o.Place == InWorktree {
			s.setStep(st, "Setting up the worktree")
		}
		if err := s.place(ctx, st); err != nil {
			return err
		}
	}
	s.mu.Lock()
	launched := st.launched
	s.mu.Unlock()
	if !launched {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.setStep(st, "Starting Claude")
		if err := s.boot(ctx, st); err != nil {
			if !st.o.Later {
				s.unplace(st)
			}
			return err
		}
	}
	// After the agent: tmux ends its server when its last session ends, and a set-up that ends at once, alone in it,
	// would take the server away from the agent's own start.
	s.startSetup(st)
	if st.work.branch != "" {
		s.life.ensureWatchers(st.app)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if st.text != "" && !st.delivered {
		if !st.o.Later {
			go s.giveHost(st.app, st.name, st.text, st.send)
			return nil
		}
		first := host.Message{Text: st.text}
		// A reply the app did not get may have come after the host took the message: it is sent once, ever. A draft
		// replaces the draft it left, so it may go again.
		if st.send && s.hostTook(st, sendingFirst, first) {
			s.progress(st, func() { st.delivered, st.sending = true, "" })
		} else {
			before := s.hostHolds(st.name, first)
			s.progress(st, func() { st.sending, st.sentBefore = sendingFirst, before })
			if err := toHost(ctx, s.hostSocket(st.name), st.text, st.send, hostWait); err != nil {
				log.Printf("orchestrator: thread %s did not get its first prompt: %v", st.name, err)
				if refused := (*host.Refused)(nil); errors.As(err, &refused) {
					s.progress(st, func() { st.sending = "" })
					return err
				}
				return errNotDelivered
			}
			s.progress(st, func() { st.delivered, st.sending = true, "" })
		}
	}
	return s.deliverQueued(ctx, st)
}

// setStep tells the thread's row which step of its start runs.
func (s *Threads) setStep(st *starting, step string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !st.o.Later {
		return
	}
	st.step = step
	s.sync()
}

// QueueDuringStart holds a message the owner sends to a thread whose start has not ended, to go to the agent after its
// first message, and says whether it did. The row shows it as queued.
func (s *Threads) QueueDuringStart(id string, m host.Message) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.starting[id]
	if st == nil {
		return false
	}
	st.queue = append(st.queue, m)
	s.sync()
	return true
}

// Starts says whether the thread's start has not ended.
func (s *Threads) Starts(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starting[id] != nil
}

// deliverQueued gives the agent the messages that came while it started, in order, until none is left. A message that
// is not taken stays at the head of the queue for a retry.
func (s *Threads) deliverQueued(ctx context.Context, st *starting) error {
	s.mu.Lock()
	held := len(st.queue) > 0
	s.mu.Unlock()
	if held {
		// With no first message nothing has waited for the host yet.
		ready, cancel := context.WithTimeout(ctx, hostWait)
		err := host.Ready(ready, s.hostSocket(st.name))
		cancel()
		if err != nil {
			log.Printf("orchestrator: thread %s did not get a message sent while it started: %v", st.name, err)
			return errNotDelivered
		}
	}
	for {
		s.mu.Lock()
		if len(st.queue) == 0 {
			s.mu.Unlock()
			return nil
		}
		next := st.queue[0]
		s.mu.Unlock()
		if !s.hostTook(st, sendingHeld, next) {
			before := s.hostHolds(st.name, next)
			s.progress(st, func() { st.sending, st.sentBefore = sendingHeld, before })
			if _, err := host.SendMessage(ctx, s.hostSocket(st.name), next); err != nil {
				log.Printf("orchestrator: thread %s did not get a message sent while it started: %v", st.name, err)
				if refused := (*host.Refused)(nil); errors.As(err, &refused) {
					s.progress(st, func() { st.sending = "" })
					return err
				}
				return errNotDelivered
			}
		}
		s.mu.Lock()
		st.queue, st.sending = st.queue[1:], ""
		s.sync()
		s.mu.Unlock()
	}
}

// progress changes what a start's own goroutine has done, which the list and a saved start read under mu.
func (s *Threads) progress(st *starting, change func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	change()
	s.sync()
}

// hostHolds is how many messages like m the thread's chat log, which its host keeps, holds as the user's or queued.
func (s *Threads) hostHolds(name thread.Name, m host.Message) int {
	events, err := chat.ReadLog(s.chatLogPath(name), 0, chat.KindUser, chat.KindQueued)
	if err != nil {
		return 0
	}
	ids := map[string]bool{}
	for _, e := range events {
		if e.Text == m.Text && e.From == m.From {
			ids[e.ID] = true
		}
	}
	return len(ids)
}

// hostTook says whether a try that handed m to the host and got no reply, as the start's record says, was in fact
// taken: the host's log holds more such messages than before the try.
func (s *Threads) hostTook(st *starting, what string, m host.Message) bool {
	return st.sending == what && s.hostHolds(st.name, m) > st.sentBefore
}

// place finds or makes the folder the thread runs in, and the chat a fork starts with. One place at a time, so two
// threads never pick the same new branch.
func (s *Threads) place(ctx context.Context, st *starting) error {
	s.treeMu.Lock()
	defer s.treeMu.Unlock()
	work := workPlace{dir: cmp.Or(st.o.dir, st.proj.Dir)}
	if st.o.Place == InWorktree {
		branch, base := st.proj.BranchFor(st.issue), ""
		if st.issue == 0 {
			// A retry keeps the branch the first try picked, which a worktree that was undone leaves behind.
			if st.branch == "" {
				branch, err := s.life.freeBranch(ctx, st.proj.Dir, branchFromTitle(st.rawTitle))
				if err != nil {
					return err
				}
				s.progress(st, func() { st.branch = branch })
			}
			branch, base = st.branch, st.o.Base
		}
		var err error
		try := worktreeTry{prior: st.making, begin: func(dir string) { s.progress(st, func() { st.making = dir }) }}
		if work, err = s.life.makeWorktree(ctx, st.proj, st.name, branch, base, st.issue == 0, try); err != nil {
			return err
		}
		if work.made {
			// The next thread starts from fresher code; a fetch that fails costs this one nothing.
			go s.life.fetchBase(st.app, st.proj.Dir)
		}
	}
	if st.fork.at != "" {
		restore, err := s.copyChat(st.fork, st.name)
		if err != nil {
			s.life.undoWorktree(context.WithoutCancel(ctx), st.proj, st.name, work)
			return err
		}
		st.restore = restore
	}
	s.progress(st, func() {
		st.work, st.placed = work, true
		if work.note != "" {
			st.note = work.note
		}
	})
	s.setPlace(st, work)
	return nil
}

// setPlace tells the thread's row where it runs, which a listed start knows once its folder is there.
func (s *Threads) setPlace(st *starting, work workPlace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !st.o.Later {
		return
	}
	st.info.Path, st.info.Branch = work.dir, work.branch
	if i := slices.IndexFunc(s.info, func(in term.Info) bool { return in.Name == st.name }); i >= 0 && !st.launched {
		s.info[i] = st.info
	}
}

// unplace takes back what place made, for a start that is not listed and failed before its agent ran.
func (s *Threads) unplace(st *starting) {
	st.restore()
	st.restore = func() {}
	s.life.undoWorktree(context.WithoutCancel(st.app), st.proj, st.name, st.work)
	s.progress(st, func() { st.work, st.placed = workPlace{}, false })
}

// boot starts the agent under its host in tmux and adds the thread to the list as it runs. It needs createMu for a
// start that is not listed; a listed one takes it here, since a refresh must not read tmux between the two.
func (s *Threads) boot(ctx context.Context, st *starting) error {
	if st.o.Later {
		s.createMu.Lock()
		defer s.createMu.Unlock()
	}
	tctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	if st.note != "" && !st.noted {
		if err := s.writeNote(st.name, st.note); err != nil {
			log.Printf("orchestrator: telling the chat of thread %s: %v", st.name, err)
		}
		s.progress(st, func() { st.noted = true })
	}
	title := s.titleOfStart(st)
	if err := s.launch(tctx, st.p, st.name, title, st.proj, st.work.dir, st.work.branch, st.issue, st.model, st.fork.conversation, st.brief, st.handover, st.o.Access, st.action, st.fork, st.o.Parent); err != nil {
		// A stop may have cut tmux off after it made the session.
		if ctx.Err() != nil {
			killCtx, kill := context.WithTimeout(context.WithoutCancel(ctx), tmuxTimeout)
			defer kill()
			_ = s.tmux.Kill(killCtx, st.name)
		}
		return err
	}
	s.register(st.name, title, st.work, st.issue, st.model, st.created, st.o.Access, st.action, st.fork, st.o.Parent)
	s.mu.Lock()
	renamed := st.title
	s.mu.Unlock()
	// A rename that came while the agent started found no session to name. The agent runs, so a failure here stops nothing.
	if renamed != title {
		if err := s.Rename(st.name.String(), renamed); err != nil {
			log.Printf("orchestrator: naming thread %s after its start: %v", st.name, err)
		}
	}
	return nil
}

// writeNote puts a note into the thread's chat log, before its host runs: the chat shows it as a notice.
func (s *Threads) writeNote(name thread.Name, text string) error {
	chatLog, err := chat.OpenLog(s.chatLogPath(name))
	if err != nil {
		return err
	}
	defer chatLog.Close()
	_, err = chatLog.Append(chat.Event{At: time.Now().UnixMilli(), Kind: chat.KindNote, Text: text})
	return err
}

func (s *Threads) titleOfStart(st *starting) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return st.title
}

// renameStarting gives the title to a thread whose agent has not started, which has no tmux session to name yet, and
// says whether it did.
func (s *Threads) renameStarting(name thread.Name, title string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.starting[name.String()]
	if st == nil || st.launched {
		return false
	}
	st.title, st.info.Title = title, title
	if i := slices.IndexFunc(s.info, func(in term.Info) bool { return in.Name == name }); i >= 0 {
		s.info[i].Title = title
	}
	s.sync()
	return true
}

// viewOf is the row of the thread id.
func (s *Threads) viewOf(id string) (Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.list() {
		if v.ID == id {
			return v, nil
		}
	}
	return Thread{}, fmt.Errorf("Thread %s is gone", id)
}

// RetryStart goes on with a start that failed, from the step that failed.
func (s *Threads) RetryStart(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.starting[id]
	if st == nil || st.status != StartFailed && st.status != StartCancelled || st.running || st.aborted {
		return fmt.Errorf("Thread %s has no failed start to retry", id)
	}
	st.status, st.failure, st.running, st.cancelled = Starting, "", true, false
	st.setupTried = false
	st.done = make(chan struct{})
	var run context.Context
	run, st.cancel = context.WithCancel(st.app)
	s.sync()
	go s.runStart(run, st)
	return nil
}

// CancelStart stops the start of the thread id that is under way, at the owner's word. It stops the set-up command and
// the agent's start, and removes the worktree the start made when no thread or terminal is in it and the checks of a
// clean-up pass; one that stays is said in the row. The thread stays listed as a cancelled start, with its first message
// and the held ones kept for a retry, which makes the worktree again if it went. The call returns once that is so.
func (s *Threads) CancelStart(id string) error {
	s.mu.Lock()
	st := s.starting[id]
	if st == nil || st.status != Starting || !st.running || st.aborted {
		s.mu.Unlock()
		return fmt.Errorf("Thread %s has no start to cancel", id)
	}
	st.cancelled = true
	cancel, done := st.cancel, st.done
	s.mu.Unlock()
	cancel()
	<-done
	return nil
}

// abortStart stops a start of the thread id that is listed, for a delete of it, and says whether it took the thread
// away. With worktree, the worktree the start made goes first, by the checks of a clean-up, and one that stays refuses
// the delete; the thread is then listed as a stopped start. Without, the worktree stays on disk and only its record goes.
// A thread whose agent already runs is left to the delete of a running thread, and its start goes on, as a failed one if
// the delete stopped it, until that delete is done.
func (s *Threads) abortStart(id string, worktree bool) (bool, error) {
	s.mu.Lock()
	st := s.starting[id]
	if st == nil {
		s.mu.Unlock()
		return false, nil
	}
	st.aborted = true
	running, cancel, done := st.running, st.cancel, st.done
	s.mu.Unlock()
	if running {
		cancel()
		<-done
	}
	s.createMu.Lock()
	s.mu.Lock()
	if st.launched {
		st.aborted = false
		if s.starting[id] == st && st.status == Starting && !st.running {
			st.status, st.failure = StartFailed, "The start was stopped"
			s.sync()
		}
		s.mu.Unlock()
		s.createMu.Unlock()
		return false, nil
	}
	s.mu.Unlock()
	if worktree {
		s.stopSetup(id)
		if reason := s.dropWorktreeOf(s.context(), st); reason != "" {
			s.mu.Lock()
			st.aborted = false
			if st.status == Starting {
				st.status, st.failure = StartFailed, "The start was stopped"
			}
			s.sync()
			s.mu.Unlock()
			s.createMu.Unlock()
			return false, fmt.Errorf("Thread %d keeps its worktree: %s. Untick the box to keep the worktree", s.Number(id), reason)
		}
	}
	s.mu.Lock()
	s.info = slices.DeleteFunc(s.info, func(in term.Info) bool { return in.Name == st.name })
	if err := bus.RemoveState(s.stateDir, id); err != nil {
		log.Printf("orchestrator: removing the state of thread %s, which did not start: %v", id, err)
	}
	delete(s.seen, id)
	delete(s.records, id)
	s.forgetDeleted(st.name)
	s.sync()
	s.mu.Unlock()
	s.createMu.Unlock()
	s.onDelete(id)
	if err := s.made.Forget(st.name.Project, id); err != nil {
		log.Printf("orchestrator: forgetting the worktree of thread %s, which did not start: %v", id, err)
	}
	for _, path := range []string{s.tmpDir(st.name), s.handoverPath(st.name), s.chatLogPath(st.name), s.chatMediaDir(st.name)} {
		if err := os.RemoveAll(path); err != nil {
			log.Printf("orchestrator: removing %s of thread %s, which did not start: %v", path, id, err)
		}
	}
	return true, nil
}

// startingError is why a thread whose start has not ended cannot be put to sleep or given another access, nil when it
// started. It takes mu.
func (s *Threads) startingError(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.starting[id] != nil {
		return fmt.Errorf("Thread %d is still starting", s.nums[id])
	}
	return nil
}

// listedStarts adds the threads that start but are not in tmux yet to the list tmux gave. It needs mu.
func (s *Threads) listedStarts(infos []term.Info) []term.Info {
	for id, st := range s.starting {
		if !st.launched && !slices.ContainsFunc(infos, func(in term.Info) bool { return in.Name.String() == id }) {
			infos = append(infos, st.info)
		}
	}
	return infos
}

// issueDeleteWait is the longest a start of an issue waits for the delete of the issue's thread.
const issueDeleteWait = 30 * time.Second

// awaitIssueDelete waits until no delete runs for a thread of the issue, so a start of the issue after a delete sees
// the issue free, as the queue shows it, and makes a new thread. When the delete is refused the thread is still there
// and the start gives it back. It waits at most issueDeleteWait, and then the start finds what it finds: the thread being
// deleted, which refuses it. It does not hold issueMu, so the starts of other issues go on.
func (s *Threads) awaitIssueDelete(key string, issue int) {
	deadline := time.After(issueDeleteWait)
	for {
		ended := s.issueDeleting(key, issue)
		if ended == nil {
			return
		}
		select {
		case <-ended:
		case <-deadline:
			return
		}
	}
}

// issueDeleting is the channel that closes when a delete of a thread of the issue ends, nil when none runs. It takes mu.
func (s *Threads) issueDeleting(key string, issue int) chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, ended := range s.deleteWait {
		in, ok := s.infoOf(id)
		if ok && in.Name.Project == key && in.Issue == strconv.Itoa(issue) && in.Fork == "" {
			return ended
		}
	}
	return nil
}
