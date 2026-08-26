//go:build windows

package main

// V4 arrival-toast fixes (Dave's retest feedback): title is the app name,
// details live in the body, and auto-draft mode suppresses the toast
// entirely (its buttons promised choices automode had already made).

import (
	"strings"
	"testing"

	"github.com/marcfargas/go-mapi/internal/mapi"
)

func TestArrivalToastBodyScanShape(t *testing.T) {
	// The scan-to-email case: no recipient yet, scanner subject, one PDF.
	msg := &mapi.MailMessage{Subject: "Scan 2026-08-27"}
	msg.Attachments = []mapi.Attachment{{}}
	body := arrivalToastBody(msg)
	if strings.Contains(body, "To:") {
		t.Errorf("no recipient must mean no To: line, got %q", body)
	}
	if !strings.Contains(body, "Scan 2026-08-27") || !strings.Contains(body, "1 attachment") {
		t.Errorf("body = %q, want subject + attachment count", body)
	}
}

func TestArrivalToastBodyPrefersRecipientName(t *testing.T) {
	msg := &mapi.MailMessage{Subject: "Invoice"}
	msg.Recipients.To = []mapi.Recipient{{Name: "Accounts", Address: "a@x.com"}}
	body := arrivalToastBody(msg)
	if !strings.HasPrefix(body, "To: Accounts") {
		t.Errorf("body = %q, want name preferred over address (QUAL-03)", body)
	}
	if strings.Contains(body, "a@x.com") {
		t.Errorf("body = %q must not leak the address when a name exists", body)
	}
}

func TestArrivalToastBodyNeverEmpty(t *testing.T) {
	if got := arrivalToastBody(&mapi.MailMessage{}); got == "" {
		t.Error("an empty message must still produce a readable body")
	}
}

func TestEmitArrivalToastSuppressedInAutoDraftMode(t *testing.T) {
	// In auto-draft mode the function must return BEFORE any COM work —
	// reaching the push in this headless test environment would fail loudly,
	// so surviving the call with mode=auto-draft is itself the assertion.
	app := &App{}
	app.settings.Mode = "auto-draft"
	emitArrivalToast(app, mapi.EmailWithId{Id: "x", Message: &mapi.MailMessage{Subject: "s"}})
}
