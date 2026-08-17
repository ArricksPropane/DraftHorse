package main

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// ARRICKS-09 tests: mailto parsing (RFC 6068 edge cases), Gmail compose URL
// construction, and the --mailto handler's fallback behavior.

func TestParseMailto(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want *mailtoFields
	}{
		{
			name: "bare address",
			raw:  "mailto:office@arrickspropane.com",
			want: &mailtoFields{To: []string{"office@arrickspropane.com"}},
		},
		{
			name: "subject and body",
			raw:  "mailto:a@b.com?subject=Propane%20order&body=Tank%20refill%20please",
			want: &mailtoFields{
				To:      []string{"a@b.com"},
				Subject: "Propane order",
				Body:    "Tank refill please",
			},
		},
		{
			name: "plus is a literal plus, not a space (RFC 6068)",
			raw:  "mailto:a@b.com?subject=J%2BM+Invoices",
			want: &mailtoFields{To: []string{"a@b.com"}, Subject: "J+M+Invoices"},
		},
		{
			name: "multiple recipients, cc, bcc, to-header appends",
			raw:  "mailto:a@b.com,c@d.com?to=e@f.com&cc=g@h.com&bcc=i@j.com",
			want: &mailtoFields{
				To:  []string{"a@b.com", "c@d.com", "e@f.com"},
				CC:  []string{"g@h.com"},
				BCC: []string{"i@j.com"},
			},
		},
		{
			name: "no address, subject only",
			raw:  "mailto:?subject=hello",
			want: &mailtoFields{Subject: "hello"},
		},
		{
			name: "unknown and unsafe headers are dropped",
			raw:  "mailto:a@b.com?attach=C:%5Csecret.txt&x-custom=1&in-reply-to=%3Cid%3E&subject=ok",
			want: &mailtoFields{To: []string{"a@b.com"}, Subject: "ok"},
		},
		{
			name: "header keys are case-insensitive",
			raw:  "mailto:a@b.com?Subject=Hi&CC=c@d.com",
			want: &mailtoFields{To: []string{"a@b.com"}, CC: []string{"c@d.com"}, Subject: "Hi"},
		},
		{
			name: "percent-encoded address part",
			raw:  "mailto:a%40b.com",
			want: &mailtoFields{To: []string{"a@b.com"}},
		},
		{
			name: "malformed escape drops that field only",
			raw:  "mailto:a@b.com?subject=%ZZbroken&body=still%20here",
			want: &mailtoFields{To: []string{"a@b.com"}, Body: "still here"},
		},
		{
			name: "scheme is case-insensitive",
			raw:  "MAILTO:a@b.com",
			want: &mailtoFields{To: []string{"a@b.com"}},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := parseMailto(tc.raw)
			if got == nil {
				t.Fatalf("parseMailto(%q) = nil, want %+v", tc.raw, tc.want)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseMailto(%q) = %+v, want %+v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestParseMailtoRejectsNonMailto(t *testing.T) {
	for _, raw := range []string{"", "http://evil.example/", "mailt", "mailto"} {
		if got := parseMailto(raw); got != nil {
			t.Errorf("parseMailto(%q) = %+v, want nil", raw, got)
		}
	}
}

func TestGmailComposeURL(t *testing.T) {
	f := &mailtoFields{
		To:      []string{"a@b.com", "c@d.com"},
		CC:      []string{"e@f.com"},
		Subject: "Propane order",
		Body:    "Tank refill & delivery",
	}
	got := gmailComposeURL(f)

	if !strings.HasPrefix(got, "https://mail.google.com/mail/u/0/?") {
		t.Fatalf("unexpected base: %q", got)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("compose URL does not parse: %v", err)
	}
	q := u.Query()
	if q.Get("view") != "cm" || q.Get("fs") != "1" {
		t.Errorf("missing view=cm/fs=1: %q", got)
	}
	if q.Get("to") != "a@b.com,c@d.com" {
		t.Errorf("to = %q", q.Get("to"))
	}
	if q.Get("cc") != "e@f.com" {
		t.Errorf("cc = %q", q.Get("cc"))
	}
	if q.Get("su") != "Propane order" {
		t.Errorf("su = %q", q.Get("su"))
	}
	if q.Get("body") != "Tank refill & delivery" {
		t.Errorf("body = %q", q.Get("body"))
	}
	if q.Has("bcc") {
		t.Errorf("empty bcc should be omitted: %q", got)
	}
}

func TestRunMailtoHandler(t *testing.T) {
	restore := openDraftURL
	defer func() { openDraftURL = restore }()

	var opened []string
	openDraftURL = func(u string) error {
		opened = append(opened, u)
		return nil
	}

	// Normal mailto → prefilled compose, exit 0.
	if code := runMailtoHandler("mailto:a@b.com?subject=hi"); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(opened) != 1 || !strings.Contains(opened[0], "to=a%40b.com") {
		t.Fatalf("opened = %v, want prefilled compose", opened)
	}

	// Non-mailto argument → bare compose window, still exit 0.
	opened = nil
	if code := runMailtoHandler("http://not-mail.example/"); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "https://mail.google.com/mail/u/0/?fs=1&view=cm"
	if len(opened) != 1 || opened[0] != want {
		t.Fatalf("opened = %v, want [%s]", opened, want)
	}
}
