//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ARRICKS-21 tests: argument shape, routing between the dedicated profile
// and the default-browser fallback, and the settings default.

func TestDedicatedBrowserArgs(t *testing.T) {
	args := dedicatedBrowserArgs(`C:\Users\x\AppData\Local\go-mapi\browser-profile`, "https://accounts.google.com/AccountChooser?x=1")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		`--user-data-dir=C:\Users\x\AppData\Local\go-mapi\browser-profile`,
		"--no-first-run",
		"--no-default-browser-check",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %v", want, args)
		}
	}
	// ARRICKS-28: --new-window must STAY out. With it, Chromium's singleton
	// handoff opens a brand-new window per scan instead of reusing the
	// profile's existing one. This assertion is the regression guard.
	if strings.Contains(joined, "--new-window") {
		t.Errorf("--new-window must not be passed (ARRICKS-28 window reuse): %v", args)
	}
	if args[len(args)-1] != "https://accounts.google.com/AccountChooser?x=1" {
		t.Errorf("URL must be the final argument, got %v", args)
	}
}

func TestDedicatedBrowserProfileDirUnderLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\Users\x\AppData\Local`)
	if got, want := dedicatedBrowserProfileDir(), filepath.Join(`C:\Users\x\AppData\Local`, "go-mapi", "browser-profile"); got != want {
		t.Errorf("dedicatedBrowserProfileDir() = %q, want %q", got, want)
	}
}

// Windows runners ship Edge, so App Paths resolution must succeed there.
func TestFindChromiumBrowserResolvesOnWindows(t *testing.T) {
	exe, ok := findChromiumBrowser()
	if !ok {
		t.Skip("no Edge/Chrome App Paths entry on this machine")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Errorf("resolved browser %q does not exist: %v", exe, err)
	}
}

func TestOpenDraftInBrowserPrefersDedicatedProfile(t *testing.T) {
	restoreOpen, restoreLaunch := openDraftURL, launchDraftInDedicatedBrowser
	defer func() { openDraftURL, launchDraftInDedicatedBrowser = restoreOpen, restoreLaunch }()

	// ARRICKS-23: async open — channel capture, App{} zero delay.
	defaultOpens := make(chan string, 2)
	dedicatedOpens := make(chan string, 2)
	openDraftURL = func(u string) error { defaultOpens <- u; return nil }
	launchDraftInDedicatedBrowser = func(u string) error { dedicatedOpens <- u; return nil }

	app := &App{}
	app.settings.OpenDraftInBrowser = true
	app.settings.DraftBrowserDedicated = true
	app.openDraftInBrowser("msg123")
	select {
	case u := <-dedicatedOpens:
		if !strings.Contains(u, "#drafts") {
			t.Errorf("dedicated launch got %q, want the drafts-list link", u)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dedicated launch never happened")
	}
	select {
	case u := <-defaultOpens:
		t.Fatalf("default browser must not open when dedicated succeeds, got %q", u)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestOpenDraftInBrowserFallsBackWhenNoChromium(t *testing.T) {
	restoreOpen, restoreLaunch := openDraftURL, launchDraftInDedicatedBrowser
	defer func() { openDraftURL, launchDraftInDedicatedBrowser = restoreOpen, restoreLaunch }()

	defaultOpens := make(chan string, 2)
	openDraftURL = func(u string) error { defaultOpens <- u; return nil }
	launchDraftInDedicatedBrowser = func(string) error { return errNoChromiumBrowser }

	app := &App{}
	app.settings.OpenDraftInBrowser = true
	app.settings.DraftBrowserDedicated = true
	app.openDraftInBrowser("msg123")
	select {
	case <-defaultOpens:
	case <-time.After(2 * time.Second):
		t.Fatal("expected default-browser fallback, got none")
	}
}

func TestOpenDraftInBrowserDedicatedOffUsesDefault(t *testing.T) {
	restoreOpen, restoreLaunch := openDraftURL, launchDraftInDedicatedBrowser
	defer func() { openDraftURL, launchDraftInDedicatedBrowser = restoreOpen, restoreLaunch }()

	defaultOpens := make(chan string, 2)
	openDraftURL = func(u string) error { defaultOpens <- u; return nil }
	launchDraftInDedicatedBrowser = func(string) error { return errors.New("must not be called") }

	app := &App{}
	app.settings.OpenDraftInBrowser = true
	app.settings.DraftBrowserDedicated = false
	app.openDraftInBrowser("msg123")
	select {
	case <-defaultOpens:
	case <-time.After(2 * time.Second):
		t.Fatal("dedicated off: default browser never opened")
	}
}

func TestDraftOpenDelaySettingDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	// First run → 4000ms.
	if got := loadSettings(); got.DraftOpenDelayMs != 4000 {
		t.Errorf("DraftOpenDelayMs first-run default = %d, want 4000", got.DraftOpenDelayMs)
	}
	write := func(body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, "go-mapi"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go-mapi", "settings.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Absent field (pre-3.8 file) → 4000.
	write(`{"mode":"manual"}`)
	if got := loadSettings(); got.DraftOpenDelayMs != 4000 {
		t.Errorf("absent field = %d, want 4000", got.DraftOpenDelayMs)
	}
	// Explicit 0 disables the delay.
	write(`{"mode":"manual","draft_open_delay_ms":0}`)
	if got := loadSettings(); got.DraftOpenDelayMs != 0 {
		t.Errorf("explicit 0 = %d, want 0", got.DraftOpenDelayMs)
	}
	// Field-typo clamps.
	write(`{"mode":"manual","draft_open_delay_ms":-5}`)
	if got := loadSettings(); got.DraftOpenDelayMs != 0 {
		t.Errorf("negative clamp = %d, want 0", got.DraftOpenDelayMs)
	}
	write(`{"mode":"manual","draft_open_delay_ms":999999}`)
	if got := loadSettings(); got.DraftOpenDelayMs != 60000 {
		t.Errorf("upper clamp = %d, want 60000", got.DraftOpenDelayMs)
	}
}

func TestDraftBrowserDedicatedSettingDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	// First run (no file) → true.
	if got := loadSettings(); !got.DraftBrowserDedicated {
		t.Error("DraftBrowserDedicated should default to true on first run")
	}
	// Pre-3.7 file without the field → true.
	if err := os.MkdirAll(filepath.Join(dir, "go-mapi"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go-mapi", "settings.json"), []byte(`{"mode":"manual","open_draft_in_browser":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(); !got.DraftBrowserDedicated {
		t.Error("DraftBrowserDedicated should default to true when the field is absent")
	}
	// Explicit opt-out is honored.
	if err := os.WriteFile(filepath.Join(dir, "go-mapi", "settings.json"), []byte(`{"mode":"manual","draft_browser_dedicated":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(); got.DraftBrowserDedicated {
		t.Error("explicit draft_browser_dedicated=false should be honored")
	}
}
