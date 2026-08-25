//go:build windows

package main

// ARRICKS-30 tests: the toast XML that reaches XmlLite must parse.
//
// The regression these lock down shipped silently for months — every arrival
// toast died at LoadXml because its action arguments carry a raw "&", so the
// queue's main manual-mode surface never appeared. The assertions are written
// against DECODED values rather than the raw XML on purpose: whether the fix
// escapes or the upstream template starts doing it for us, what must hold is
// that the string the activation callback receives is the string we sent.

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
)

// decodeXMLValues returns every attribute value and text node in doc, with
// XML escapes already decoded by the parser.
func decodeXMLValues(t *testing.T, doc string) []string {
	t.Helper()
	var out []string
	dec := xml.NewDecoder(strings.NewReader(doc))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			for _, attr := range el.Attr {
				out = append(out, attr.Value)
			}
		case xml.CharData:
			if s := strings.TrimSpace(string(el)); s != "" {
				out = append(out, s)
			}
		}
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// arrivalShaped mirrors emitArrivalToast — the toast that never worked.
func arrivalShaped() toast.Notification {
	return toast.Notification{
		AppID:               "com.marcfargas.gomapi.test",
		Title:               "To: Accounts & Billing",
		Body:                "Scan 2026-08-25\n📎 1 attachment(s)",
		ActivationType:      toast.Foreground,
		ActivationArguments: "action=open&emailId=abc123",
		Actions: []toast.Action{
			{Type: toast.Foreground, Content: "Create draft", Arguments: "action=create-draft&emailId=abc123"},
			{Type: toast.Foreground, Content: "Dismiss", Arguments: "action=dismiss&emailId=abc123"},
		},
	}
}

func TestBuildToastXMLArrivalToastParses(t *testing.T) {
	got, err := buildToastXML(arrivalShaped())
	if err != nil {
		t.Fatalf("buildToastXML: %v", err)
	}
	if !wellFormedXML(got) {
		t.Fatal("arrival toast XML does not parse — this is the LoadXml 0xc00ce50d failure")
	}
}

func TestBuildToastXMLPreservesActivationArguments(t *testing.T) {
	got, err := buildToastXML(arrivalShaped())
	if err != nil {
		t.Fatalf("buildToastXML: %v", err)
	}
	values := decodeXMLValues(t, got)

	// Routing depends on these surviving verbatim through the round trip:
	// handleToastAction parses "action=...&emailId=...".
	for _, want := range []string{
		"action=open&emailId=abc123",
		"action=create-draft&emailId=abc123",
		"action=dismiss&emailId=abc123",
	} {
		if !contains(values, want) {
			t.Errorf("decoded values missing %q — action routing would break; got %q", want, values)
		}
	}
	// Double-escaping would surface as a literal "&amp;" AFTER decoding.
	for _, v := range values {
		if strings.Contains(v, "&amp;") {
			t.Errorf("value %q is double-escaped", v)
		}
	}
}

func TestBuildToastXMLPreservesSubjectText(t *testing.T) {
	got, err := buildToastXML(arrivalShaped())
	if err != nil {
		t.Fatalf("buildToastXML: %v", err)
	}
	values := decodeXMLValues(t, got)
	if !contains(values, "To: Accounts & Billing") {
		t.Errorf("title did not round-trip; got %q", values)
	}
}

// A toast with nothing to escape must come back exactly as the upstream
// template rendered it — the escape pass is a fallback, not a filter.
func TestBuildToastXMLLeavesCleanPayloadUntouched(t *testing.T) {
	n := toast.Notification{
		AppID:               "com.marcfargas.gomapi.test",
		Title:               "DraftHorse",
		Body:                "Draft created",
		ActivationType:      toast.Foreground,
		ActivationArguments: "action=open",
	}
	got, err := buildToastXML(n)
	if err != nil {
		t.Fatalf("buildToastXML: %v", err)
	}
	// Same defaults buildToastXML applies before rendering.
	n.Duration = toast.Short
	n.Audio = toast.Default
	want, err := renderToastXML(n)
	if err != nil {
		t.Fatalf("renderToastXML: %v", err)
	}
	if got != want {
		t.Errorf("clean payload was rewritten:\n got %q\nwant %q", got, want)
	}
}

// The retry path must not corrupt the caller's notification.
func TestEscapeToastFieldsDoesNotMutateCallerActions(t *testing.T) {
	n := arrivalShaped()
	original := n.Actions[0].Arguments
	_ = escapeToastFields(n)
	if n.Actions[0].Arguments != original {
		t.Errorf("caller's Actions slice was mutated: %q became %q", original, n.Actions[0].Arguments)
	}
}

func TestWellFormedXML(t *testing.T) {
	if !wellFormedXML(`<toast launch="action=open"><visual/></toast>`) {
		t.Error("valid document reported malformed")
	}
	if wellFormedXML(`<toast launch="action=open&emailId=1"></toast>`) {
		t.Error("bare ampersand in an attribute must be reported malformed")
	}
}
