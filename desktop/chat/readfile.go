package chat

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/nednella/orchestrator/desktop/sessions"
	chatlog "github.com/nednella/orchestrator/internal/chat"
)

const (
	maxShown = 1 << 20 // the most of a text file the Files view reads
	maxImage = 8 << 20 // the largest image the Files view shows
)

var errTooBig = errors.New("This file is too big to show")

// FileContent is a file as the Files view shows it. Kind is "text", "image" or "binary". Text is the file's text, at
// most its first maxShown bytes, and Truncated says it was cut; Size is the file's own size in bytes. An image has its
// Mime type and its bytes as base64 in Data. A binary file has neither.
type FileContent struct {
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
	Mime      string `json:"mime"`
	Data      string `json:"data"`
}

// imageMimes are the image types the Files view shows, by file extension.
var imageMimes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".bmp": "image/bmp", ".ico": "image/x-icon", ".avif": "image/avif", ".svg": "image/svg+xml",
}

// ChatReadFile is the file at path, relative to the thread's folder or absolute: its worktree, else the project
// folder. Only a regular file inside that folder, symlinks resolved, is read.
func (v *Service) ChatReadFile(id, path string) (FileContent, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return FileContent{}, err
	}
	return readFile(v.threadDir(ch), path, fmt.Sprintf("thread %d", ch.N))
}

// ProjectReadFile is ChatReadFile for the project's folder.
func (v *Service) ProjectReadFile(project, path string) (FileContent, error) {
	dir := v.projectDir(project)
	if dir == "" {
		return FileContent{}, fmt.Errorf("No project %s", project)
	}
	return readFile(dir, path, "project "+project)
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
		log.Printf("orchestrator: reading the chat log of %s: %v", id, err)
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

// readFile is the file at path in dir, as sessions.OpenFile opens it; of names whose folder dir is, for the error.
func readFile(dir, path, of string) (FileContent, error) {
	f, real, err := sessions.OpenFile(dir, path, of)
	if err != nil {
		return FileContent{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		log.Printf("orchestrator: reading %s in the folder of %s: %v", path, of, err)
		return FileContent{}, errors.New("Could not read this file")
	}
	if mime, ok := imageMimes[strings.ToLower(filepath.Ext(real))]; ok {
		data, err := readAtMost(f, maxImage)
		if err != nil {
			log.Printf("orchestrator: reading %s in the folder of %s: %v", path, of, err)
			return FileContent{}, errors.New("Could not read this file")
		}
		if len(data) > maxImage {
			return FileContent{}, errTooBig
		}
		return FileContent{Kind: "image", Size: int64(len(data)), Mime: mime, Data: base64.StdEncoding.EncodeToString(data)}, nil
	}
	data, err := readAtMost(f, maxShown)
	if err != nil {
		log.Printf("orchestrator: reading %s in the folder of %s: %v", path, of, err)
		return FileContent{}, errors.New("Could not read this file")
	}
	content := FileContent{Kind: "text", Size: max(info.Size(), int64(len(data)))}
	if len(data) > maxShown {
		data, content.Truncated = data[:maxShown], true
		// The cut may fall inside a character.
		for cut := 0; cut < utf8.UTFMax && !utf8.Valid(data) && len(data) > 0; cut++ {
			data = data[:len(data)-1]
		}
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return FileContent{Kind: "binary", Size: content.Size}, nil
	}
	content.Text = string(data)
	return content, nil
}

// readAtMost is the first limit+1 bytes of f at most, so that a longer file shows as longer than limit.
func readAtMost(f io.Reader, limit int) ([]byte, error) {
	return io.ReadAll(io.LimitReader(f, int64(limit)+1))
}
