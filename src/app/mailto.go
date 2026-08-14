package main

import (
	"net/url"
	"strings"
)

// ARRICKS-09: mailto: protocol handler.
//
// The installer registers `go-mapi.exe --mailto "%1"` as the shell open
// command for the go-mapi.mailto ProgID and announces it through the
// Capabilities / RegisteredApplications model, so go-mapi is selectable as
// the mailto handler in Settings > Default apps (and via Intune
// DefaultAssociations policy — see docs/mailto-default-associations.xml).
//
// A mailto click means "compose right now", so this path opens Gmail's web
// compose prefilled from the URL and exits — no Wails init, no queue, no
// OAuth dependency (the browser's own Gmail session does the work). It
// runs before the single-instance gate in main() so a click while the
// tray app is running never triggers the raise-window path, and a click
// while it is NOT running never boots the full app.
//
// Consistent with the fork contract: nothing is ever sent. view=cm opens
// a compose window; the user still presses Send themselves.
//
// RFC 6068 notes:
//   - '+' in a mailto URL is a literal plus, NOT a space (that convention
//     belongs to HTML form encoding). url.ParseQuery would corrupt
//     "J+M Invoices" into "J M Invoices", so the header fields are split
//     manually and decoded with PathUnescape, which leaves '+' alone.
//   - Only to/cc/bcc/subject/body are honored. Everything else is dropped
//     per the RFC's guidance on unsafe headers — an "attach=" from a web
//     page must never reach the compose window.

// mailtoFields is the parsed, decoded content of a mailto URL.
type mailtoFields struct {
	To      []string
	CC      []string
	BCC     []string
	Subject string
	Body    string
}

// parseMailto parses a raw mailto URL. Unknown header fields are ignored;
// malformed percent-escapes in a field drop that field rather than failing
// the whole URL (a mail link should degrade to a mostly-filled compose,
// not a dead click). Returns nil only when raw does not start with
// "mailto:" (case-insensitive).
func parseMailto(raw string) *mailtoFields {
	const scheme = "mailto:"
	if len(raw) < len(scheme) || !strings.EqualFold(raw[:len(scheme)], scheme) {
		return nil
	}
	rest := raw[len(scheme):]

	addrPart := rest
	queryPart := ""
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		addrPart = rest[:i]
		queryPart = rest[i+1:]
	}

	f := &mailtoFields{}
	f.To = splitAddressList(addrPart)

	for _, pair := range strings.Split(queryPart, "&") {
		if pair == "" {
			continue
		}
		key, val := pair, ""
		if i := strings.IndexByte(pair, '='); i >= 0 {
			key, val = pair[:i], pair[i+1:]
		}
		decoded, err := url.PathUnescape(val)
		if err != nil {
			continue // malformed escape: drop this field, keep the rest
		}
		switch strings.ToLower(key) {
		case "to":
			// RFC 6068: a "to" header adds to the address-part recipients.
			f.To = append(f.To, splitAddressList(decoded)...)
		case "cc":
			f.CC = append(f.CC, splitAddressList(decoded)...)
		case "bcc":
			f.BCC = append(f.BCC, splitAddressList(decoded)...)
		case "subject":
			f.Subject = decoded
		case "body":
			f.Body = decoded
		}
		// Any other header (in-reply-to, attach, arbitrary X-*) is ignored.
	}
	return f
}

// splitAddressList percent-decodes a comma-separated address list into
// trimmed, non-empty entries. Used for the address part (still encoded)
// and for already-decoded to/cc/bcc values (decoding is idempotent for
// addresses, which contain no '%').
func splitAddressList(s string) []string {
	if decoded, err := url.PathUnescape(s); err == nil {
		s = decoded
	}
	var out []string
	for _, a := range strings.Split(s, ",") {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

// gmailComposeURL builds the mail.google.com prefilled-compose URL for the
// parsed fields. view=cm is Gmail's standalone compose view; fs=1 makes it
// full-screen. /u/0 targets the browser's first signed-in Google session —
// the fleet runs one Google account per machine, and unlike the ARRICKS-08
// draft deep link this path has no running App to ask for the account.
func gmailComposeURL(f *mailtoFields) string {
	v := url.Values{}
	v.Set("view", "cm")
	v.Set("fs", "1")
	if len(f.To) > 0 {
		v.Set("to", strings.Join(f.To, ","))
	}
	if len(f.CC) > 0 {
		v.Set("cc", strings.Join(f.CC, ","))
	}
	if len(f.BCC) > 0 {
		v.Set("bcc", strings.Join(f.BCC, ","))
	}
	if f.Subject != "" {
		v.Set("su", f.Subject)
	}
	if f.Body != "" {
		v.Set("body", f.Body)
	}
	return "https://mail.google.com/mail/u/0/?" + v.Encode()
}

// runMailtoHandler is the --mailto entry point. Always returns exit code 0:
// by the time Windows invokes us the user's click already "happened", so
// the kindest failure mode is a plain Gmail compose window rather than an
// error box. Reuses the openDraftURL seam (draftlink.go) so tests capture
// the URL instead of launching a browser.
func runMailtoHandler(raw string) int {
	f := parseMailto(raw)
	if f == nil {
		// Not a mailto URL at all — log the length only (the arg is
		// attacker-adjacent input from the shell; don't echo it) and fall
		// back to a bare compose window.
		logError("mailto: argument is not a mailto URL (len=%d); opening empty compose", len(raw))
		f = &mailtoFields{}
	}
	if err := openDraftURL(gmailComposeURL(f)); err != nil {
		logError("mailto: browser open failed: %v", err)
		return 1
	}
	return 0
}
