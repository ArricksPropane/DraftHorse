package main

// ARRICKS-24 — Gmail signature on scan drafts.
//
// Gmail never inserts the account signature into API-created drafts, so
// scan drafts shipped unsigned (Affixa had the same limitation and grew
// its own signature feature). The app now reads the account's default
// sendAs signature (gmail.settings.basic — see the house-rule-6 amendment
// in auth.go) and stamps it onto the message just before draft creation;
// internal/mapi renders it into the MIME body.

import "sync"

type signatureCache struct {
	mu      sync.Mutex
	fetched bool
	value   string
}

// draftSignature returns the session-cached signature, fetching once via
// fetch on first use. Failures are logged and yield "" WITHOUT caching, so
// a later draft retries — that covers both transient network errors and
// the one-time 403 window before a machine re-consents to the new scope.
// The signature CONTENT is never logged (it can carry names/numbers).
func (a *App) draftSignature(fetch func() (string, error)) string {
	a.sigCache.mu.Lock()
	defer a.sigCache.mu.Unlock()
	if a.sigCache.fetched {
		return a.sigCache.value
	}
	sig, err := fetch()
	if err != nil {
		logError("signature: fetch failed, draft goes unsigned this attempt: %v", err)
		return ""
	}
	a.sigCache.fetched = true
	a.sigCache.value = sig
	logInfo("signature: loaded (%d bytes)", len(sig))
	return sig
}

// resetSignatureCache drops the cached signature. Called on sign-out so a
// later sign-in (possibly a different account) refetches.
func (a *App) resetSignatureCache() {
	a.sigCache.mu.Lock()
	defer a.sigCache.mu.Unlock()
	a.sigCache.fetched = false
	a.sigCache.value = ""
}
