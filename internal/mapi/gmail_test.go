package mapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GOTEST-01: HTTP-level tests for GmailClient.CreateDraft.
//
// Uses httptest.Server via the FOUND-03 NewGmailClientWithBase injection
// point so the real Gmail endpoint is never touched. Covers happy path,
// authentication failure, server-side error, network failure, and
// response-body parse error.

// newTestMail returns a minimal MailMessage that BuildFullMIME can encode
// without touching the filesystem (no attachments).
func newTestMail() *MailMessage {
	return &MailMessage{
		Version:    1,
		Timestamp:  "2026-04-10T00:00:00Z",
		Subject:    "Gmail client test",
		Body:       "body text",
		BodyFormat: "plain",
		Recipients: Recipients{
			To: []Recipient{{Name: "Alice", Address: "alice@example.com"}},
		},
	}
}

func TestGmailClient_CreateDraft(t *testing.T) {
	type stubHandler struct {
		status int
		body   string
	}

	cases := []struct {
		name         string
		stub         stubHandler
		closeServer  bool // true = start then close, simulating network failure
		wantID       string
		wantErrSub   string
		expectCalled bool // false only when server is closed before the call
	}{
		{
			name:         "happy path returns draft id",
			stub:         stubHandler{status: 200, body: `{"id":"draft_abc123"}`},
			wantID:       "draft_abc123",
			expectCalled: true,
		},
		{
			name:         "401 unauthorized surfaces token expired",
			stub:         stubHandler{status: 401, body: `{"error":"unauthorized"}`},
			wantErrSub:   "token expired",
			expectCalled: true,
		},
		{
			name:         "500 server error surfaces gmail api error",
			stub:         stubHandler{status: 500, body: `{"error":"internal"}`},
			wantErrSub:   "Gmail API error (500)",
			expectCalled: true,
		},
		{
			name:         "200 with non-json body surfaces parse error",
			stub:         stubHandler{status: 200, body: `not-json-at-all`},
			wantErrSub:   "failed to parse response",
			expectCalled: true,
		},
		{
			name:         "network failure when server is closed",
			closeServer:  true,
			wantErrSub:   "failed to create draft",
			expectCalled: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var (
				gotMethod string
				gotPath   string
				gotAuth   string
				called    bool
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// ARRICKS-25: a successful create is followed by a warm GET;
				// record only the FIRST request (the create under test) so
				// the warm read doesn't overwrite the captured method/path.
				if !called {
					called = true
					gotMethod = r.Method
					gotPath = r.URL.Path
					gotAuth = r.Header.Get("Authorization")
				}
				// Drain the request body so the client sees a clean response cycle.
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.stub.status)
				_, _ = io.WriteString(w, tc.stub.body)
			}))

			baseURL := srv.URL
			if tc.closeServer {
				// Close the server first so the client gets a connection error.
				srv.Close()
			} else {
				defer srv.Close()
			}

			client := NewGmailClientWithBase("test-token", baseURL)
			id, err := client.CreateDraft(newTestMail())

			if tc.wantErrSub == "" {
				if err != nil {
					t.Fatalf("CreateDraft unexpected error: %v", err)
				}
				if id != tc.wantID {
					t.Fatalf("CreateDraft id = %q, want %q", id, tc.wantID)
				}
			} else {
				if err == nil {
					t.Fatalf("CreateDraft expected error containing %q, got nil (id=%q)", tc.wantErrSub, id)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("CreateDraft error = %q, want substring %q", err.Error(), tc.wantErrSub)
				}
			}

			if tc.expectCalled {
				if !called {
					t.Fatalf("expected server to be called, wasn't")
				}
				if gotMethod != http.MethodPost {
					t.Errorf("request method = %q, want POST", gotMethod)
				}
				if gotPath != "/drafts" {
					t.Errorf("request path = %q, want /drafts", gotPath)
				}
				if gotAuth != "Bearer test-token" {
					t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
				}
			}
		})
	}
}

func TestGmailClient_CreateDraft_RequestBodyShape(t *testing.T) {
	// ARRICKS-26: creation goes through the media-upload endpoint — the
	// request body IS the raw RFC822 message (no JSON envelope, no base64
	// inflation), marked by uploadType=media and message/rfc822. Keeps us
	// honest against accidental refactors of the wire format.
	var (
		gotBody        string
		gotContentType string
		gotUploadType  string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ARRICKS-25: the create is followed by a warm GET — only the POST
		// carries the body under test here.
		if r.Method != http.MethodPost {
			w.WriteHeader(200)
			_, _ = io.WriteString(w, `{}`)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		gotBody = string(raw)
		gotContentType = r.Header.Get("Content-Type")
		gotUploadType = r.URL.Query().Get("uploadType")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `{"id":"abc"}`)
	}))
	defer srv.Close()

	client := NewGmailClientWithBase("t", srv.URL)
	if _, err := client.CreateDraft(newTestMail()); err != nil {
		t.Fatalf("CreateDraft error: %v", err)
	}
	if gotUploadType != "media" {
		t.Errorf("uploadType = %q, want media", gotUploadType)
	}
	if gotContentType != "message/rfc822" {
		t.Errorf("Content-Type = %q, want message/rfc822", gotContentType)
	}
	if !strings.Contains(gotBody, "Subject: ") {
		t.Errorf("body is not raw RFC822 (no Subject header):\n%s", gotBody)
	}
	if strings.Contains(gotBody, `"raw"`) {
		t.Errorf("body still carries the old JSON envelope:\n%s", gotBody)
	}
}

// ARRICKS-08: CreateDraftFull surfaces the backing message id for the
// open-in-browser deep link; CreateDraft keeps returning the draft id only.
func TestGmailClient_CreateDraftFull_MessageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"r-draft1","message":{"id":"18f0aa55","threadId":"t1","labelIds":["DRAFT"]}}`)
	}))
	defer srv.Close()

	client := NewGmailClientWithBase("test-token", srv.URL)

	draft, err := client.CreateDraftFull(newTestMail())
	if err != nil {
		t.Fatalf("CreateDraftFull error: %v", err)
	}
	if draft.ID != "r-draft1" {
		t.Errorf("draft id = %q, want %q", draft.ID, "r-draft1")
	}
	if draft.Message.ID != "18f0aa55" {
		t.Errorf("message id = %q, want %q", draft.Message.ID, "18f0aa55")
	}

	// The thin wrapper's contract is unchanged.
	id, err := client.CreateDraft(newTestMail())
	if err != nil {
		t.Fatalf("CreateDraft error: %v", err)
	}
	if id != "r-draft1" {
		t.Errorf("CreateDraft id = %q, want %q", id, "r-draft1")
	}
}
