package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/nednella/orchestrator/internal/atomicfile"
	chatlog "github.com/nednella/orchestrator/internal/chat"
)

// Command is one slash command, as Claude lists them when it starts.
type Command struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	ArgumentHint string   `json:"argumentHint"`
	Builtin      bool     `json:"builtin,omitempty"`
	Aliases      []string `json:"aliases,omitempty"`
}

// commandList is the list a thread of the project reported last, and when.
type commandList struct {
	At       int64     `json:"at"`
	Commands []Command `json:"commands"`
}

// commandLists keeps the latest command list a thread of each project reported, in <dir>/<project>/commands.json, so
// that a project's new thread can offer them before its Claude starts.
type commandLists struct {
	dir string

	mu     sync.Mutex
	latest map[string]commandList // each project's latest list, once read; its time can be later than the saved one's
}

func newCommandLists(dir string) *commandLists {
	return &commandLists{dir: dir, latest: map[string]commandList{}}
}

func (c *commandLists) path(project string) string {
	return filepath.Join(c.dir, project, "commands.json")
}

// keep saves the list a commands event of the project holds, unless the latest one was reported later or is the
// same.
func (c *commandLists) keep(project string, e chatlog.Event) {
	var commands []Command
	if err := json.Unmarshal(e.Input, &commands); err != nil {
		log.Printf("orchestrator: reading the commands of a thread of %s: %v", project, err)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	latest, ok := c.latest[project]
	if !ok {
		saved, err := c.readLocked(project)
		if err != nil {
			log.Printf("orchestrator: %v", err)
		}
		latest = saved
	}
	if e.At <= latest.At {
		c.latest[project] = latest
		return
	}
	next := commandList{At: e.At, Commands: commands}
	if !reflect.DeepEqual(next.Commands, latest.Commands) {
		data, err := json.Marshal(next)
		if err == nil {
			err = atomicfile.Write(c.path(project), data, 0o600)
		}
		if err != nil {
			log.Printf("orchestrator: saving the commands of %s: %v", project, err)
			return
		}
	}
	c.latest[project] = next
}

// list is the project's saved commands, none when no thread of it has reported any.
func (c *commandLists) list(project string) ([]Command, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	saved, err := c.readLocked(project)
	if err != nil {
		return nil, err
	}
	return saved.Commands, nil
}

// readLocked reads the project's saved list. It needs mu.
func (c *commandLists) readLocked(project string) (commandList, error) {
	data, err := os.ReadFile(c.path(project))
	if errors.Is(err, fs.ErrNotExist) {
		return commandList{Commands: []Command{}}, nil
	}
	saved := commandList{Commands: []Command{}}
	if err == nil {
		err = json.Unmarshal(data, &saved)
	}
	if err != nil {
		return commandList{Commands: []Command{}}, fmt.Errorf("reading the commands of %s: %w", project, err)
	}
	if saved.Commands == nil {
		saved.Commands = []Command{}
	}
	return saved, nil
}
