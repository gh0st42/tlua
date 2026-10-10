package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeRelease serves a release the way GitHub does: the API's answer, an
// archive whose tlua is a shell script that says it is that version, and
// SHA256SUMS. sum, when given, is put in SHA256SUMS instead of the right one.
func fakeRelease(t *testing.T, tag, script, sum string) *httptest.Server {
	t.Helper()
	ver := strings.TrimPrefix(tag, "v")
	name := "tlua-" + ver + "-linux-amd64"
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: name + "/README.md", Mode: 0o644, Size: 2})
	tw.Write([]byte("hi"))
	tw.WriteHeader(&tar.Header{Name: name + "/tlua", Mode: 0o755, Size: int64(len(script))})
	tw.Write([]byte(script))
	tw.Close()
	gz.Close()
	archive := buf.Bytes()
	if sum == "" {
		h := sha256.Sum256(archive)
		sum = hex.EncodeToString(h[:])
	}
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"html_url": "https://example.com/" + tag,
			"assets": []map[string]string{
				{"name": name + ".tar.gz", "browser_download_url": srv.URL + "/archive"},
				{"name": "SHA256SUMS", "browser_download_url": srv.URL + "/sums"},
			},
		})
	})
	mux.HandleFunc("/archive", func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/sums", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  other.zip\n%s  %s.tar.gz\n", strings.Repeat("0", 64), sum, name)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// updater is one for a tlua 0.5.3 released for linux-amd64, at a file of
// its own, answering what answer says when asked.
func updater(t *testing.T, srv *httptest.Server, answer string) (*Updater, *bytes.Buffer) {
	exe := filepath.Join(t.TempDir(), "tlua")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	return &Updater{
		API: srv.URL + "/latest", Build: "linux-amd64", Current: "0.5.3", Exe: exe,
		GOOS: runtime.GOOS, In: strings.NewReader(answer), Out: &out, Asking: true,
		Client: srv.Client(),
	}, &out
}

func contents(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const newTlua = "#!/bin/sh\necho 'tlua 0.6.0 (Lua 5.1 via gopher-lua, pure Go)'\n"

func TestUpdateReplacesTheBinaryWhenAskedTo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in tlua is a shell script")
	}
	srv := fakeRelease(t, "v0.6.0", newTlua, "")
	u, out := updater(t, srv, "y\n")
	if err := u.Run(false, false); err != nil {
		t.Fatal(err)
	}
	if got := contents(t, u.Exe); got != newTlua {
		t.Errorf("the binary is %q", got)
	}
	if !strings.Contains(out.String(), "tlua 0.6.0 is out; this is 0.5.3") || !strings.Contains(out.String(), "tlua is 0.6.0 now") {
		t.Errorf("it said %q", out.String())
	}
	// Nothing is left beside it.
	entries, _ := os.ReadDir(filepath.Dir(u.Exe))
	if len(entries) != 1 {
		t.Errorf("left behind: %v", entries)
	}
}

func TestUpdateLeavesTheBinaryAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in tlua is a shell script")
	}
	cases := []struct {
		name, tag, script, sum, answer string
		check, yes, notAsking          bool
		err, says                      string
	}{
		{name: "no", tag: "v0.6.0", script: newTlua, answer: "n\n", says: "Not updated"},
		{name: "nothing typed", tag: "v0.6.0", script: newTlua, answer: "", says: "Not updated"},
		{name: "only checking", tag: "v0.6.0", script: newTlua, check: true, says: "tlua 0.6.0 is out"},
		{name: "nobody to ask", tag: "v0.6.0", script: newTlua, notAsking: true, says: "tlua update -y"},
		{name: "the same release", tag: "v0.5.3", script: newTlua, yes: true, says: "0.5.3 is the latest"},
		{name: "an older release", tag: "v0.4.9", script: newTlua, yes: true, says: "0.5.3 is the latest"},
		{name: "a wrong checksum", tag: "v0.6.0", script: newTlua, sum: strings.Repeat("ab", 32), yes: true, err: "not what SHA256SUMS says"},
		{name: "a binary that fails", tag: "v0.6.0", script: "#!/bin/sh\nexit 3\n", yes: true, err: "does not run here"},
		{name: "a binary of another version", tag: "v0.6.0", script: "#!/bin/sh\necho 'tlua 0.5.9 (x)'\n", yes: true, err: "does not run here"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := fakeRelease(t, c.tag, c.script, c.sum)
			u, out := updater(t, srv, c.answer)
			u.Asking = !c.notAsking
			err := u.Run(c.check, c.yes)
			switch {
			case c.err == "" && err != nil:
				t.Errorf("error: %v", err)
			case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
				t.Errorf("error %v, want one saying %q", err, c.err)
			}
			if c.says != "" && !strings.Contains(out.String(), c.says) {
				t.Errorf("it said %q, not %q", out.String(), c.says)
			}
			if got := contents(t, u.Exe); got != "old" {
				t.Errorf("the binary was changed to %q", got)
			}
			entries, _ := os.ReadDir(filepath.Dir(u.Exe))
			if len(entries) != 1 {
				t.Errorf("left behind: %v", entries)
			}
		})
	}
}

func TestUpdateSkipsWhatItCannotUpdate(t *testing.T) {
	srv := fakeRelease(t, "v0.6.0", newTlua, "")
	for build, says := range map[string]string{
		"":              "built from source",
		"freebsd-amd64": "no build for freebsd-amd64",
		"darwin-arm64":  "has no darwin-arm64 build yet", // the release only has linux-amd64
	} {
		u, out := updater(t, srv, "y\n")
		u.Build = build
		if err := u.Run(false, true); err != nil {
			t.Errorf("%q: %v", build, err)
		}
		if !strings.Contains(out.String(), says) {
			t.Errorf("%q: it said %q, not %q", build, out.String(), says)
		}
		if contents(t, u.Exe) != "old" {
			t.Errorf("%q: the binary was changed", build)
		}
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.5.4", "0.5.3", true}, {"0.6.0", "0.5.9", true}, {"1.0.0", "0.99.99", true},
		{"0.5.10", "0.5.9", true}, {"0.5.3", "0.5.3", false}, {"0.5.2", "0.5.3", false},
		{"0.5", "0.5.0", false},
	} {
		if got := newer(c.a, c.b); got != c.want {
			t.Errorf("newer(%s, %s) = %v", c.a, c.b, got)
		}
	}
}

// Every platform the release workflow builds is one an update can fetch.
func TestPlatformsAreTheReleases(t *testing.T) {
	data, err := os.ReadFile("../../scripts/build-release.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`build "tlua-$version-$os-$arch-musl"`, `build "tlua-$version-$os-$arch-static"`, `version.Build=${name#tlua-$version-}`} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("build-release.sh no longer has %s", want)
		}
	}
}
