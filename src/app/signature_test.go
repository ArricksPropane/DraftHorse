//go:build windows

package main

import (
	"errors"
	"testing"
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
