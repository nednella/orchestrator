// Package update finds, downloads and installs a newer release of orchestrator.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// Repo is where the releases are.
	Repo = "nednella/orchestrator"
	// Asset is the zip of the app bundle attached to each release; its sha256 sits beside it as Asset + ".sha256".
	Asset = "orchestrator-darwin-arm64.zip"

	// latestURL redirects to the newest release's page. It is github.com, not the API, which allows
	// only 60 requests an hour from an IP without a token.
	latestURL   = "https://github.com/" + Repo + "/releases/latest"
	downloadURL = "https://github.com/" + Repo + "/releases/download/"
	// tagURL is the API's answer for one release; one request when the notes of the running version are read.
	tagURL = "https://api.github.com/repos/" + Repo + "/releases/tags/"
	bundle = "orchestrator.app"
)

// sizeEvery is how often a download's partial file is measured; a var so tests can shorten it.
var sizeEvery = 250 * time.Millisecond

// Runner runs a command and returns its stdout: curl and ditto. Tests inject one that never
// reaches the network.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Release is a published version and where its bundle is.
type Release struct {
	Version string `json:"version"` // "1.2.3", without the v of the tag
	Zip     string `json:"zip"`     // where to download the bundle
	Sum     string `json:"sum"`     // where its sha256 is
}

// Latest asks GitHub for the newest release.
func Latest(ctx context.Context, run Runner) (Release, error) {
	out, err := run(ctx, "curl", "-fsS", "-o", "/dev/null", "--max-time", "10", "-w", "%{redirect_url}", latestURL)
	if err != nil {
		return Release{}, fmt.Errorf("asking GitHub for the latest release: %w", err)
	}
	_, tag, ok := strings.Cut(string(out), "/releases/tag/")
	if !ok || tag == "" {
		return Release{}, fmt.Errorf("GitHub sent no latest release, only %q", out)
	}
	rel := Release{Version: strings.TrimPrefix(tag, "v"), Zip: downloadURL + tag + "/" + Asset, Sum: downloadURL + tag + "/" + Asset + ".sha256"}
	// A release is published before its assets are uploaded.
	if _, err := run(ctx, "curl", "-fsSI", "-o", "/dev/null", "--max-time", "10", rel.Sum); err != nil {
		return Release{}, fmt.Errorf("release %s has no %s with its sha256 yet: %w", tag, Asset, err)
	}
	return rel, nil
}

// ErrNoNotes says GitHub has no release for the version asked, or one with no notes.
var ErrNoNotes = errors.New("there is no release with notes for this version")

// Notes reads the notes of the release of version, as markdown.
func Notes(ctx context.Context, run Runner, version string) (string, error) {
	out, err := run(ctx, "curl", "-sS", "--max-time", "10", "-H", "Accept: application/vnd.github+json", "-w", "\n%{http_code}", tagURL+"v"+version)
	if err != nil {
		return "", fmt.Errorf("asking GitHub for the notes of %s: %w", version, err)
	}
	// -w appends the status after the body, on a line of its own.
	i := strings.LastIndex(string(out), "\n")
	if i < 0 {
		return "", fmt.Errorf("GitHub answered nothing for the notes of %s", version)
	}
	body, code := out[:i], strings.TrimSpace(string(out[i+1:]))
	switch code {
	case "200":
	case "404":
		return "", ErrNoNotes
	default:
		return "", fmt.Errorf("GitHub answered %s for the notes of %s", code, version)
	}
	var rel struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", fmt.Errorf("reading the notes of %s: %w", version, err)
	}
	if strings.TrimSpace(rel.Body) == "" {
		return "", ErrNoNotes
	}
	return rel.Body, nil
}

// Newer says whether latest is a higher version than current. A dev build is never behind.
func Newer(current, latest string) bool {
	a, okA := parse(current)
	b, okB := parse(latest)
	if !okA || !okB {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return b[i] > a[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var n [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return n, false
	}
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return n, false
		}
		n[i] = x
	}
	return n, true
}

// Download fetches rel's bundle into dir and says how far it got: at each 10% step and at 100%, or -1 once when
// GitHub sends no size. The download goes through curl, which sets no quarantine flag on what it fetches.
func Download(ctx context.Context, run Runner, rel Release, dir string, progress func(percent int)) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("making room for the download: %w", err)
	}
	zip := filepath.Join(dir, Asset)
	size := downloadSize(ctx, run, rel.Zip)
	if size <= 0 {
		progress(-1)
	}
	done := make(chan struct{})
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		if size > 0 {
			watchSize(zip, size, done, progress)
		}
	}()
	_, err := run(ctx, "curl", "-fsSL", "--max-time", "600", "-o", zip, rel.Zip)
	close(done)
	<-watched
	if err != nil {
		_ = os.Remove(zip)
		return "", fmt.Errorf("downloading %s: %w", rel.Zip, err)
	}
	progress(100)
	return zip, nil
}

// downloadSize is the content-length of the last response on the way to url, or 0 when there is none.
func downloadSize(ctx context.Context, run Runner, url string) int64 {
	out, err := run(ctx, "curl", "-fsSIL", "--max-time", "10", url)
	if err != nil {
		return 0
	}
	var size int64
	for _, line := range strings.Split(string(out), "\n") {
		name, value, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "content-length") {
			continue
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
			size = n
		}
	}
	return size
}

// watchSize reads how big the partial file at path is until done, and reports each new 10% step below 100: only a
// finished curl makes it 100.
func watchSize(path string, size int64, done <-chan struct{}, progress func(int)) {
	tick := time.NewTicker(sizeEvery)
	defer tick.Stop()
	last := 0
	for {
		select {
		case <-done:
			return
		case <-tick.C:
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		step := min(int(info.Size()*10/size)*10, 90)
		if step > last {
			last = step
			progress(step)
		}
	}
}

// Verify checks the downloaded zip against rel's sha256.
func Verify(ctx context.Context, run Runner, rel Release, zip string) error {
	sum, err := run(ctx, "curl", "-fsSL", "--max-time", "60", rel.Sum)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", rel.Sum, err)
	}
	return verify(zip, string(sum))
}

// InstallZip checks the downloaded zip against rel's sha256 and puts its bundle in place of the one at path.
func InstallZip(ctx context.Context, run Runner, rel Release, zip, path string) error {
	if err := Verify(ctx, run, rel, zip); err != nil {
		return err
	}
	work, err := os.MkdirTemp(filepath.Dir(path), ".orchestrator-update-") // beside the bundle: the swap is then a rename
	if err != nil {
		return fmt.Errorf("making room for the new app: %w", err)
	}
	defer os.RemoveAll(work)
	if _, err := run(ctx, "ditto", "-x", "-k", zip, work); err != nil {
		return fmt.Errorf("unpacking %s: %w", Asset, err)
	}
	fresh := filepath.Join(work, bundle)
	if _, err := os.Stat(filepath.Join(fresh, "Contents", "MacOS", "orchestrator")); err != nil {
		return fmt.Errorf("%s holds no %s", Asset, bundle)
	}
	return swap(fresh, path)
}

// verify checks the file against a sha256 line of the form "<hex>  <name>".
func verify(path, sum string) error {
	want := strings.Fields(sum)
	if len(want) == 0 {
		return errors.New("the release's sha256 file is empty")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want[0] {
		return fmt.Errorf("the download's sha256 is %s, the release says %s", got, want[0])
	}
	return nil
}

// swap puts fresh at path and removes what was there. The old bundle is moved aside first, so a
// running app keeps its files until the move succeeds.
func swap(fresh, path string) error {
	old := path + ".old"
	_ = os.RemoveAll(old)
	if err := os.Rename(path, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("moving the old app aside: %w", err)
	}
	if err := os.Rename(fresh, path); err != nil {
		_ = os.Rename(old, path)
		return fmt.Errorf("putting the new app in place: %w", err)
	}
	return os.RemoveAll(old)
}

// Relaunch opens the bundle at path once this process has ended; the caller quits right after.
func Relaunch(path string) error {
	script := "while kill -0 $1 2>/dev/null; do sleep 0.2; done; open \"$2\""
	cmd := exec.Command("sh", "-c", script, "sh", strconv.Itoa(os.Getpid()), path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("arranging the relaunch: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
