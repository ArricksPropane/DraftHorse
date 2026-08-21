//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"--new-window",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q: %v", want, args)
		}
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

	var defaultOpens, dedicatedOpens []string
	openDraftURL = func(u string) error { defaultOpens = append(defaultOpens, u); return nil }
	launchDraftInDedicatedBrowser = func(u string) error { dedicatedOpens = append(dedicatedOpens, u); return nil }

	app := &App{}
	app.settings.OpenDraftInBrowser = true
	app.settings.DraftBrowserDedicated = true
	app.openDraftInBrowser("msg123")
	if len(dedicatedOpens) != 1 || len(defaultOpens) != 0 {
		t.Fatalf("dedicated on: dedicated=%v default=%v, want exactly one dedicated open", dedicatedOpens, defaultOpens)
	}
	if !strings.Contains(dedicatedOpens[0], "compose=msg123") {
		t.Errorf("dedicated launch got %q, want the draft link", dedicatedOpens[0])
	}
}

func TestOpenDraftInBrowserFallsBackWhenNoChromium(t *testing.T) {
	restoreOpen, restoreLaunch := openDraftURL, launchDraftInDedicatedBrowser
	defer func() { openDraftURL, launchDraftInDedicatedBrowser = restoreOpen, restoreLaunch }()

	var defaultOpens []string
	openDraftURL = func(u string) error { defaultOpens = append(defaultOpens, u); return nil }
	launchDraftInDedicatedBrowser = func(string) error { return errNoChromiumBrowser }

	app := &App{}
	app.settings.OpenDraftInBrowser = true
	app.settings.DraftBrowserDedicated = true
	app.openDraftInBrowser("msg123")
	if len(defaultOpens) != 1 {
		t.Fatalf("expected default-browser fallback exactly once, got %v", defaultOpens)
	}
}

func TestOpenDraftInBrowserDedicatedOffUsesDefault(t *testing.T) {
	restoreOpen, restoreLaunch := openDraftURL, launchDraftInDedicatedBrowser
	defer func() { openDraftURL, launchDraftInDedicatedBrowser = restoreOpen, restoreLaunch }()

	var defaultOpens int
	openDraftURL = func(string) error { defaultOpens++; return nil }
	launchDraftInDedicatedBrowser = func(string) error { return errors.New("must not be called") }

	app := &App{}
	app.settings.OpenDraftInBrowser = true
	app.settings.DraftBrowserDedicated = false
	app.openDraftInBrowser("msg123")
	if defaultOpens != 1 {
		t.Fatalf("dedicated off: default opens = %d, want 1", defaultOpens)
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
