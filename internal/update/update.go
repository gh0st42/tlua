// Package update is tlua update: it asks GitHub for tlua's latest release,
// and when it is newer than the tlua running, offers to put it in this one's
// place. Only a release build does this, one that scripts/build-release.sh
// made and stamped with the archive it went into (version.Build): it knows
// which archive of the new release to fetch. A tlua built from source, or for
// a platform the releases leave out, says so and leaves itself alone.
package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"tlua/internal/version"
)

// Repo is where the releases are.
const Repo = "gh0st42/tlua"

// Platforms are the builds a release has, by the end of their archive's
// name: tlua-VERSION-<platform>.tar.gz, or .zip on Windows.
var Platforms = []string{
	"darwin-arm64",
	"linux-amd64", "linux-amd64-static", "linux-amd64-musl",
	"linux-arm64", "linux-arm64-static", "linux-arm64-musl",
	"windows-amd64",
}

const usage = `usage: tlua update [-check] [-y]

Asks GitHub for tlua's latest release, and when it is newer than this one,
offers to download it and put it in this tlua's place. The download is
checked against the release's SHA256SUMS, and tried, before it replaces
anything.

  -check  only say whether there is a newer release
  -y      update without asking

Only a release build updates itself; one built from source is updated the
way it was built.
`

// Updater is one run of tlua update; the fields are what it would otherwise
// take from the world, so that tests can stand in for it.
type Updater struct {
	API     string // the latest release, as GitHub's API describes it
	Build   string // the platform this tlua was released for; "" from source
	Current string // its version
	Exe     string // the file to replace
	GOOS    string
	In      io.Reader
	Out     io.Writer
	// Asking says whether In is someone who can answer; without them, an
	// update needs -y.
	Asking bool
	Client *http.Client
}

// Command runs tlua update.
func Command(args []string) int {
	fs := flag.NewFlagSet("tlua update", flag.ContinueOnError)
	check := fs.Bool("check", false, "")
	yes := fs.Bool("y", false, "")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Print(usage)
			return 0
		}
		fmt.Fprint(os.Stderr, usage)
		return 1
	}
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "tlua update: cannot tell where this tlua is: %v\n", err)
		return 1
	}
	stat, _ := os.Stdin.Stat()
	u := &Updater{
		API:     "https://api.github.com/repos/" + Repo + "/releases/latest",
		Build:   version.Build,
		Current: version.Number,
		Exe:     exe,
		GOOS:    runtime.GOOS,
		In:      os.Stdin,
		Out:     os.Stdout,
		Asking:  stat != nil && stat.Mode()&os.ModeCharDevice != 0,
		Client:  &http.Client{Timeout: 5 * time.Minute},
	}
	if err := u.Run(*check, *yes); err != nil {
		fmt.Fprintf(os.Stderr, "tlua update: %v\n", err)
		return 1
	}
	return 0
}

// release is what the API says of one.
type release struct {
	Tag    string `json:"tag_name"`
	URL    string `json:"html_url"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (r *release) asset(name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

// Run checks, and unless only checking, asks and updates. A tlua that
// cannot update itself says why and is not an error.
func (u *Updater) Run(checkOnly, yes bool) error {
	if u.Build == "" {
		fmt.Fprintf(u.Out, "This tlua %s was built from source, not by a release, so it does not update itself.\n"+
			"Update it the way it was built, or get a release from https://github.com/%s/releases\n", u.Current, Repo)
		return nil
	}
	if !supported(u.Build) {
		fmt.Fprintf(u.Out, "tlua's releases have no build for %s, so this tlua does not update itself.\n", u.Build)
		return nil
	}
	rel, err := u.latest()
	if err != nil {
		return err
	}
	latest := strings.TrimPrefix(rel.Tag, "v")
	if !newer(latest, u.Current) {
		fmt.Fprintf(u.Out, "tlua %s is the latest release.\n", u.Current)
		return nil
	}
	ext := ".tar.gz"
	if strings.HasPrefix(u.Build, "windows-") {
		ext = ".zip"
	}
	name := "tlua-" + latest + "-" + u.Build
	archiveURL, sumsURL := rel.asset(name+ext), rel.asset("SHA256SUMS")
	if archiveURL == "" || sumsURL == "" {
		fmt.Fprintf(u.Out, "tlua %s is out, but has no %s build yet: %s\n", latest, u.Build, rel.URL)
		return nil
	}
	fmt.Fprintf(u.Out, "tlua %s is out; this is %s. %s\n", latest, u.Current, rel.URL)
	if checkOnly {
		return nil
	}
	if !yes {
		if !u.Asking {
			fmt.Fprintln(u.Out, "Run tlua update -y to update.")
			return nil
		}
		fmt.Fprintf(u.Out, "Update %s to %s? [y/N] ", u.Exe, latest)
		answer, _ := bufio.NewReader(u.In).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
			fmt.Fprintln(u.Out, "Not updated.")
			return nil
		}
	}

	fmt.Fprintf(u.Out, "Downloading %s%s...\n", name, ext)
	archive, err := u.get(archiveURL, 512<<20)
	if err != nil {
		return err
	}
	sums, err := u.get(sumsURL, 1<<20)
	if err != nil {
		return err
	}
	if err := checkSum(archive, name+ext, sums); err != nil {
		return err
	}
	exeName := "tlua"
	if strings.HasPrefix(u.Build, "windows-") {
		exeName = "tlua.exe"
	}
	binary, err := extract(archive, ext, name+"/"+exeName)
	if err != nil {
		return fmt.Errorf("%s%s: %v", name, ext, err)
	}
	if err := u.replace(binary, latest); err != nil {
		return err
	}
	fmt.Fprintf(u.Out, "tlua is %s now.\n", latest)
	return nil
}

func supported(build string) bool {
	for _, p := range Platforms {
		if p == build {
			return true
		}
	}
	return false
}

func (u *Updater) latest() (*release, error) {
	data, err := u.get(u.API, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("asking GitHub for the latest release: %v", err)
	}
	var rel release
	if err := json.Unmarshal(data, &rel); err != nil || rel.Tag == "" {
		return nil, fmt.Errorf("GitHub's answer about the latest release makes no sense")
	}
	return &rel, nil
}

// get fetches a URL, no more than limit bytes of it.
func (u *Updater) get(url string, limit int64) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tlua/"+u.Current)
	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s: bigger than a release of tlua could be", url)
	}
	return data, nil
}

// newer says whether version a comes after b: major, minor and patch, as
// numbers.
func newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parts(v string) [3]int {
	var out [3]int
	for i, s := range strings.SplitN(v, ".", 3) {
		n, _ := strconv.Atoi(strings.TrimFunc(s, func(r rune) bool { return r < '0' || r > '9' }))
		out[i] = n
	}
	return out
}

// checkSum finds name in a SHA256SUMS file and compares.
func checkSum(data []byte, name string, sums []byte) error {
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			got := sha256.Sum256(data)
			if hex.EncodeToString(got[:]) != strings.ToLower(f[0]) {
				return fmt.Errorf("%s is not what SHA256SUMS says it is; nothing was changed", name)
			}
			return nil
		}
	}
	return fmt.Errorf("SHA256SUMS does not list %s; nothing was changed", name)
}

// extract takes one file out of a .tar.gz or a .zip.
func extract(archive []byte, ext, path string) ([]byte, error) {
	if ext == ".zip" {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if f.Name == path {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("no %s in it", path)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("no %s in it", path)
		}
		if err != nil {
			return nil, err
		}
		if strings.TrimPrefix(h.Name, "./") == path {
			return io.ReadAll(tr)
		}
	}
}

// replace puts binary where the running tlua is, once it has shown that it
// runs and is the version it should be. It is written beside the old one
// and renamed over it, so that the old one is there until the new one is.
// Windows does not let a running program be replaced, but does let it be
// renamed: the old one is moved aside as tlua.exe.old, and the next tlua
// removes it (RemoveOld).
func (u *Updater) replace(binary []byte, want string) error {
	dir := filepath.Dir(u.Exe)
	tmp, err := os.CreateTemp(dir, ".tlua-update-*")
	if err != nil {
		return fmt.Errorf("cannot write beside %s (%v); run tlua update as someone who can, or download the release by hand", u.Exe, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	if u.GOOS == "windows" {
		// It has to be called .exe to be run.
		if err := os.Rename(tmpName, tmpName+".exe"); err != nil {
			return err
		}
		tmpName += ".exe"
		defer os.Remove(tmpName)
	}
	out, err := exec.Command(tmpName, "-v").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "tlua "+want+" ") {
		return fmt.Errorf("the downloaded tlua does not run here (%v: %s); nothing was changed", err, strings.TrimSpace(string(out)))
	}
	if u.GOOS == "windows" {
		old := u.Exe + ".old"
		os.Remove(old)
		if err := os.Rename(u.Exe, old); err != nil {
			return fmt.Errorf("cannot move %s aside: %v", u.Exe, err)
		}
		if err := os.Rename(tmpName, u.Exe); err != nil {
			os.Rename(old, u.Exe)
			return fmt.Errorf("cannot put the new tlua in place: %v", err)
		}
		return nil
	}
	if err := os.Rename(tmpName, u.Exe); err != nil {
		return fmt.Errorf("cannot put the new tlua in place: %v", err)
	}
	return nil
}

// RemoveOld removes what an update on Windows left: the tlua it replaced,
// which could not be removed while it ran.
func RemoveOld() {
	if runtime.GOOS != "windows" {
		return
	}
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}
}
