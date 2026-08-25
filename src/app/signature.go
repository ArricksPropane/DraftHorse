package main

// ARRICKS-24 — Gmail signature on scan drafts.
//
// Gmail never inserts the account signature into API-created drafts, so
// scan drafts shipped unsigned (Affixa had the same limitation and grew
// its own signature feature). The app now reads the account's default
// sendAs signature (gmail.settings.basic — see the house-rule-6 amendment
// in auth.go) and stamps it onto the message just before draft creation;
// internal/mapi renders it into the MIME body.
//
// ARRICKS-29 — make a missing signature scope VISIBLE.
//
// Refreshing an OAuth token returns the scopes the grant was originally
// issued with, so upgrading the binary never widens an existing grant. A
// machine that signed in before 3.8.0 keeps producing unsigned drafts
// forever, and the only evidence was one ERROR line in app.log that nobody
// reads. That is exactly how a scanner PC shipped unsigned drafts for a day
// (found 2026-08-25 only because every test draft was inspected by hand).
//
// The 403 is a perfectly good signal — the app just threw it away. Now it
// raises a tray row and one toast telling the user the single thing that
// fixes it: sign out, sign in. Fresh installs on 3.8.0+ consent to the
// scope at first sign-in and never see any of this; the guard exists for
// pre-3.8 machines and for the NEXT time a scope is added.

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/marcfargas/go-mapi/internal/mapi"
)

type signatureCache struct {
	mu      sync.Mutex
	fetched bool
	value   string

	// scopeMissing is true once a fetch has failed with
	// mapi.ErrSignatureScopeMissing — the user-fixable failure. Drives the
	// tray row; cleared by a later success or by sign-out.
	scopeMissing bool

	// warned guards the toast so a backlog drain cannot fire one per email.
	// One toast per session; the tray row is the persistent surface.
	warned bool
}

// draftSignature returns the session-cached signature, fetching once via
// fetch on first use. Failures are logged and yield "" WITHOUT caching, so
// a later draft retries — that covers both transient network errors and
// the one-time 403 window before a machine re-consents to the new scope.
// The signature CONTENT is never logged (it can carry names/numbers).
//
// The fetch stays under the mutex so concurrent drafts cannot stampede the
// API. Notification happens after the unlock — emitting a toast while
// holding the cache lock would let a COM call block every other draft.
// See fetchSignatureLocked for the split.
func (a *App) draftSignature(fetch func() (string, error)) string {
	sig, notify := a.fetchSignatureLocked(fetch)
	if notify {
		announceSignatureScope(a)
	}
	return sig
}

// fetchSignatureLocked owns the cache mutex for the whole read-or-fetch and
// reports whether the caller should raise the ARRICKS-29 warning. Split from
// draftSignature purely so the notification lands outside the lock while the
// unlock stays deferred (panic-safe, as it was before ARRICKS-29).
func (a *App) fetchSignatureLocked(fetch func() (string, error)) (string, bool) {
	a.sigCache.mu.Lock()
	defer a.sigCache.mu.Unlock()
	if a.sigCache.fetched {
		return a.sigCache.value, false
	}
	sig, err := fetch()
	if err != nil {
		// ARRICKS-29: separate the failure the user can fix from the ones
		// they cannot. Only the scope 403 gets a surface; a flaky network
		// stays an anonymous log line that retries on the next draft.
		notify := false
		if errors.Is(err, mapi.ErrSignatureScopeMissing) {
			a.sigCache.scopeMissing = true
			if !a.sigCache.warned {
				a.sigCache.warned = true
				notify = true
			}
		}
		logError("signature: fetch failed, draft goes unsigned this attempt: %v", err)
		return "", notify
	}
	a.sigCache.fetched = true
	a.sigCache.value = sig
	a.sigCache.scopeMissing = false
	logInfo("signature: loaded (%d bytes)", len(sig))
	return sig, false
}

// announceSignatureScope raises the two user-facing surfaces: one toast
// (this session) and the tray row (until it is fixed). Never called with the
// cache lock held.
//
// A package var rather than a method so tests can swap it, mirroring the
// openDraftURL / launchDraftInDedicatedBrowser seams in draftlink.go — a
// unit test must not push a real toast through COM.
var announceSignatureScope = func(a *App) {
	emitSignatureScopeToast(a)
	a.signalTrayRefresh()
}

// signatureScopeMissing reports whether the stored grant is known to lack
// gmail.settings.basic. Read by the tray refresh loop.
func (a *App) signatureScopeMissing() bool {
	a.sigCache.mu.Lock()
	defer a.sigCache.mu.Unlock()
	return a.sigCache.scopeMissing
}

// primeSignatureCache does the ARRICKS-24 fetch once at startup instead of
// waiting for the first scan. Two reasons: a pre-3.8 grant is reported the
// moment the app starts rather than after the user has already sent an
// unsigned draft, and the first scan of the day stops paying the fetch
// latency inside the draft path.
//
// Runs on its own goroutine — startup must never block on a network call.
// Every failure is swallowed: this is a diagnostic, and D-04 says nothing
// here may ever break drafting.
func (a *App) primeSignatureCache(ctx context.Context) {
	if a.auth == nil || !a.auth.Status().Authenticated {
		return
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var fetchErr error
	err := a.MakeAuthenticatedGmailCall(callCtx, func(token string) (int, error) {
		gc := mapi.NewGmailClientWithBase(token, gmailBaseURLOverride)
		fetchErr = nil
		_ = a.draftSignature(func() (string, error) {
			sig, e := gc.GetPrimarySignature()
			fetchErr = e
			return sig, e
		})
		// Same convention as automode/CreateDraftForID: "token expired" is
		// the 401 signal that drives refresh-and-retry-once. Anything else
		// is already handled inside draftSignature.
		if fetchErr != nil && fetchErr.Error() == "token expired" {
			return 401, fetchErr
		}
		return 200, nil
	})
	if err != nil {
		logError("signature: startup prime skipped: %v", err)
	}
}

// resetSignatureCache drops the cached signature. Called on sign-out so a
// later sign-in (possibly a different account) refetches — and so the
// ARRICKS-29 tray row clears, since signing out and back in is precisely
// the fix it asks for.
func (a *App) resetSignatureCache() {
	a.sigCache.mu.Lock()
	defer a.sigCache.mu.Unlock()
	a.sigCache.fetched = false
	a.sigCache.value = ""
	a.sigCache.scopeMissing = false
	a.sigCache.warned = false
}
