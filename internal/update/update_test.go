package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const latestTag = "https://github.com/nednella/orchestrator/releases/tag/v1.2.3"

// fakeGitHub answers the latest release's redirect with tag, and has only the assets in sums.
func fakeGitHub(tag string, sums ...string) Runner {
	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		url := args[len(args)-1]
		switch {
		case name != "curl":
		case url == latestURL:
			return []byte(tag), nil
		case slices.Contains(sums, url):
			return nil, nil
		}
		return nil, fmt.Errorf("curl: (56) The requested URL returned error: 404 for %s %v", name, args)
	}
}

func TestLatestReadsTheReleaseFromTheRedirect(t *testing.T) {
	sum := "https://github.com/nednella/orchestrator/releases/download/v1.2.3/orchestrator-darwin-arm64.zip.sha256"
	rel, err := Latest(context.Background(), fakeGitHub(latestTag, sum))
	if err != nil {
		t.Fatal(err)
	}
	want := Release{Version: "1.2.3", Zip: "https://github.com/nednella/orchestrator/releases/download/v1.2.3/orchestrator-darwin-arm64.zip", Sum: sum}
	if rel != want {
		t.Errorf("got %+v, want %+v", rel, want)
	}

	if _, err := Latest(context.Background(), fakeGitHub(latestTag)); err == nil || !strings.Contains(err.Error(), "no orchestrator-darwin-arm64.zip") {
		t.Errorf("a release without the bundle: %v", err)
	}
	if _, err := Latest(context.Background(), fakeGitHub("", sum)); err == nil {
		t.Error("no redirect gave a release")
	}
}

func TestNewer(t *testing.T) {
	for _, tt := range []struct {
		current, latest string
		want            bool
	}{
		{"1.2.3", "1.2.4", true},
		{"1.2.3", "1.10.0", true},
		{"1.2.3", "2.0.0", true},
		{"1.2.3", "1.2.3", false},
		{"1.2.3", "1.2.2", false},
		{"v1.2.3", "v1.3.0", true},
		{"dev", "1.2.3", false},
		{"1.2.3", "", false},
	} {
		if got := Newer(tt.current, tt.latest); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v", tt.current, tt.latest, got)
		}
	}
}

// fakeDownloads answers curl from files on disk and runs ditto for real.
func fakeDownloads(t *testing.T, files map[string]string) Runner {
	t.Helper()
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "ditto" {
			return exec.CommandContext(ctx, name, args...).Output()
		}
		if name != "curl" {
			return nil, fmt.Errorf("unexpected %s", name)
		}
		url := args[len(args)-1]
		src, ok := files[url]
		if !ok {
			return nil, fmt.Errorf("curl: (22) 404 for %s", url)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		for i, a := range args {
			if a == "-o" {
				return nil, os.WriteFile(args[i+1], data, 0o600)
			}
		}
		return data, nil
	}
}

// releaseZip makes a zip of a bundle whose binary prints version, and its sha256 file.
func releaseZip(t *testing.T, dir, version string) (zip, sum string) {
	t.Helper()
	app := filepath.Join(dir, bundle)
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "MacOS", "orchestrator"), []byte("#!/bin/sh\necho "+version+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	zip = filepath.Join(dir, Asset)
	if out, err := exec.Command("ditto", "-c", "-k", "--keepParent", app, zip).CombinedOutput(); err != nil {
		t.Fatalf("ditto: %v: %s", err, out)
	}
	data, err := os.ReadFile(zip)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	sum = zip + ".sha256"
	if err := os.WriteFile(sum, []byte(hex.EncodeToString(h[:])+"  "+Asset+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return zip, sum
}

// fakeRelease answers curl for one release whose zip holds body, and unpacks the zip as ditto would into a bundle
// whose binary is the zip's bytes; ditto itself never runs.
type fakeRelease struct {
	rel     Release
	body    []byte
	sum     string
	size    string                           // the content-length GitHub sends; "" sends none
	written func(n int)                      // called after each tenth of the body is written, when set
	cut     bool                             // the download breaks off halfway
	calls   func(name string, args []string) // sees every command, when set
}

func (f *fakeRelease) run(_ context.Context, name string, args ...string) ([]byte, error) {
	if f.calls != nil {
		f.calls(name, args)
	}
	switch name {
	case "ditto":
		data, err := os.ReadFile(args[2])
		if err != nil {
			return nil, err
		}
		bin := filepath.Join(args[3], bundle, "Contents", "MacOS")
		if err := os.MkdirAll(bin, 0o700); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(filepath.Join(bin, "orchestrator"), data, 0o700)
	case "curl":
	default:
		return nil, fmt.Errorf("unexpected %s", name)
	}
	url := args[len(args)-1]
	switch {
	case url == f.rel.Zip && slices.Contains(args, "-fsSIL"):
		if f.size == "" {
			return nil, errors.New("curl: (22) 403")
		}
		return []byte("HTTP/2 302\r\ncontent-length: 0\r\nlocation: https://objects/x\r\n\r\nHTTP/2 200\r\nContent-Length: " + f.size + "\r\n\r\n"), nil
	case url == f.rel.Zip:
		out := args[slices.Index(args, "-o")+1]
		file, err := os.Create(out)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		for i := 1; i <= 10; i++ {
			if _, err := file.Write(f.body[(i-1)*len(f.body)/10 : i*len(f.body)/10]); err != nil {
				return nil, err
			}
			if f.written != nil {
				f.written(i)
			}
			if f.cut && i == 5 {
				return nil, errors.New("curl: (18) transfer closed with bytes remaining")
			}
		}
		return nil, nil
	case url == f.rel.Sum:
		return []byte(f.sum), nil
	}
	return nil, fmt.Errorf("curl: (22) 404 for %s", url)
}

func newFakeRelease(version string) *fakeRelease {
	body := []byte(strings.Repeat("orchestrator "+version+"\n", 100))
	h := sha256.Sum256(body)
	return &fakeRelease{
		rel:  Release{Version: version, Zip: "https://x/" + version + "/zip", Sum: "https://x/" + version + "/sum"},
		body: body,
		sum:  hex.EncodeToString(h[:]) + "  " + Asset + "\n",
		size: fmt.Sprint(len(body)),
	}
}

func TestDownloadSaysHowFarItGotInTenPercentSteps(t *testing.T) {
	every := sizeEvery
	sizeEvery = 10 * time.Millisecond
	t.Cleanup(func() { sizeEvery = every })
	f := newFakeRelease("2.0.0")
	steps := make(chan int, 32)
	// Each tenth waits until its step is reported, so every step shows however the reads fall.
	f.written = func(i int) {
		if i == 10 {
			return
		}
		deadline := time.After(5 * time.Second)
		for {
			select {
			case p := <-steps:
				steps <- p
				if p >= i*10 {
					return
				}
				time.Sleep(10 * time.Millisecond)
			case <-deadline:
				return
			}
		}
	}
	var got []int
	progress := func(p int) {
		got = append(got, p)
		select {
		case <-steps:
		default:
		}
		steps <- p
	}
	dir := filepath.Join(t.TempDir(), "update")
	zip, err := Download(context.Background(), f.run, f.rel, dir, progress)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}; !slices.Equal(got, want) {
		t.Errorf("progress %v, want %v", got, want)
	}
	if data, err := os.ReadFile(zip); err != nil || string(data) != string(f.body) || zip != filepath.Join(dir, Asset) {
		t.Errorf("the zip at %s holds %d bytes, %v", zip, len(data), err)
	}
}

func TestDownloadWithNoSizeSaysSoOnce(t *testing.T) {
	f := newFakeRelease("2.0.0")
	f.size = ""
	var got []int
	if _, err := Download(context.Background(), f.run, f.rel, t.TempDir(), func(p int) { got = append(got, p) }); err != nil {
		t.Fatal(err)
	}
	if want := []int{-1, 100}; !slices.Equal(got, want) {
		t.Errorf("progress %v, want %v", got, want)
	}
}

func TestAFailedDownloadLeavesNoZip(t *testing.T) {
	f := newFakeRelease("2.0.0")
	f.cut = true
	dir := t.TempDir()
	if _, err := Download(context.Background(), f.run, f.rel, dir, func(int) {}); err == nil || !strings.Contains(err.Error(), "downloading https://x/2.0.0/zip") {
		t.Errorf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, Asset)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a zip stayed: %v", err)
	}
}

// installed makes an app bundle at <temp>/Applications/orchestrator.app whose binary holds "old".
func installed(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Applications", bundle)
	if err := os.MkdirAll(filepath.Join(path, "Contents", "MacOS"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "Contents", "MacOS", "orchestrator"), []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func binary(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(path, "Contents", "MacOS", "orchestrator"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallZipSwapsInTheVerifiedBundle(t *testing.T) {
	f := newFakeRelease("2.0.0")
	path := installed(t)
	zip, err := Download(context.Background(), f.run, f.rel, t.TempDir(), func(int) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := InstallZip(context.Background(), f.run, f.rel, zip, path); err != nil {
		t.Fatal(err)
	}
	if got := binary(t, path); got != string(f.body) {
		t.Errorf("the installed binary is %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("leftovers beside the bundle: %v", entries)
	}
}

func TestInstallZipRefusesABadSum(t *testing.T) {
	f := newFakeRelease("2.0.0")
	path := installed(t)
	zip, err := Download(context.Background(), f.run, f.rel, t.TempDir(), func(int) {})
	if err != nil {
		t.Fatal(err)
	}
	f.sum = strings.Repeat("0", 64) + "  " + Asset + "\n"
	unpacked := false
	f.calls = func(name string, _ []string) { unpacked = unpacked || name == "ditto" }
	if err := InstallZip(context.Background(), f.run, f.rel, zip, path); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Errorf("a wrong sum installed anyway: %v", err)
	}
	if unpacked || binary(t, path) != "old" {
		t.Errorf("unpacked %v, the bundle holds %q", unpacked, binary(t, path))
	}
}

func notesRunner(out string, err error) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) { return []byte(out), err }
}

func TestNotesReadsTheBodyOfTheRelease(t *testing.T) {
	got, err := Notes(context.Background(), notesRunner("{\"body\":\"## 1.2.3\\n\\n* a fix\"}\n200", nil), "1.2.3")
	if err != nil || got != "## 1.2.3\n\n* a fix" {
		t.Errorf("Notes = %q, %v", got, err)
	}
}

func TestNotesTellsAMissingReleaseFromAFailure(t *testing.T) {
	if _, err := Notes(context.Background(), notesRunner("{\"message\":\"Not Found\"}\n404", nil), "1.2.3"); !errors.Is(err, ErrNoNotes) {
		t.Errorf("404 = %v, want ErrNoNotes", err)
	}
	if _, err := Notes(context.Background(), notesRunner("{\"body\":\"\"}\n200", nil), "1.2.3"); !errors.Is(err, ErrNoNotes) {
		t.Errorf("empty body = %v, want ErrNoNotes", err)
	}
	for name, run := range map[string]Runner{
		"curl failed": notesRunner("", errors.New("exit 6")),
		"rate limit":  notesRunner("{}\n403", nil),
		"no status":   notesRunner("garbage", nil),
		"not json":    notesRunner("<html>\n200", nil),
	} {
		if _, err := Notes(context.Background(), run, "1.2.3"); err == nil || errors.Is(err, ErrNoNotes) {
			t.Errorf("%s = %v, want a failure", name, err)
		}
	}
}
