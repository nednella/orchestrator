package threads

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nednella/orchestrator/internal/atomicfile"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/term"
	"github.com/nednella/orchestrator/internal/thread"
)

// quitFailure is why a start that the app's quit cut short is listed as failed when the app starts again.
const quitFailure = "The app quit while the thread started"

// startRecord is a listed start on disk, from the moment the thread is listed until its start ends: the thread has no
// state file before its agent runs, so a quit would lose it, and its first message with it.
type startRecord struct {
	Name     string         `json:"name"`
	Number   int            `json:"number"`
	RawTitle string         `json:"rawTitle"`
	Title    string         `json:"title"`
	Text     string         `json:"text"`
	Send     bool           `json:"send"`
	Issue    int            `json:"issue"`
	Brief    string         `json:"brief"`
	Handover string         `json:"handover"`
	Action   string         `json:"action"`
	Model    project.Model  `json:"model"`
	Access   thread.Access  `json:"access"`
	Place    Place          `json:"place"`
	Base     string         `json:"base"`
	Parent   string         `json:"parent"`
	Created  int64          `json:"created"`
	Branch   string         `json:"branch"`
	WorkDir  string         `json:"workDir"`
	WorkRef  string         `json:"workBranch"`
	Made     bool           `json:"made"`
	Placed   bool           `json:"placed"`
	Launched bool           `json:"launched"`
	Done     bool           `json:"delivered"`
	Sending  string         `json:"sending"`
	Before   int            `json:"sentBefore"`
	Queue    []host.Message `json:"queue"`
	Status   StartState     `json:"status"`
	Failure  string         `json:"failure"`
}

func (s *Threads) startRecordPath(name thread.Name) string {
	return filepath.Join(s.localDir, name.Project, "starts", name.Token+".json")
}

// keepStarts writes the record of each listed start that changed since it was written. It needs mu.
func (s *Threads) keepStarts() {
	for id, st := range s.starting {
		if !st.o.Later {
			continue
		}
		data, err := json.Marshal(startRecord{
			Name: id, Number: s.nums[id], RawTitle: st.rawTitle, Title: st.title, Text: st.text, Send: st.send, Issue: st.issue,
			Brief: st.brief, Handover: st.handover, Action: st.action, Model: st.model, Access: st.o.Access, Place: st.o.Place,
			Base: st.o.Base, Parent: st.o.Parent, Created: st.created.UnixMilli(), Branch: st.branch,
			WorkDir: st.work.dir, WorkRef: st.work.branch, Made: st.work.made, Placed: st.placed, Launched: st.launched,
			Done: st.delivered, Sending: st.sending, Before: st.sentBefore, Queue: st.queue, Status: st.status, Failure: st.failure,
		})
		if err != nil || bytes.Equal(data, st.saved) {
			continue
		}
		path := s.startRecordPath(st.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
			err = atomicfile.Write(path, data, 0o600)
		}
		if err != nil {
			log.Printf("orchestrator: saving the start of thread %s: %v", id, err)
			continue
		}
		st.saved = data
	}
}

// dropStartRecord removes the saved start of the thread, for a start that ended or a thread that went.
func (s *Threads) dropStartRecord(name thread.Name) {
	if err := os.Remove(s.startRecordPath(name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("orchestrator: removing the saved start of thread %s: %v", name, err)
	}
}

// restoreStarts brings back the starts that the app's quit cut short: each is listed as a failed start with its first
// message and the messages it held, for a retry, which goes on from the worktree the start made. A start whose agent
// runs under tmux still is that thread's row, and a retry sends what the agent did not take.
func (s *Threads) restoreStarts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	restored := false
	for _, proj := range s.projects() {
		files, err := os.ReadDir(filepath.Join(s.localDir, proj.Key(), "starts"))
		if err != nil {
			continue
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			token := strings.TrimSuffix(f.Name(), ".json")
			if s.restoreStart(proj, thread.Name{Project: proj.Key(), Token: token}) {
				restored = true
			}
		}
	}
	if restored {
		s.number()
		s.sync()
	}
}

// restoreStart reads the saved start of the thread name and lists it, and says whether it did. It needs mu.
func (s *Threads) restoreStart(proj project.Project, name thread.Name) bool {
	id := name.String()
	data, err := os.ReadFile(s.startRecordPath(name))
	var rec startRecord
	if err != nil || json.Unmarshal(data, &rec) != nil || rec.Name != id {
		log.Printf("orchestrator: the saved start of thread %s cannot be read: %v", id, err)
		return false
	}
	_, ended := s.ended[id]
	live := slices.ContainsFunc(s.info, func(in term.Info) bool { return in.Name == name })
	if s.starting[id] != nil || s.deleted[id] || ended || live && rec.Done && len(rec.Queue) == 0 {
		s.dropStartRecord(name)
		return false
	}
	st := &starting{
		proj: proj, name: name, rawTitle: rec.RawTitle, title: rec.Title, text: rec.Text, send: rec.Send, issue: rec.Issue,
		brief: rec.Brief, handover: rec.Handover, action: rec.Action, o: StartOptions{Model: rec.Model, Access: rec.Access, Place: rec.Place, Base: rec.Base, Parent: rec.Parent, Later: true},
		p: s.Provider(), model: rec.Model, created: time.UnixMilli(rec.Created), app: s.ctx, restore: func() {},
		branch: rec.Branch, work: workPlace{dir: rec.WorkDir, branch: rec.WorkRef, made: rec.Made}, placed: rec.Placed, launched: live,
		delivered: rec.Done, sending: rec.Sending, sentBefore: rec.Before, queue: rec.Queue, status: rec.Status, failure: rec.Failure,
		saved: data,
	}
	st.status, st.failure = StartFailed, quitFailure
	st.info = st.placeholder()
	if st.placed {
		st.info.Path, st.info.Branch = st.work.dir, st.work.branch
	}
	s.starting[id] = st
	if !live {
		s.info = append(s.info, st.info)
	}
	if rec.Number > 0 {
		s.lastNum[proj.Key()] = max(s.lastNum[proj.Key()], rec.Number)
		if s.nums[id] == 0 {
			s.nums[id] = rec.Number
		}
	}
	return true
}
