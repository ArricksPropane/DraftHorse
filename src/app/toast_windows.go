//go:build windows

package main

// toast_windows.go is the Windows implementation of the toast notification
// subsystem for DraftHorse. Uses jackmordaunt/go-toast/v2 for COM activator
// registration and toast_shim_windows.go for Tag/Group/ClearToast (NOTIF-05).
//
// Privacy (QUAL-03): Toast payloads include ONLY:
//   - Title: sender display name (never email address if display name exists)
//   - Body: subject + "📎 N attachment(s)" (never body text, filenames, recipients)
//   - Icon: absolute path to app icon (not email content)
//
// D-11: Arrival + draft-success toasts suppressed when main window is visible
// and focused; error toasts always fire.

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/marcfargas/go-mapi/internal/mapi"
)

// initToasts registers the COM activator + activation callback.
// MUST be called before any emitXxxToast call.
// Safe to call more than once; underlying library is idempotent on SetAppData.
func initToasts(a *App) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("toast: resolve exe path: %w", err)
	}
	// Cache exe path for later calls.
	cachedExePath = exe

	icon := toastIconPath(exe)
	if err := toast.SetAppData(toast.AppData{
		AppID:         activeAUMID(),
		GUID:          toastActivatorGUID,
		ActivationExe: exe,
		IconPath:      icon,
	}); err != nil {
		return fmt.Errorf("toast: SetAppData: %w", err)
	}
	toast.SetActivationCallback(func(args string, _ []toast.UserData) {
		// COM thread — dispatch to a goroutine. Never do business logic here.
		go a.handleToastAction(args)
	})
	logInfo("toast: initialized (aumid=%s)", activeAUMID())
	return nil
}

// toastIconPath returns the absolute path to the app icon used in toast visuals.
// jackmordaunt/go-toast requires an absolute path; the icon must be an .ico or .png.
// We ship DraftHorse.ico alongside the exe in both dev and prod layouts.
func toastIconPath(exePath string) string {
	return filepath.Join(filepath.Dir(exePath), "DraftHorse.ico")
}

// cachedExePath is populated by initToasts; avoids repeated os.Executable calls.
var cachedExePath string

// mustExePath returns the cached exe path, calling os.Executable on first use.
// Panics on failure — the toast subsystem cannot function without it.
func mustExePath() string {
	if cachedExePath == "" {
		exe, err := os.Executable()
		if err != nil {
			panic(fmt.Errorf("toast: os.Executable: %w", err))
		}
		cachedExePath = exe
	}
	return cachedExePath
}

// windowFocused returns whether the main Wails window currently has input focus.
// Wails runtime does not expose a direct "is focused" query; we approximate
// by checking isVisible(). Plan 09 can refine with a Svelte-side focus listener.
func windowFocused(a *App) bool {
	return a.isVisible()
}

// emitArrivalToast fires a toast for a newly-arrived email. Suppressed when
// the main window is visible AND focused (D-11), or paused (D-14).
//
// V4 retest feedback (Dave, 2026-08-27) reshaped it twice:
//
// 1. Title is the app name, "DraftHorse", like every other toast. It used
//    to be the recipient or, for scans (which have no recipient yet), the
//    ORIGINATING APP's name — an unclear title from software the user
//    never thinks about. The recipient/origin line moved into the body.
//
// 2. Mode decides the SHAPE (Dave's explicit choice: informational in auto
//    mode, not suppressed). Auto-draft mode gets a button-less status toast
//    ("Creating Gmail draft…") — automode drafts within seconds of arrival,
//    so "Create draft" / "Dismiss" buttons were promises the app had
//    already broken: Dismiss removed the queue row while the draft existed
//    anyway. The clear-on-processed plumbing (NOTIF-05) then removes the
//    status toast when the draft lands and the success toast replaces it.
//    Manual mode keeps the buttons — nothing drafts without a click there,
//    so they are honest.
//
// Privacy: body = recipient display line + subject + attachment count.
// NEVER includes attachment filenames, recipient list, or body text (QUAL-03).
func emitArrivalToast(a *App, e mapi.EmailWithId) {
	if a.isVisible() && windowFocused(a) {
		return
	}
	if a.isPaused() {
		return
	}
	if e.Message == nil {
		return
	}
	n := arrivalToastNotification(a.getMode(), e)
	if err := shimPushWithTagGroup(activeAUMID(), n, e.Id, toastGroup); err != nil {
		// Privacy-safe log: id prefix + error class only.
		logError("toast: arrival push failed for %s: %v", safeIDPrefix(e.Id), err)
	}
}

// arrivalToastNotification builds the arrival toast for the given mode —
// pure with respect to App state, so tests can assert the mode split
// (buttons vs informational) without touching COM.
func arrivalToastNotification(mode string, e mapi.EmailWithId) toast.Notification {
	body := arrivalToastBody(e.Message)
	n := toast.Notification{
		AppID:               activeAUMID(),
		Title:               "DraftHorse",
		Body:                body,
		Icon:                toastIconPath(mustExePath()),
		ActivationType:      toast.Foreground,
		ActivationArguments: fmt.Sprintf("action=open&emailId=%s", url.QueryEscape(e.Id)),
	}
	if mode == "auto-draft" {
		// Status line first, details under it. No buttons: the draft is
		// already being created; clicking the toast opens the window.
		n.Body = "Creating Gmail draft…\n" + body
		return n
	}
	n.Actions = []toast.Action{
		{
			Type:      toast.Foreground,
			Content:   "Create draft",
			Arguments: fmt.Sprintf("action=create-draft&emailId=%s", url.QueryEscape(e.Id)),
		},
		{
			Type:      toast.Foreground,
			Content:   "Dismiss",
			Arguments: fmt.Sprintf("action=dismiss&emailId=%s", url.QueryEscape(e.Id)),
		},
	}
	return n
}

// emitDraftSuccessToast fires only when the window is hidden (D-04 + D-11).
// No action buttons — dismissible only. Subject is included per UI-SPEC copywriting.
func emitDraftSuccessToast(a *App, subject, emailID string) {
	if a.isVisible() && windowFocused(a) {
		return
	}
	if a.isPaused() {
		return
	}
	n := toast.Notification{
		AppID:               activeAUMID(),
		Title:               "Draft created: " + subject,
		Icon:                toastIconPath(mustExePath()),
		ActivationType:      toast.Foreground,
		ActivationArguments: "action=open",
	}
	// Use emailID+":success" as the tag so it's distinct from the arrival toast.
	// The ":success" toast is not cleared by clearToastForEmail — left for the
	// user to dismiss (it confirms a successful draft).
	tag := emailID + ":success"
	if err := shimPushWithTagGroup(activeAUMID(), n, tag, toastGroup); err != nil {
		logError("toast: draft-success push failed for %s: %v", safeIDPrefix(emailID), err)
	}
}

// emitErrorToast fires regardless of window state (D-11: errors always surface).
// Per D-09: error category drives the copy via toastErrorCopy.
func emitErrorToast(a *App, category, emailID string) {
	n := toast.Notification{
		AppID:               activeAUMID(),
		Title:               "DraftHorse",
		Body:                toastErrorCopy(category),
		Icon:                toastIconPath(mustExePath()),
		ActivationType:      toast.Foreground,
		ActivationArguments: "action=open",
	}
	// Include emailID in tag so subsequent MarkProcessed/Delete can clear it.
	tag := emailID + ":err"
	if err := shimPushWithTagGroup(activeAUMID(), n, tag, toastGroup); err != nil {
		logError("toast: error push failed for %s: %v", safeIDPrefix(emailID), err)
	}
}

// emitSummaryInvalidGrantToast fires the D-10 one-shot summary on the first
// invalid_grant during a drain. Fires regardless of window state (error class).
func emitSummaryInvalidGrantToast(_ *App) {
	n := toast.Notification{
		AppID:               activeAUMID(),
		Title:               "DraftHorse",
		Body:                toastCopySummaryInvalidGrant,
		Icon:                toastIconPath(mustExePath()),
		ActivationType:      toast.Foreground,
		ActivationArguments: "action=open",
	}
	if err := shimPushWithTagGroup(activeAUMID(), n, "summary:invalid-grant", toastGroup); err != nil {
		logError("toast: summary invalid_grant push failed: %v", err)
	}
}

// emitSignatureScopeToast fires the ARRICKS-29 one-shot: the stored OAuth
// grant predates gmail.settings.basic, so every draft this session ships
// unsigned until the user re-authorizes.
//
// Treated as an error-class toast (fires regardless of window state, D-11):
// it reports work that already came out wrong, and the tray row is the only
// other surface. draftSignature guarantees it fires at most once per session
// — a backlog drain must not produce one toast per queued email.
//
// No action buttons: sign-out lives in the main window, and a toast button
// that silently revoked the user's session would be a worse surprise than
// the missing signature. Clicking the toast opens the window.
func emitSignatureScopeToast(_ *App) {
	n := toast.Notification{
		AppID:               activeAUMID(),
		Title:               toastCopySignatureScopeTitle,
		Body:                toastCopySignatureScopeBody,
		Icon:                toastIconPath(mustExePath()),
		ActivationType:      toast.Foreground,
		ActivationArguments: "action=open",
	}
	if err := shimPushWithTagGroup(activeAUMID(), n, "signature:scope-missing", toastGroup); err != nil {
		logError("toast: signature-scope push failed: %v", err)
	}
}

// clearToastForEmail removes the toast(s) associated with a processed email
// from Action Center (NOTIF-05). Called after MarkProcessed / Delete.
// Clears the arrival toast (tag = emailID) and the error toast (tag = emailID+":err").
// The ":success" tag is NOT cleared — left for the user to acknowledge.
func clearToastForEmail(emailID string) {
	aumid := activeAUMID()
	for _, tag := range []string{emailID, emailID + ":err"} {
		if err := shimClearToast(aumid, tag, toastGroup); err != nil {
			logError("toast: clear %s failed: %v", tag, err)
		}
	}
}

// arrivalToastBody builds the toast body: an optional recipient line (name
// preferred over address — QUAL-03), the subject, and the attachment count.
// Scans typically have no recipient and a scanner-generated subject, so any
// of the three lines may be absent. Never logged (QUAL-03).
func arrivalToastBody(msg *mapi.MailMessage) string {
	lines := []string{}
	if len(msg.Recipients.To) > 0 {
		r := msg.Recipients.To[0]
		if r.Name != "" {
			lines = append(lines, "To: "+r.Name)
		} else if r.Address != "" {
			lines = append(lines, "To: "+r.Address)
		}
	}
	if msg.Subject != "" {
		lines = append(lines, msg.Subject)
	}
	if c := len(msg.Attachments); c > 0 {
		lines = append(lines, fmt.Sprintf("📎 %d attachment(s)", c))
	}
	if len(lines) == 0 {
		return "New email ready to draft"
	}
	return strings.Join(lines, "\n")
}
