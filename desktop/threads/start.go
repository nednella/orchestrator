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
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/term"
	"github.com/nednella/orchestrator/internal/thread"
	"github.com/nednella/orchestrator/internal/util"
)

// StartState says how far the start of a new thread has got, for a thread whose row came before its agent.
type StartState string

const (
	Starting    StartState = "starting" // the worktree, the host or the first message is still on its way
	StartFailed StartState = "failed"   // a step failed; RetryStart goes on from it
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
	work      workPlace
	branch    string // the branch a no-issue start picked
	placed    bool
	restore   func()
	delivered bool

	// shown, under mu
	info     term.Info
	status   StartState
	failure  string
	running  bool
	launched bool
	aborted  bool
	cancel   context.CancelFunc
	done     chan struct{}
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
	err := s.execute(ctx, st)
	s.mu.Lock()
	defer s.mu.Unlock()
	st.running = false
	if err != nil {
		if !st.aborted {
			log.Printf("orchestrator: thread %s did not start: %v", st.name, err)
			st.status, st.failure = StartFailed, startFailure(err)
			s.sync()
		}
		return
	}
	delete(s.starting, st.name.String())
	s.sync()
}

// startFailure is why a start failed, in a sentence for the thread's row: the host's own reason when it refused, else
// the first line of the error.
func startFailure(err error) string {
	if refused := (*host.Refused)(nil); errors.As(err, &refused) {
		return refused.Reason
	}
	return util.FirstLine(err.Error())
}

// errNotDelivered is the error of a first message the agent did not take.
var errNotDelivered = errors.New("the agent did not take the first message in time")

// execute does each step of a start that has not been done yet, so a retry goes on where the start stopped. A step
// that fails leaves nothing behind that a retry would trip on.
func (s *Threads) execute(ctx context.Context, st *starting) error {
	if !st.placed {
		if err := s.place(ctx, st); err != nil {
			return err
		}
	}
	s.mu.Lock()
	launched := st.launched
	s.mu.Unlock()
	if !launched {
		if err := s.boot(ctx, st); err != nil {
			s.unplace(st)
			return err
		}
	}
	if st.work.branch != "" {
		s.life.ensureWatchers(st.app)
	}
	if st.text == "" || st.delivered {
		return nil
	}
	if !st.o.Later {
		go s.giveHost(st.app, st.name, st.text, st.send)
		return nil
	}
	if err := toHost(ctx, s.hostSocket(st.name), st.text, st.send, hostWait); err != nil {
		log.Printf("orchestrator: thread %s did not get its first prompt: %v", st.name, err)
		if refused := (*host.Refused)(nil); errors.As(err, &refused) {
			return err
		}
		return errNotDelivered
	}
	st.delivered = true
	return nil
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
				st.branch = s.life.freeBranch(ctx, st.proj.Dir, branchFromTitle(st.rawTitle))
			}
			branch, base = st.branch, st.o.Base
		}
		var err error
		if work, err = s.life.makeWorktree(ctx, st.proj, st.name, branch, base); err != nil {
			return err
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
	st.work, st.placed = work, true
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

// unplace takes back what place made, for a start that failed before its agent ran.
func (s *Threads) unplace(st *starting) {
	ctx := context.WithoutCancel(st.app)
	st.restore()
	st.restore = func() {}
	s.life.undoWorktree(ctx, st.proj, st.name, st.work)
	st.work, st.placed = workPlace{}, false
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.o.Later {
		st.info = st.placeholder()
		if i := slices.IndexFunc(s.info, func(in term.Info) bool { return in.Name == st.name }); i >= 0 && !st.launched {
			s.info[i] = st.info
		}
	}
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
	title := s.titleOfStart(st)
	if err := s.launch(tctx, st.p, st.name, title, st.proj, st.work.dir, st.work.branch, st.issue, st.model, st.fork.conversation, st.brief, st.handover, st.o.Access, st.action, st.fork, st.o.Parent); err != nil {
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
	if st == nil || st.status != StartFailed || st.running || st.aborted {
		return fmt.Errorf("Thread %s has no failed start to retry", id)
	}
	st.status, st.failure, st.running = Starting, "", true
	st.done = make(chan struct{})
	var run context.Context
	run, st.cancel = context.WithCancel(st.app)
	s.sync()
	go s.runStart(run, st)
	return nil
}

// abortStart stops a start of the thread id that is listed, for a delete of it, and says whether it took the thread
// away. A thread whose agent already runs is left to the delete of a running thread, and its start goes on, as a failed
// one if the delete stopped it, until that delete is done.
func (s *Threads) abortStart(id string) bool {
	s.mu.Lock()
	st := s.starting[id]
	if st == nil {
		s.mu.Unlock()
		return false
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
		return false
	}
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
	return true
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
