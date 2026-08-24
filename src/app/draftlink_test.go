package main

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// ARRICKS-08 tests: draft deep-link construction, the open-in-browser gate,
// and settings persistence/hydration for the toggle.

func TestDraftsListURL(t *testing.T) {
	cases := []struct {
		name      string
		email     string
		dedicated bool
		want      string
	}{
		{
			// ARRICKS-27: the target is the Drafts LIST (fresh server
			// fetch, always accurate), never the ?compose= overlay that
			// hydrated API-created drafts without their attachment chip.
			// ARRICKS-18/22: AccountChooser wrapper + authuser hint, never
			// the /u/<email> path form (hard-404s on this tenant).
			name:  "default browser: authuser hint inside AccountChooser",
			email: "dave@arrickspropane.com",
			want:  "https://accounts.google.com/AccountChooser?Email=dave%40arrickspropane.com&continue=https%3A%2F%2Fmail.google.com%2Fmail%2F%3Fauthuser%3Ddave%2540arrickspropane.com%23drafts",
		},
		{
			// ARRICKS-22: the dedicated profile holds exactly one session,
			// so /u/0 is the location account by construction.
			name:      "dedicated profile: u/0 inside AccountChooser",
			email:     "dave@arrickspropane.com",
			dedicated: true,
			want:      "https://accounts.google.com/AccountChooser?Email=dave%40arrickspropane.com&continue=https%3A%2F%2Fmail.google.com%2Fmail%2Fu%2F0%2F%23drafts",
		},
		{
			name:  "no email falls back to plain u/0",
			email: "",
			want:  "https://mail.google.com/mail/u/0/#drafts",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := draftsListURL(tc.email, tc.dedicated); got != tc.want {
				t.Errorf("draftsListURL(%q, %v) = %q, want %q", tc.email, tc.dedicated, got, tc.want)
			}
		})
	}
}

// A query-hostile account string must be escaped into the authuser param.
// Emails can't contain '&', but the input is whatever userinfo returned,
// and the escaping is the invariant worth locking in.
func TestDraftsListURLEscapesAccount(t *testing.T) {
	got := draftsListURL("a&b@example.com", false)
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", got, err)
	}
	inner := u.Query().Get("continue")
	if inner == "" {
		t.Fatalf("continue param missing from url: %q", got)
	}
	iu, err := url.Parse(inner)
	if err != nil {
		t.Fatalf("url.Parse(inner %q): %v", inner, err)
	}
	if iu.Query().Get("authuser") != "a&b@example.com" {
		t.Errorf("authuser did not round-trip the hostile account: %q", inner)
	}
	if !strings.HasSuffix(inner, "#drafts") {
		t.Errorf("continue target must land on the drafts list: %q", inner)
	}
}

func TestOpenDraftInBrowserRespectsToggle(t *testing.T) {
	restore := openDraftURL
	defer func() { openDraftURL = restore }()

	// ARRICKS-23 made the open async; capture through a channel and assert
	// with timeouts. App{} zero value keeps DraftOpenDelayMs at 0.
	opened := make(chan string, 4)
	openDraftURL = func(u string) error {
		opened <- u
		return nil
	}

	app := &App{}
	app.settings.OpenDraftInBrowser = false
	app.openDraftInBrowser("msg123")
	select {
	case u := <-opened:
		t.Fatalf("toggle off: browser opened %q, want no opens", u)
	case <-time.After(200 * time.Millisecond):
	}

	app.settings.OpenDraftInBrowser = true
	app.openDraftInBrowser("msg123")
	select {
	case u := <-opened:
		// No auth manager on this App → /u/0 fallback (ARRICKS-27: list view).
		want := "https://mail.google.com/mail/u/0/#drafts"
		if u != want {
			t.Errorf("opened %q, want %q", u, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("toggle on: browser never opened")
	}
}

func TestOpenDraftInBrowserSkipsEmptyMessageID(t *testing.T) {
	restore := openDraftURL
	defer func() { openDraftURL = restore }()

	opened := make(chan string, 1)
	openDraftURL = func(u string) error {
		opened <- u
		return nil
	}

	app := &App{}
	app.settings.OpenDraftInBrowser = true
	app.openDraftInBrowser("")
	select {
	case u := <-opened:
		t.Fatalf("empty message id: browser opened %q, want no opens", u)
	case <-time.After(200 * time.Millisecond):
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
