//go:build windows

package main

// V4 two-account tests (docs/V4-PLAN.md Phase 2): per-slot credential
// isolation, active-slot switching + persistence, and per-slot signature
// caches. All through the fake keyring — no Credential Manager access.

import (
	"os"
	"path/filepath"
	"testing"
)

// newTwoAccountApp builds an App with both slots on fake keyring stores.
func newTwoAccountApp() (*App, *fakeKeyringStore, *fakeKeyringStore) {
	a := &App{trayRefreshCh: make(chan struct{}, 1)}
	k0, k1 := newFakeKeyringStore(), newFakeKeyringStore()
	a.accounts[0] = NewAuthManagerWithStore(k0)
	a.accounts[1] = NewAuthManagerWithStore(k1)
	a.accounts[1].user = keyringUser2
	a.auth = a.accounts[0]
	return a, k0, k1
}

func TestAccountSlotsPersistToDistinctCredentialEntries(t *testing.T) {
	a, k0, k1 := newTwoAccountApp()
	a.accounts[0].tokens = &OAuthTokens{AccessToken: "tok-0", RefreshToken: "r0"}
	a.accounts[1].tokens = &OAuthTokens{AccessToken: "tok-1", RefreshToken: "r1"}
	if err := a.accounts[0].SaveToKeyring(); err != nil {
		t.Fatal(err)
	}
	if err := a.accounts[1].SaveToKeyring(); err != nil {
		t.Fatal(err)
	}
	if _, err := k0.Get(keyringService, keyringUser); err != nil {
		t.Errorf("slot 0 must persist under %q: %v", keyringUser, err)
	}
	if _, err := k1.Get(keyringService, keyringUser2); err != nil {
		t.Errorf("slot 1 must persist under %q: %v", keyringUser2, err)
	}
	// Cross-check: slot 1 must NOT have written slot 0's entry name.
	if _, err := k1.Get(keyringService, keyringUser); err == nil {
		t.Error("slot 1 wrote the slot-0 credential name — the slots would clobber each other on one shared store")
	}
}

func TestSetActiveAccountSwitchesAndPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	a, _, _ := newTwoAccountApp()

	if err := a.SetActiveAccount(1); err != nil {
		t.Fatal(err)
	}
	if a.auth != a.accounts[1] {
		t.Error("a.auth must repoint at slot 1")
	}
	if a.activeSlot() != 1 {
		t.Errorf("activeSlot() = %d, want 1", a.activeSlot())
	}
	// Persisted: a fresh settings load must come back with slot 1.
	if got := loadSettings(); got.ActiveAccount != 1 {
		t.Errorf("persisted ActiveAccount = %d, want 1", got.ActiveAccount)
	}
	if err := a.SetActiveAccount(2); err == nil {
		t.Error("slot 2 must be rejected — only 0 and 1 exist")
	}
}

func TestActiveAccountSettingClampsToZero(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	if err := os.MkdirAll(filepath.Join(dir, "DraftHorse"), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "DraftHorse", "settings.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Absent (pre-4.0 file) → 0.
	write(`{"mode":"manual"}`)
	if got := loadSettings(); got.ActiveAccount != 0 {
		t.Errorf("absent field = %d, want 0", got.ActiveAccount)
	}
	// Hand-edited nonsense → 0, never an index out of range.
	write(`{"mode":"manual","active_account":7}`)
	if got := loadSettings(); got.ActiveAccount != 0 {
		t.Errorf("out-of-range clamp = %d, want 0", got.ActiveAccount)
	}
	write(`{"mode":"manual","active_account":1}`)
	if got := loadSettings(); got.ActiveAccount != 1 {
		t.Errorf("valid slot 1 = %d, want 1", got.ActiveAccount)
	}
}

func TestSignatureCachesArePerSlot(t *testing.T) {
	withStubbedAnnounce(t)
	a, _, _ := newTwoAccountApp()

	// Slot 0 caches its signature.
	if got := a.draftSignature(func() (string, error) { return "sig-loc-1", nil }); got != "sig-loc-1" {
		t.Fatalf("slot 0 fetch = %q", got)
	}
	// Switch to slot 1 — its cache is cold, so the fetch runs and must not
	// see slot 0's value.
	a.auth = a.accounts[1]
	if got := a.draftSignature(func() (string, error) { return "sig-loc-2", nil }); got != "sig-loc-2" {
		t.Fatalf("slot 1 fetch = %q", got)
	}
	// Back to slot 0: cached value survives the round trip untouched.
	a.auth = a.accounts[0]
	calls := 0
	if got := a.draftSignature(func() (string, error) { calls++; return "MUST-NOT-FETCH", nil }); got != "sig-loc-1" {
		t.Errorf("slot 0 cache = %q, want sig-loc-1", got)
	}
	if calls != 0 {
		t.Error("switching accounts must not evict the other slot's signature cache")
	}
}

func TestSignOutAccountLeavesOtherSlotUntouched(t *testing.T) {
	withStubbedAnnounce(t)
	a, k0, k1 := newTwoAccountApp()
	a.accounts[0].tokens = &OAuthTokens{AccessToken: "tok-0"}
	a.accounts[1].tokens = &OAuthTokens{AccessToken: "tok-1"}
	_ = a.accounts[0].SaveToKeyring()
	_ = a.accounts[1].SaveToKeyring()

	if err := a.SignOutAccount(1); err != nil {
		t.Fatal(err)
	}
	if a.accounts[1].tokens != nil {
		t.Error("slot 1 tokens must clear")
	}
	if _, err := k1.Get(keyringService, keyringUser2); err == nil {
		t.Error("slot 1 credential entry must be deleted")
	}
	if a.accounts[0].tokens == nil {
		t.Error("slot 0 must remain signed in")
	}
	if _, err := k0.Get(keyringService, keyringUser); err != nil {
		t.Errorf("slot 0 credential entry must survive: %v", err)
	}
}
