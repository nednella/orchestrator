package chat

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/nednella/orchestrator/desktop/internal/media"
	"github.com/nednella/orchestrator/internal/atomicfile"
	"github.com/nednella/orchestrator/internal/host"
)

// maxImages is how many images one message takes.
const maxImages = 10

// Image is an image the user attached to a message: its bytes in base64 and the type it was given, which its bytes
// overrule.
type Image struct {
	Base64 string `json:"base64"`
	Mime   string `json:"mime"`
}

// saveImages checks the images and saves each in dir, named by its content, and returns their paths. When one is
// refused, none is saved.
func saveImages(dir string, images []Image) ([]string, error) {
	if len(images) > maxImages {
		return nil, fmt.Errorf("A message takes at most %d images", maxImages)
	}
	files := make(map[string][]byte, len(images))
	paths := make([]string, 0, len(images))
	for i, image := range images {
		data, ext, err := decodeImage(image)
		if err != nil {
			return nil, fmt.Errorf("Image %d %w", i+1, err)
		}
		sum := sha256.Sum256(data)
		path := filepath.Join(dir, hex.EncodeToString(sum[:8])+ext)
		files[path] = data
		paths = append(paths, path)
	}
	for path, data := range files {
		if err := atomicfile.Write(path, data, 0o600); err != nil {
			log.Printf("orchestrator: saving a chat image: %v", err)
			return nil, errors.New("Could not save the images")
		}
	}
	return paths, nil
}

// decodeImage is the image's bytes and the file extension of the type they are, or what is wrong with it, to follow
// "Image <n>".
func decodeImage(image Image) ([]byte, string, error) {
	tooLarge := fmt.Errorf("is too large (the limit is %s MB)", strconv.FormatFloat(float64(host.MaxImage)/(1<<20), 'f', -1, 64))
	if len(image.Base64) > base64.StdEncoding.EncodedLen(host.MaxImage) {
		return nil, "", tooLarge
	}
	data, err := base64.StdEncoding.DecodeString(image.Base64)
	if err != nil {
		return nil, "", errors.New("could not be read")
	}
	if len(data) > host.MaxImage {
		return nil, "", tooLarge
	}
	ext, ok := media.Ext(http.DetectContentType(data))
	if !ok {
		return nil, "", errors.New("is not png, jpeg, gif or webp")
	}
	return data, ext, nil
}
