package update

import "testing"

func TestReleaseNamesAreOrchestrators(t *testing.T) {
	tests := []struct{ name, got, want string }{
		{"repo", Repo, "nednella/orchestrator"},
		{"asset", Asset, "orchestrator-darwin-arm64.zip"},
		{"bundle", bundle, "orchestrator.app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
