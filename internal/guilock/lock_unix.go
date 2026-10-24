//go:build unix

// Package guilock lets only one test run at a time open real windows and
// send input to them. Two runs at once (a suite started while another is
// still going) take each other's focus and keys, and fail in ways that look
// like bugs: a stray character, a hover that never rests, a crash in the
// toolkit. The lock is a file in the temporary folder, held until the
// process ends.
package guilock

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

var held *os.File

// Acquire waits until no other process holds the lock, and holds it until
// this one exits.
func Acquire() {
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "tlua-gui-tests.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return // no lock is better than no tests
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		fmt.Fprintln(os.Stderr, "waiting for another run of the GUI tests to finish...")
		start := time.Now()
		if syscall.Flock(int(f.Fd()), syscall.LOCK_EX) != nil {
			f.Close()
			return
		}
		fmt.Fprintf(os.Stderr, "went on after %s\n", time.Since(start).Round(time.Second))
	}
	held = f
}
