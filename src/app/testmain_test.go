//go:build windows

package main

// TestMain pins the package-global log file OUTSIDE any test's t.TempDir.
//
// The trap (caught by CI on the V4 accounts branch): writeLog's sync.Once
// opens %APPDATA%\DraftHorse\app.log on the FIRST log call of the process
// and holds the handle until exit. Whichever test logs first while holding
// a t.Setenv("APPDATA", t.TempDir()) therefore pins app.log inside its own
// temp dir, and on Windows the framework's TempDir cleanup then fails with
// "file in use" — failing that test even though its assertions all passed.
// Which test logs first depends on alphabetical test order, so the suite
// was one rename away from this before V4 added accounts_test.go at the
// top of the alphabet.
//
// Fix: initialize logging once, up front, into a process-lifetime temp dir
// that nothing ever tries to remove while the process lives.

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if dir, err := os.MkdirTemp("", "drafthorse-test-appdata-"); err == nil {
		orig := os.Getenv("APPDATA")
		_ = os.Setenv("APPDATA", dir)
		logInfo("test suite: log pinned to %s", dir)
		_ = os.Setenv("APPDATA", orig)
		// No cleanup: the log handle stays open for the process lifetime
		// (that is the point), and os.Exit skips defers anyway. One tiny
		// dir per test process lives in %TEMP%, which the OS owns.
	}
	os.Exit(m.Run())
}
