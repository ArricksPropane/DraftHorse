//go:build windows

package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/marcfargas/go-mapi/internal/mapi"
)

// ARRICKS-24 cache semantics: fetch once per session, failures retry,
// sign-out resets.

func TestDraftSignatureCachesSuccess(t *testing.T) {
	app := &App{}
	calls := 0
	fetch := func() (string, error) { calls++; return "<b>sig</b>", nil }

	if got := app.draftSignature(fetch); got != "<b>sig</b>" {
		t.Fatalf("first call = %q", got)
	}
	if got := app.draftSignature(fetch); got != "<b>sig</b>" {
		t.Fatalf("second call = %q", got)
	}
	if calls != 1 {
		t.Errorf("fetch called %d times, want 1 (session cache)", calls)
	}
}

func TestDraftSignatureFailureRetriesLater(t *testing.T) {
	app := &App{}
	calls := 0
	fetch := func() (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("403 pre-re-consent")
		}
		return "sig", nil
	}

	if got := app.draftSignature(fetch); got != "" {
		t.Fatalf("failed fetch must yield empty signature, got %q", got)
	}
	if got := app.draftSignature(fetch); got != "sig" {
		t.Fatalf("retry after failure = %q, want sig", got)
	}
	if calls != 2 {
		t.Errorf("fetch called %d times, want 2 (failure must not cache)", calls)
	}
}

func TestResetSignatureCacheForcesRefetch(t *testing.T) {
	app := &App{}
	calls := 0
	fetch := func() (string, error) { calls++; return "sig", nil }

	_ = app.draftSignature(fetch)
	app.resetSignatureCache()
	_ = app.draftSignature(fetch)
	if calls != 2 {
		t.Errorf("fetch called %d times, want 2 after reset", calls)
	}
}

// ARRICKS-29: the 403 is the one signature failure a user can fix, so it —
// and only it — raises a surface.

// withStubbedAnnounce swaps the toast/tray seam for a counter.
func withStubbedAnnounce(t *testing.T) *int {
	t.Helper()
	restore := announceSignatureScope
	t.Cleanup(func() { announceSignatureScope = restore })
	calls := 0
	announceSignatureScope = func(*App) { calls++ }
	return &calls
}

func TestSignatureScopeMissingRaisesOnceAndSticks(t *testing.T) {
	calls := withStubbedAnnounce(t)
	app := &App{}
	// Wrapped, exactly as gmail.go returns it — errors.Is must see through.
	fetch := func() (string, error) {
		return "", fmt.Errorf("signature fetch forbidden: %w", mapi.ErrSignatureScopeMissing)
	}

	for i := 0; i < 3; i++ {
		if got := app.draftSignature(fetch); got != "" {
			t.Fatalf("attempt %d returned %q, want empty", i, got)
		}
	}
	if !app.signatureScopeMissing() {
		t.Error("scope-missing flag should stay set while the 403 persists")
	}
	// A backlog drain must not fire one toast per queued email.
	if *calls != 1 {
		t.Errorf("announce called %d times, want exactly 1 per session", *calls)
	}
}

func TestTransientFailureDoesNotRaiseScopeWarning(t *testing.T) {
	calls := withStubbedAnnounce(t)
	app := &App{}
	fetch := func() (string, error) { return "", errors.New("dial tcp: network is unreachable") }

	_ = app.draftSignature(fetch)
	if app.signatureScopeMissing() {
		t.Error("a network error must not be reported as a missing scope")
	}
	if *calls != 0 {
		t.Errorf("announce called %d times for a transient failure, want 0", *calls)
	}
}

func TestSignatureSuccessClearsScopeWarning(t *testing.T) {
	withStubbedAnnounce(t)
	app := &App{}
	failed := false
	fetch := func() (string, error) {
		if !failed {
			failed = true
			return "", fmt.Errorf("forbidden: %w", mapi.ErrSignatureScopeMissing)
		}
		return "<b>sig</b>", nil
	}

	_ = app.draftSignature(fetch)
	if !app.signatureScopeMissing() {
		t.Fatal("flag should be set after the 403")
	}
	// Re-consent happened; the next fetch succeeds and the tray row clears.
	if got := app.draftSignature(fetch); got != "<b>sig</b>" {
		t.Fatalf("post-re-consent fetch = %q", got)
	}
	if app.signatureScopeMissing() {
		t.Error("a successful fetch must clear the scope-missing flag")
	}
}

func TestResetSignatureCacheClearsScopeWarning(t *testing.T) {
	calls := withStubbedAnnounce(t)
	app := &App{}
	fetch := func() (string, error) {
		return "", fmt.Errorf("forbidden: %w", mapi.ErrSignatureScopeMissing)
	}

	_ = app.draftSignature(fetch)
	app.resetSignatureCache()
	if app.signatureScopeMissing() {
		t.Error("sign-out must clear the scope-missing flag")
	}
	// Sign-out also re-arms the one-shot: a different account signing in on
	// the same session deserves its own warning.
	_ = app.draftSignature(fetch)
	if *calls != 2 {
		t.Errorf("announce called %d times across two sign-ins, want 2", *calls)
	}
}
