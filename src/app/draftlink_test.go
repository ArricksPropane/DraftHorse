package main

import (
	"net/url"
	"strings"
	"testing"
)

// ARRICKS-08 tests: draft deep-link construction, the open-in-browser gate,
// and settings persistence/hydration for the toggle.

func TestDraftComposeURL(t *testing.T) {
	cases := []struct {
		name      string
		email     string
		msgID     string
		dedicated bool
		want      string
	}{
		{
			// ARRICKS-18/22: known account routes through AccountChooser;
			// the continue target uses the authuser HINT, never the
			// /u/<email> path form (hard-404s on this tenant even with a
			// live session — proven in validation via the dedicated profile).
			name:  "default browser: authuser hint inside AccountChooser",
			email: "dave@arrickspropane.com",
			msgID: "18f0abc123def456",
			want:  "https://accounts.google.com/AccountChooser?Email=dave%40arrickspropane.com&continue=https%3A%2F%2Fmail.google.com%2Fmail%2F%3Fauthuser%3Ddave%2540arrickspropane.com%23drafts%3Fcompose%3D18f0abc123def456",
		},
		{
			// ARRICKS-22: the dedicated profile holds exactly one session,
			// so /u/0 is the location account by construction.
			name:      "dedicated profile: u/0 inside AccountChooser",
			email:     "dave@arrickspropane.com",
			msgID:     "18f0abc123def456",
			dedicated: true,
			want:      "https://accounts.google.com/AccountChooser?Email=dave%40arrickspropane.com&continue=https%3A%2F%2Fmail.google.com%2Fmail%2Fu%2F0%2F%23drafts%3Fcompose%3D18f0abc123def456",
		},
		{
			name:  "no email falls back to plain u/0",
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
			if got := draftComposeURL(tc.email, tc.msgID, tc.dedicated); got != tc.want {
				t.Errorf("draftComposeURL(%q, %q, %v) = %q, want %q", tc.email, tc.msgID, tc.dedicated, got, tc.want)
			}
		})
	}
}

// A path-hostile account string must not survive into the URL path unescaped.
// Emails can't contain '/', but the input is whatever userinfo returned, and
// the escaping is the invariant worth locking in. ARRICKS-18: the inner
// mail.google.com link now rides encoded inside AccountChooser's `continue`
// param — decode it back out before asserting on its shape.
func TestDraftComposeURLEscapesAccount(t *testing.T) {
	got := draftComposeURL("a/b@example.com", "msg123", false)
	if strings.Contains(got, "u/a/b@") {
		t.Errorf("account not path-escaped: %q", got)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", got, err)
	}
	inner := u.Query().Get("continue")
	if inner == "" {
		t.Fatalf("continue param missing from url: %q", got)
	}
	if strings.Contains(inner, "u/a/b@") {
		t.Errorf("account not path-escaped in continue target: %q", inner)
	}
	if !strings.Contains(inner, "compose=msg123") {
		t.Errorf("message id missing from continue target: %q", inner)
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
