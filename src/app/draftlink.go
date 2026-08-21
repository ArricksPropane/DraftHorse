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

// draftComposeURL builds the deep link for a just-created draft.
// messageID is DraftResponse.Message.ID (NOT the draft id). Returns "" when
// messageID is empty — without it there is nothing to deep-link to.
//
// ARRICKS-18: when the account email is known, the mail.google.com link is
// wrapped in accounts.google.com/AccountChooser. Gmail's bare /u/<email>
// slot only resolves against sessions ALREADY signed in to that browser —
// when the scanner PC's Chrome has no session for the Workspace account,
// it serves a dead "Temporary Error (404) / Numeric Code: 6446" page
// (observed in validation). AccountChooser degrades properly instead: an
// existing session passes straight through to the draft; a missing one
// gets Google's sign-in prefilled with the right address, and `continue`
// then lands on the draft. Empty email keeps the plain /u/0 link, which
// already falls back to a normal sign-in redirect on its own.
func draftComposeURL(accountEmail, messageID string) string {
	if messageID == "" {
		return ""
	}
	if accountEmail == "" {
		return fmt.Sprintf("https://mail.google.com/mail/u/0/#drafts?compose=%s",
			url.QueryEscape(messageID))
	}
	target := fmt.Sprintf("https://mail.google.com/mail/u/%s/#drafts?compose=%s",
		url.PathEscape(accountEmail), url.QueryEscape(messageID))
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
	target := draftComposeURL(email, messageID)
	// ARRICKS-21: prefer the dedicated isolated profile (see draftbrowser.go);
	// fall through to the default browser if no Edge/Chrome is installed or
	// the launch fails, so the draft is never left unreachable.
	if a.isDraftBrowserDedicated() {
		err := launchDraftInDedicatedBrowser(target)
		if err == nil {
			return
		}
		logError("draftlink: dedicated browser launch failed (%v); falling back to default browser", err)
	}
	if err := openDraftURL(target); err != nil {
		// D-04: log + swallow. The draft exists; the user can still reach
		// it via mail.google.com.
		logError("draftlink: browser open failed: %v", err)
	}
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
