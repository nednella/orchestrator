package main

import (
	"errors"
	"strings"
	"testing"
)

func TestStartFailureTexts(t *testing.T) {
	cause := errors.New("loading config: parsing /x/config.yaml: line 4: bad")
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"config", configFailure(cause, "/x/config.yaml"), []string{"line 4: bad", "Config file: /x/config.yaml"}},
		{"start", startFailure(errors.New("tmux is gone")), []string{"tmux is gone", "no thread is followed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, w := range tt.want {
				if !strings.Contains(tt.text, w) {
					t.Errorf("text %q lacks %q", tt.text, w)
				}
			}
		})
	}
}
