package main

import (
	"strings"
	"testing"
)

// ARRICKS-08 tests: draft deep-link construction, the open-in-browser gate,
// and settings persistence/hydration for the toggle.

func TestDraftComposeURL(t *testing.T) {
	cases := []struct {
		name    string
		email   string
		msgID   string
		want    string
	}{
		{
			name:  "email and message id",
			email: "dave@arrickspropane.com",
			msgID: "18f0abc123def456",
			want:  "https://mail.google.com/mail/u/dave@arrickspropane.com/#drafts?compose=18f0abc123def456",
		},
		{
			name:  "no email falls back to u/0",
			email: "",
			msgID: "18f0abc123def456",
			want:  "https://mail.google.com/mail/u/0/#drafts?compose=18f0abc123def456",
		},
		{
			name:  "empty message id yields empty url",
			email: "dave@arrickspropane.com",
			msgID: "",
			want:  "",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := draftComposeURL(tc.email, tc.msgID); got != tc.want {
				t.Errorf("draftComposeURL(%q, %q) = %q, want %q", tc.email, tc.msgID, got, tc.want)
			}
		})
	}
}

// A path-hostile account string must not survive into the URL path unescaped.
// Emails can't contain '/', but the input is whatever userinfo returned, and
// the escaping is the invariant worth locking in.
func TestDraftComposeURLEscapesAccount(t *testing.T) {
	got := draftComposeURL("a/b@example.com", "msg123")
	if strings.Contains(got, "u/a/b@") {
		t.Errorf("account not path-escaped: %q", got)
	}
	if !strings.Contains(got, "compose=msg123") {
		t.Errorf("message id missing from url: %q", got)
	}
}

func TestOpenDraftInBrowserRespectsToggle(t *testing.T) {
	restore := openDraftURL
	defer func() { openDraftURL = restore }()

	var opened []string
	openDraftURL = func(u string) error {
		opened = append(opened, u)
		return nil
	}

	app := &App{}
	app.settings.OpenDraftInBrowser = false
	app.openDraftInBrowser("msg123")
	if len(opened) != 0 {
		t.Fatalf("toggle off: browser opened %v, want no opens", opened)
	}

	app.settings.OpenDraftInBrowser = true
	app.openDraftInBrowser("msg123")
	if len(opened) != 1 {
		t.Fatalf("toggle on: got %d opens, want 1", len(opened))
	}
	// No auth manager on this App → /u/0 fallback.
	want := "https://mail.google.com/mail/u/0/#drafts?compose=msg123"
	if opened[0] != want {
		t.Errorf("opened %q, want %q", opened[0], want)
	}
}

func TestOpenDraftInBrowserSkipsEmptyMessageID(t *testing.T) {
	restore := openDraftURL
	defer func() { openDraftURL = restore }()

	var opens int
	openDraftURL = func(string) error {
		opens++
		return nil
	}

	app := &App{}
	app.settings.OpenDraftInBrowser = true
	app.openDraftInBrowser("")
	if opens != 0 {
		t.Fatalf("empty message id: got %d opens, want 0", opens)
	}
}

// Settings round-trip: default on, absent field hydrates on (pre-ARRICKS-08
// files), explicit false survives a load.
func TestOpenDraftInBrowserSettingDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOMAPI_APPDATA_DIR", dir)

	if got := loadSettings(); !got.OpenDraftInBrowser {
		t.Error("OpenDraftInBrowser should default to true on first run")
	}

	// Field absent from an older settings file → default on.
	if err := saveSettingsRaw(dir, `{"mode":"manual"}`); err != nil {
		t.Fatalf("saveSettingsRaw: %v", err)
	}
	if got := loadSettings(); !got.OpenDraftInBrowser {
		t.Error("OpenDraftInBrowser should default to true when field is absent")
	}

	// Explicit opt-out persists.
	if err := saveSettingsRaw(dir, `{"mode":"manual","open_draft_in_browser":false}`); err != nil {
		t.Fatalf("saveSettingsRaw: %v", err)
	}
	if got := loadSettings(); got.OpenDraftInBrowser {
		t.Error("OpenDraftInBrowser=false should survive a settings load")
	}
}
