package chat

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/threads"
	chatlog "github.com/nednella/orchestrator/internal/chat"
)

// imageThread is one thread, number 7, with a folder, a temp folder and a folder of chat images.
type imageThread struct {
	folderThread
	tmp, media string
}

func (f imageThread) ChatOf(id string) (threads.Chat, error) {
	return threads.Chat{ID: id, Project: "main", Dir: string(f.folderThread), Tmp: f.tmp, Media: f.media, N: 7}, nil
}

func TestReadImageForAPreview(t *testing.T) {
	t.Parallel()
	dir, tmp, media, elsewhere := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	writeFiles(t, dir, map[string]string{"shots/a.png": "png-a", "notes.txt": "text"})
	writeFiles(t, tmp, map[string]string{"b.png": "png-b", "c.svg": "<svg/>"})
	writeFiles(t, media, map[string]string{"d.webp": "webp-d"})
	writeFiles(t, elsewhere, map[string]string{"secret.png": "png"})
	symlink(t, filepath.Join(elsewhere, "secret.png"), filepath.Join(tmp, "link.png"))
	svc := NewService(New(imageThread{folderThread(dir), tmp, media}, t.TempDir(), func(string, any) {}), run.Exec, func(string) string { return "" }, context.Background)

	for _, path := range []string{"shots/a.png", filepath.Join(dir, "shots/a.png"), filepath.Join(tmp, "b.png"), filepath.Join(media, "d.webp")} {
		if got, err := svc.ChatReadImage("main/abcdefgh", path); err != nil || got.Kind != "image" || got.Data == "" {
			t.Errorf("ChatReadImage(%q) = %+v, %v; want the image", path, got, err)
		}
	}
	for _, path := range []string{
		filepath.Join(elsewhere, "secret.png"),
		"../secret.png",
		"b.png", // relative paths start from the folder, not the temp folder
		filepath.Join(tmp, "link.png"),
		filepath.Join(tmp, "c.svg"),
		filepath.Join(dir, "notes.txt"),
	} {
		if got, err := svc.ChatReadImage("main/abcdefgh", path); err == nil {
			t.Errorf("ChatReadImage(%q) = %+v; want it refused", path, got)
		}
	}
	if _, err := svc.ChatReadImage("main/abcdefgh", filepath.Join(elsewhere, "secret.png")); err == nil || err.Error() != "This file is outside thread 7's folder" {
		t.Errorf("a file outside every folder gave %v, want the outside-folder sentence", err)
	}
}

func TestPendingQuestionCarriesThePreview(t *testing.T) {
	t.Parallel()
	input := `{"questions":[{"question":"Which layout?","header":"Layout","multiSelect":false,"options":[{"label":"Stepper","description":"d","preview":"![shot](/tmp/a.png)"}]}]}`
	p := pendingOf("main/abcdefgh", chatlog.Event{Kind: chatlog.KindApproval, ID: "r1", Tool: "AskUserQuestion", Input: json.RawMessage(input)})
	sent, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Kind  string `json:"kind"`
		Input struct {
			Questions []struct {
				Options []struct {
					Preview string `json:"preview"`
				} `json:"options"`
			} `json:"questions"`
		} `json:"input"`
	}
	if err := json.Unmarshal(sent, &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != "question" || p.InputTooLarge || got.Input.Questions[0].Options[0].Preview != "![shot](/tmp/a.png)" {
		t.Errorf("the pending question sent is %s; want its option's preview in the input", sent)
	}
}
