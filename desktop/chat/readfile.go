package chat

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
)

const (
	maxShown         = 1 << 20 // the largest file the Files view shows
	maxMarkdownFiles = 500
)

var errTooBig = errors.New("This file is too big to show")

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
	return v.markdownIn(v.threadDir(ch), "thread "+id)
}

// ProjectMarkdownFiles is ChatMarkdownFiles for the project's folder.
func (v *Service) ProjectMarkdownFiles(project string) ([]string, error) {
	dir := v.projectDir(project)
	if dir == "" {
		return nil, fmt.Errorf("No project %s", project)
	}
	return v.markdownIn(dir, "project "+project)
}

// ChatLatestPlan is the plan of the latest ExitPlanMode approval in the thread's chat, "" when it has none.
func (v *Service) ChatLatestPlan(id string) (string, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return "", err
	}
	events, err := chatlog.ReadLog(ch.Log, 0)
	if err != nil {
		log.Printf("agentos: reading the chat log of %s: %v", id, err)
		return "", fmt.Errorf("Could not read the chat of thread %s", id)
	}
	for i := len(events) - 1; i >= 0; i-- {
		if e := events[i]; e.Kind == chatlog.KindApproval && e.Tool == "ExitPlanMode" {
			var in struct {
				Plan string `json:"plan"`
			}
			_ = json.Unmarshal(e.Input, &in)
			return in.Plan, nil
		}
	}
	return "", nil
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
		if len(out) == maxMarkdownFiles {
			break
		}
	}
	return out, nil
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
