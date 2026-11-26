package chat

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
)

const (
	maxShown         = 1 << 20 // the largest file the Files view shows
	maxMarkdownFiles = 500
)

var errTooBig = errors.New("This file is too big to show")

// beforeRead, when set, runs between readMarkdown's size check and its read, for a test to grow the file there.
var beforeRead func()

// ChatReadFile is the text of the markdown file at path, relative to the thread's folder or absolute: its worktree,
// else the project folder. Only a file inside that folder, symlinks resolved, of at most 1 MB is read.
func (v *Service) ChatReadFile(id, path string) (string, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return "", err
	}
	return readMarkdown(v.threadDir(ch), path, fmt.Sprintf("thread %d", ch.N))
}

// ProjectReadFile is ChatReadFile for the project's folder.
func (v *Service) ProjectReadFile(project, path string) (string, error) {
	dir := v.projectDir(project)
	if dir == "" {
		return "", fmt.Errorf("No project %s", project)
	}
	return readMarkdown(dir, path, "project "+project)
}

// ChatMarkdownFiles is the markdown files of the thread's folder that git tracks or does not ignore, at most 500.
func (v *Service) ChatMarkdownFiles(id string) ([]string, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return nil, err
	}
	return v.markdownIn(v.threadDir(ch), fmt.Sprintf("thread %d", ch.N))
}

// ProjectMarkdownFiles is ChatMarkdownFiles for the project's folder.
func (v *Service) ProjectMarkdownFiles(project string) ([]string, error) {
	dir := v.projectDir(project)
	if dir == "" {
		return nil, fmt.Errorf("No project %s", project)
	}
	return v.markdownIn(dir, "project "+project)
}

// Plan is the plan the thread's own Claude last asked to have approved, and whether it still waits on the owner.
type Plan struct {
	Text    string `json:"text"`
	Waiting bool   `json:"waiting"`
}

// ChatLatestPlan is the thread's latest plan: of its own Claude, never a subagent's, and not one the owner denied. A
// plan whose input does not parse is left out, and the one before it counts. None is the zero Plan. The log is read
// again only once it has changed.
func (v *Service) ChatLatestPlan(id string) (Plan, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return Plan{}, err
	}
	plan, err := v.plans.of(ch.Log)
	if err != nil {
		log.Printf("agentos: reading the chat log of %s: %v", id, err)
		return Plan{}, fmt.Errorf("Could not read the chat of thread %d", ch.N)
	}
	return plan, nil
}

// planCache keeps each chat log's latest plan with the size and time of change the log had when it was read.
type planCache struct {
	mu    sync.Mutex
	byLog map[string]cachedPlan
}

type cachedPlan struct {
	size int64
	mod  time.Time
	plan Plan
}

func (c *planCache) of(path string) (Plan, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Plan{}, nil
	}
	if err != nil {
		return Plan{}, err
	}
	c.mu.Lock()
	hit, ok := c.byLog[path]
	c.mu.Unlock()
	if ok && hit.size == info.Size() && hit.mod.Equal(info.ModTime()) {
		return hit.plan, nil
	}
	events, err := chatlog.ReadLog(path, 0)
	if err != nil {
		return Plan{}, err
	}
	plan := latestPlan(events)
	c.mu.Lock()
	if c.byLog == nil {
		c.byLog = map[string]cachedPlan{}
	}
	c.byLog[path] = cachedPlan{size: info.Size(), mod: info.ModTime(), plan: plan}
	c.mu.Unlock()
	return plan, nil
}

// latestPlan is the latest plan in the events, as ChatLatestPlan has it. An approval waits as chatlog.Waiting says.
func latestPlan(events []chatlog.Event) Plan {
	var plans []chatlog.Event
	denied := map[int]bool{}
	unanswered := map[string]int{} // the plan each request id last asked for, by its place in plans
	var waiting []chatlog.Event
	var tasks chatlog.Tasks
	for _, e := range events {
		waiting = chatlog.Waiting(waiting, e, &tasks)
		tasks.See(e)
		switch {
		case e.Kind == chatlog.KindApproval && e.Tool == "ExitPlanMode" && e.Agent == "":
			unanswered[e.ID] = len(plans)
			plans = append(plans, e)
		case e.Kind == chatlog.KindAnswer:
			if i, ok := unanswered[e.ID]; ok {
				denied[i] = e.Text != "allow"
				delete(unanswered, e.ID)
			}
		}
	}
	for i, p := range slices.Backward(plans) {
		var in struct {
			Plan string `json:"plan"`
		}
		if denied[i] || json.Unmarshal(p.Input, &in) != nil || in.Plan == "" {
			continue
		}
		return Plan{Text: in.Plan, Waiting: slices.ContainsFunc(waiting, func(a chatlog.Event) bool { return a.Seq == p.Seq && a.ID == p.ID })}
	}
	return Plan{}
}

// threadDir is the folder the thread works in: its worktree, else the project folder.
func (v *Service) threadDir(ch sessions.Chat) string { return cmp.Or(ch.Dir, v.projectDir(ch.Project)) }

// markdownIn is the markdown files of dir, as filesIn lists them; of names whose files they are, for the error.
func (v *Service) markdownIn(dir, of string) ([]string, error) {
	if dir == "" {
		return []string{}, nil
	}
	paths, err := v.files.list(v.ctx(), dir)
	if err != nil {
		log.Printf("agentos: listing the files of %s in %s: %v", of, dir, err)
		return nil, fmt.Errorf("Could not list the files of %s", of)
	}
	out := []string{}
	for _, p := range paths {
		if sessions.IsMarkdown(p) {
			out = append(out, p)
		}
	}
	slices.SortStableFunc(out, func(a, b string) int { return pickerRank(a) - pickerRank(b) })
	return out[:min(len(out), maxMarkdownFiles)], nil
}

// pickerRank is where a file goes in the Files view's picker, so the cap keeps the files it lists first: AGENTS.md,
// README.md, the files under docs/, then the rest.
func pickerRank(path string) int {
	switch {
	case path == "AGENTS.md":
		return 0
	case path == "README.md":
		return 1
	case strings.HasPrefix(path, "docs/"):
		return 2
	}
	return 3
}

// readMarkdown is the text of the markdown file at path in dir, as sessions.OpenMarkdown opens it, when it is at most
// maxShown bytes.
func readMarkdown(dir, path, of string) (string, error) {
	f, _, err := sessions.OpenMarkdown(dir, path, of)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err == nil && info.Size() > maxShown {
		return "", errTooBig
	}
	if beforeRead != nil {
		beforeRead()
	}
	var data []byte
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(f, maxShown+1))
	}
	if err != nil {
		log.Printf("agentos: reading %s in the folder of %s: %v", path, of, err)
		return "", errors.New("Could not read this file")
	}
	if len(data) > maxShown {
		return "", errTooBig
	}
	return string(data), nil
}
