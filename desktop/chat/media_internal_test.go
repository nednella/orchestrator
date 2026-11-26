package chat

import (
	"encoding/base64"
	"testing"

	"github.com/nednella/orchestrator/internal/host"
)

func TestAnImageAtTheLimitIsTaken(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	for _, tt := range []struct {
		size int
		ok   bool
	}{{host.MaxImage, true}, {host.MaxImage - 1, true}, {host.MaxImage + 1, false}} {
		data := append(png, make([]byte, tt.size-len(png))...)
		_, ext, err := decodeImage(Image{Base64: base64.StdEncoding.EncodeToString(data), Mime: "image/png"})
		if (err == nil) != tt.ok || (tt.ok && ext != ".png") {
			t.Errorf("an image of %d bytes: %q, %v; want taken %v", tt.size, ext, err, tt.ok)
		}
	}
}
