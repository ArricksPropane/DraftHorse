package main

import (
	"fmt"
	"net/url"

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

// draftComposeURL builds the mail.google.com deep link for a just-created
// draft. messageID is DraftResponse.Message.ID (NOT the draft id).
// accountEmail selects the Google account; empty falls back to /u/0/ (the
// browser's first signed-in session). Returns "" when messageID is empty —
// without it there is nothing to deep-link to.
func draftComposeURL(accountEmail, messageID string) string {
	if messageID == "" {
		return ""
	}
	account := "0"
	if accountEmail != "" {
		account = url.PathEscape(accountEmail)
	}
	return fmt.Sprintf("https://mail.google.com/mail/u/%s/#drafts?compose=%s",
		account, url.QueryEscape(messageID))
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
	if err := openDraftURL(draftComposeURL(email, messageID)); err != nil {
		// D-04: log + swallow. The draft exists; the user can still reach
		// it via mail.google.com.
		logError("draftlink: browser open failed: %v", err)
	}
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
