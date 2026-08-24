package mapi

// ARRICKS-24 tests: sendAs signature fetch + signature rendering in MIME.

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetPrimarySignature(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr string
	}{
		{
			name:   "default entry wins over non-default",
			status: 200,
			body:   `{"sendAs":[{"sendAsEmail":"alias@x.com","isDefault":false,"signature":"<b>alias</b>"},{"sendAsEmail":"scans@x.com","isDefault":true,"signature":"<b>Arrick's Propane</b>"}]}`,
			want:   "<b>Arrick's Propane</b>",
		},
		{
			name:   "no default falls back to first entry",
			status: 200,
			body:   `{"sendAs":[{"sendAsEmail":"a@x.com","signature":"sig-a"},{"sendAsEmail":"b@x.com","signature":"sig-b"}]}`,
			want:   "sig-a",
		},
		{
			name:   "empty list yields empty signature",
			status: 200,
			body:   `{"sendAs":[]}`,
			want:   "",
		},
		{
			name:    "403 explains the re-consent path",
			status:  403,
			body:    `{"error":"insufficient scopes"}`,
			wantErr: "sign out and back in",
		},
		{
			name:    "401 maps to the token-expired convention",
			status:  401,
			body:    `{}`,
			wantErr: "token expired",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/settings/sendAs" {
					t.Errorf("unexpected path %q", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer tok" {
					t.Errorf("Authorization = %q", got)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			gc := NewGmailClientWithBase("tok", srv.URL)
			got, err := gc.GetPrimarySignature()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetPrimarySignature: %v", err)
			}
			if got != tc.want {
				t.Errorf("signature = %q, want %q", got, tc.want)
			}
		})
	}
}

// decodeBodyPart pulls the base64 payload after the final blank line of a
// no-attachment MIME message and decodes it.
func decodeBodyPart(t *testing.T, mime []byte) string {
	t.Helper()
	s := string(mime)
	idx := strings.Index(s, "\r\n\r\n")
	if idx < 0 {
		t.Fatalf("no header/body separator in %q", s)
	}
	payload := strings.ReplaceAll(s[idx+4:], "\r\n", "")
	payload = strings.ReplaceAll(payload, "\n", "")
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("base64 decode: %v (payload %q)", err, payload)
	}
	return string(raw)
}

func TestBuildFullMIME_SignatureRendering(t *testing.T) {
	base := MailMessage{
		BodyFormat: "plain",
		Body:       "Scan attached.\nSee <notes> & follow up.",
		Subject:    "s",
		Recipients: Recipients{To: []Recipient{{Address: "ok@example.com"}}},
	}

	t.Run("plain body is promoted to escaped HTML with the signature", func(t *testing.T) {
		msg := base
		msg.Signature = `<b>Arrick's Propane</b>`
		out, err := BuildFullMIME(&msg)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), "Content-Type: text/html; charset=UTF-8") {
			t.Error("signed plain body must be promoted to text/html")
		}
		body := decodeBodyPart(t, out)
		for _, want := range []string{
			"Scan attached.<br>",
			"&lt;notes&gt; &amp; follow up.",
			`<div class="gmail_signature"><b>Arrick's Propane</b></div>`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("body missing %q:\n%s", want, body)
			}
		}
	})

	t.Run("html body gets the signature appended unescaped", func(t *testing.T) {
		msg := base
		msg.BodyFormat = "html"
		msg.Body = "<p>Scan attached.</p>"
		msg.Signature = "<i>sig</i>"
		out, err := BuildFullMIME(&msg)
		if err != nil {
			t.Fatal(err)
		}
		body := decodeBodyPart(t, out)
		if !strings.Contains(body, "<p>Scan attached.</p>") {
			t.Errorf("html body must pass through unescaped:\n%s", body)
		}
		if !strings.Contains(body, `<div class="gmail_signature"><i>sig</i></div>`) {
			t.Errorf("signature missing:\n%s", body)
		}
	})

	t.Run("no signature keeps the pre-ARRICKS-24 plain output", func(t *testing.T) {
		msg := base
		out, err := BuildFullMIME(&msg)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), "Content-Type: text/plain; charset=UTF-8") {
			t.Error("unsigned plain body must stay text/plain")
		}
		if got := decodeBodyPart(t, out); got != base.Body {
			t.Errorf("unsigned body changed: %q, want %q", got, base.Body)
		}
	})
}
