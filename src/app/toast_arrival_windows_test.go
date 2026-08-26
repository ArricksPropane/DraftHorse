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

func TestArrivalToastAutoModeIsInformational(t *testing.T) {
	// Dave's choice: auto mode keeps an arrival pop-up, but button-less —
	// automode has already claimed the email, so "Create draft" / "Dismiss"
	// would promise choices already made.
	e := mapi.EmailWithId{Id: "x", Message: &mapi.MailMessage{Subject: "Scan"}}
	n := arrivalToastNotification("auto-draft", e)
	if len(n.Actions) != 0 {
		t.Errorf("auto-draft arrival toast must have no buttons, got %d", len(n.Actions))
	}
	if !strings.Contains(n.Body, "Creating Gmail draft") {
		t.Errorf("auto-draft body = %q, want the status line", n.Body)
	}
	if n.Title != "DraftHorse" {
		t.Errorf("title = %q, want DraftHorse", n.Title)
	}
}

func TestArrivalToastManualModeKeepsButtons(t *testing.T) {
	e := mapi.EmailWithId{Id: "x", Message: &mapi.MailMessage{Subject: "Scan"}}
	n := arrivalToastNotification("manual", e)
	if len(n.Actions) != 2 {
		t.Fatalf("manual arrival toast must keep Create draft + Dismiss, got %d actions", len(n.Actions))
	}
	if n.Actions[0].Content != "Create draft" || n.Actions[1].Content != "Dismiss" {
		t.Errorf("actions = %q, %q", n.Actions[0].Content, n.Actions[1].Content)
	}
	if strings.Contains(n.Body, "Creating Gmail draft") {
		t.Errorf("manual body must not claim a draft is being created: %q", n.Body)
	}
	if n.Title != "DraftHorse" {
		t.Errorf("title = %q, want DraftHorse", n.Title)
	}
}
