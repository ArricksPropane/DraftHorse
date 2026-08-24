package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/browser"
)

// ARRICKS-08: open a just-created draft in the user's default browser.
//
// The scan-to-email flow ends with the user finishing the draft in Gmail —
// adding the recipient, checking the scan, pressing Send. Without this, the
// draft lands silently in mail.google.com's Drafts folder and the user has
// to go find it. With it, the compose window is on screen the moment the
// draft exists.
//
// Gmail's web UI accepts a message id in the `compose` fragment parameter;
// the account is selected by putting the signed-in address in the /u/ slot
// (mail.google.com redirects it to the right authuser index), so a machine
// with several Google sessions in the browser still lands on the account
// the app created the draft under.
//
// D-04 invariant (same as the update-download path): a failed browser open
// is logged and swallowed — the draft already exists, so failing to surface
// it must never fail the queue flow.

// openDraftURL is the browser-launch seam; tests swap it to capture the URL
// instead of opening a real browser. Production points at browser.OpenURL,
// which resolves the OS default browser.
var openDraftURL = browser.OpenURL

// draftComposeURL builds the deep link for a just-created draft.
// messageID is DraftResponse.Message.ID (NOT the draft id). Returns "" when
// messageID is empty — without it there is nothing to deep-link to.
//
// ARRICKS-18: when the account email is known, the mail.google.com link is
// wrapped in accounts.google.com/AccountChooser — an existing session
// passes straight through; a missing one gets Google's sign-in prefilled
// with the right address, and `continue` then lands on the draft.
//
// ARRICKS-22: the continue target NEVER uses Gmail's /mail/u/<email>/
// email-in-path form. Validation proved it serves the dead "Temporary
// Error (404) / 6446" page on this tenant even WITH a live session for
// that exact account (the dedicated ARRICKS-21 profile, signed in by IT).
// Instead:
//   - dedicated profile: /mail/u/0/ — the profile holds exactly one
//     session, so index 0 is the location account by construction.
//   - default browser: ?authuser=<email> — Google's account-selection
//     hint; on a mismatch it falls back to the default session instead of
//     hard-404ing, and the AccountChooser hop upstream has already steered
//     the right account where possible.
func draftComposeURL(accountEmail, messageID string, dedicated bool) string {
	if messageID == "" {
		return ""
	}
	frag := "#drafts?compose=" + url.QueryEscape(messageID)
	var target string
	if dedicated || accountEmail == "" {
		target = "https://mail.google.com/mail/u/0/" + frag
	} else {
		target = "https://mail.google.com/mail/?authuser=" + url.QueryEscape(accountEmail) + frag
	}
	if accountEmail == "" {
		return target
	}
	v := url.Values{
		"Email":    {accountEmail},
		"continue": {target},
	}
	return "https://accounts.google.com/AccountChooser?" + v.Encode()
}

// isOpenDraftInBrowserEnabled reads the ARRICKS-08 toggle under the
// settings RLock (mirrors isUpdateChecksEnabled).
func (a *App) isOpenDraftInBrowserEnabled() bool {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	return a.settings.OpenDraftInBrowser
}

// openDraftInBrowser opens the compose deep link for messageID if the
// toggle is on. Safe from any goroutine: settings are read under RLock,
// AuthManager.Status locks internally, and the browser launch is a
// fire-and-forget exec. Called from automode.draftOne and CreateDraftForID
// after MarkProcessed succeeds.
func (a *App) openDraftInBrowser(messageID string) {
	if !a.isOpenDraftInBrowserEnabled() {
		return
	}
	if messageID == "" {
		// Draft was created but the response carried no message id —
		// nothing to link to. Log id-free per the privacy contract.
		logError("draftlink: draft response missing message id; skipping browser open")
		return
	}
	email := ""
	if a.auth != nil {
		email = a.auth.Status().Email
	}
	// Everything the launch needs is captured HERE, before the goroutine —
	// the async hop must not read App state later (races with settings
	// writes and shutdown).
	dedicated := a.isDraftBrowserDedicated()
	delay := a.draftOpenDelay()
	ctx := a.shutdownCtx
	if ctx == nil {
		ctx = context.Background()
	}

	// ARRICKS-23: wait before opening. Gmail's web client syncs API-created
	// drafts on its own schedule; opening the compose view immediately
	// renders it without the attachment chip until reopened (validation
	// finding — Affixa exhibited the same). The delay runs async so a
	// backlog drain is never slowed by it.
	//
	// ARRICKS-21/22 inside the goroutine: prefer the dedicated isolated
	// profile; fall through to the default browser if no Edge/Chrome is
	// installed or the launch fails, so the draft is never left
	// unreachable. The URL is rebuilt per mode: /u/0 inside the
	// single-account dedicated profile, authuser hint for the default
	// browser.
	go func() {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}
		if dedicated {
			err := launchDraftInDedicatedBrowser(draftComposeURL(email, messageID, true))
			if err == nil {
				return
			}
			logError("draftlink: dedicated browser launch failed (%v); falling back to default browser", err)
		}
		if err := openDraftURL(draftComposeURL(email, messageID, false)); err != nil {
			// D-04: log + swallow. The draft exists; the user can still
			// reach it via mail.google.com.
			logError("draftlink: browser open failed: %v", err)
		}
	}()
}

// draftOpenDelay reads the ARRICKS-23 delay under the settings RLock.
func (a *App) draftOpenDelay() time.Duration {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	return time.Duration(a.settings.DraftOpenDelayMs) * time.Millisecond
}

// isDraftBrowserDedicated reads the ARRICKS-21 toggle under the settings RLock.
func (a *App) isDraftBrowserDedicated() bool {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	return a.settings.DraftBrowserDedicated
}

// setDraftBrowserDedicated flips the ARRICKS-21 toggle and persists through
// the single-writer settings path (mirrors setOpenDraftInBrowser).
func (a *App) setDraftBrowserDedicated(enabled bool) error {
	a.settingsMu.Lock()
	a.settings.DraftBrowserDedicated = enabled
	s := a.settings
	a.settingsMu.Unlock()
	if err := saveSettings(s); err != nil {
		return fmt.Errorf("settings: persist draft-browser-dedicated: %w", err)
	}
	a.signalTrayRefresh()
	return nil
}

// SetDraftBrowserDedicated is the Wails binding form of setDraftBrowserDedicated.
func (a *App) SetDraftBrowserDedicated(enabled bool) error {
	return a.setDraftBrowserDedicated(enabled)
}

// setOpenDraftInBrowser flips the toggle and persists through the
// single-writer settings path (mirrors setUpdateChecksEnabled, minus the
// update-state sync it doesn't need). Signals tray refresh so the checkbox
// re-renders.
func (a *App) setOpenDraftInBrowser(enabled bool) error {
	a.settingsMu.Lock()
	a.settings.OpenDraftInBrowser = enabled
	s := a.settings
	a.settingsMu.Unlock()
	if err := saveSettings(s); err != nil {
		return fmt.Errorf("settings: persist open-draft-in-browser: %w", err)
	}
	a.signalTrayRefresh()
	return nil
}

// SetOpenDraftInBrowser is the Wails binding form of setOpenDraftInBrowser.
// GetSettings already surfaces the field, so a future settings panel needs
// no extra read binding.
func (a *App) SetOpenDraftInBrowser(enabled bool) error {
	return a.setOpenDraftInBrowser(enabled)
}
