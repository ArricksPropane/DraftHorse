package mapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	GmailAPIBase = "https://www.googleapis.com/gmail/v1/users/me"

	// ARRICKS-12 (R9): Gmail's 25MB limit applies to the ENCODED message.
	// Base64 inflates raw bytes by 4/3, so a 25MB raw cap let 19-25MB scans
	// through to a guaranteed Gmail rejection — which Auto mode then retried
	// forever. 18MB raw ≈ 24MB encoded, leaving headroom for headers + body.
	// MaxTotalAttachmentSize caps the SUM across attachments for the same
	// reason: five 5MB pages fail exactly like one 25MB file.
	MaxFileSize            = 18 * 1024 * 1024
	MaxTotalAttachmentSize = 18 * 1024 * 1024
)

// GmailClient handles Gmail API operations
type GmailClient struct {
	httpClient *http.Client
	token      string
	baseURL    string // injection point for tests and CLI flag; defaults to GmailAPIBase
}

// NewGmailClient creates a new Gmail API client with the given OAuth token
// using the default Gmail API base URL. For tests or alternate endpoints,
// use NewGmailClientWithBase.
func NewGmailClient(token string) *GmailClient {
	return NewGmailClientWithBase(token, GmailAPIBase)
}

// NewGmailClientWithBase creates a Gmail API client with an explicit base URL.
// Used by tests (httptest.Server) and by the native host when --gmail-api-base
// is passed on the command line.
// If baseURL is empty, the default GmailAPIBase is used.
func NewGmailClientWithBase(token, baseURL string) *GmailClient {
	if baseURL == "" {
		baseURL = GmailAPIBase
	}
	return &GmailClient{
		httpClient: &http.Client{},
		token:      token,
		baseURL:    baseURL,
	}
}

// DraftResponse represents a Gmail API draft creation response.
// ID is the draft id (drafts.* API namespace). Message.ID is the immutable
// message id backing the draft — it is the value mail.google.com's
// ?compose= deep link expects (ARRICKS-08), NOT the draft id.
type DraftResponse struct {
	ID      string `json:"id"`
	Message struct {
		ID string `json:"id"`
	} `json:"message"`
}

// CreateDraft creates a Gmail draft from a MailMessage, including attachments.
// Returns the draft id only; callers that need the backing message id (for
// the ARRICKS-08 open-in-browser deep link) should use CreateDraftFull.
func (gc *GmailClient) CreateDraft(msg *MailMessage) (string, error) {
	draft, err := gc.CreateDraftFull(msg)
	if err != nil {
		return "", err
	}
	return draft.ID, nil
}

// CreateDraftFull creates a Gmail draft and returns the full API response.
// Builds the full MIME message locally (one API call, no round-trips).
func (gc *GmailClient) CreateDraftFull(msg *MailMessage) (*DraftResponse, error) {
	// Build full MIME message with attachments
	mimeMsg, err := BuildFullMIME(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to build MIME message: %w", err)
	}

	encodedMsg := Base64URLEncode(mimeMsg)

	body := map[string]interface{}{
		"message": map[string]interface{}{
			"raw": encodedMsg,
		},
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/drafts", gc.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+gc.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := gc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create draft: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("token expired")
	}
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gmail API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var draft DraftResponse
	if err := json.NewDecoder(resp.Body).Decode(&draft); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &draft, nil
}

// BuildFullMIME builds a complete RFC 2822 message from a MailMessage,
// including attachments as MIME parts. Single-pass, no network calls.
func BuildFullMIME(msg *MailMessage) ([]byte, error) {
	var buf bytes.Buffer

	hasAttachments := len(msg.Attachments) > 0
	boundary := fmt.Sprintf("go_mapi_%d", os.Getpid())

	// ARRICKS-12 (R4): addresses are written into headers verbatim below
	// (RFC 2047 encoded-words are not legal inside an addr-spec, so encoding
	// is not an option the way it is for names). A CR/LF or other control
	// character in an address would let a hostile queue JSON inject arbitrary
	// headers. Reject the message instead — it lands in errors\ and surfaces
	// in the UI like any other invalid input.
	for _, list := range [][]Recipient{msg.Recipients.To, msg.Recipients.CC, msg.Recipients.BCC} {
		for _, r := range list {
			if err := validateHeaderAddress(r.Address); err != nil {
				return nil, err
			}
		}
	}

	// Headers
	if len(msg.Recipients.To) > 0 {
		buf.WriteString(fmt.Sprintf("To: %s\r\n", formatRecipients(msg.Recipients.To)))
	}
	if len(msg.Recipients.CC) > 0 {
		buf.WriteString(fmt.Sprintf("Cc: %s\r\n", formatRecipients(msg.Recipients.CC)))
	}
	if len(msg.Recipients.BCC) > 0 {
		buf.WriteString(fmt.Sprintf("Bcc: %s\r\n", formatRecipients(msg.Recipients.BCC)))
	}
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", mimeEncodeHeader(msg.Subject)))

	// ARRICKS-24: one body rendering for both branches; applies the
	// signature (and the plain→HTML promotion it requires) when present.
	bodyType, bodyContent := renderBody(msg)

	if hasAttachments {
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
		buf.WriteString("\r\n")

		// Body part
		buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		buf.WriteString(fmt.Sprintf("Content-Type: %s; charset=UTF-8\r\n", bodyType))
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		buf.WriteString("\r\n")
		buf.WriteString(base64Wrap([]byte(bodyContent)))
		buf.WriteString("\r\n")

		// Attachment parts
		var totalAttachmentBytes int64
		for _, att := range msg.Attachments {
			info, err := os.Stat(att.Path)
			if err != nil {
				return nil, fmt.Errorf("attachment not found: %s: %w", att.Path, err)
			}
			if info.Size() > MaxFileSize {
				return nil, fmt.Errorf("attachment too large (%d bytes): %s", info.Size(), att.Filename)
			}
			// ARRICKS-12 (R9): cumulative cap — see the constant's comment.
			totalAttachmentBytes += info.Size()
			if totalAttachmentBytes > MaxTotalAttachmentSize {
				return nil, fmt.Errorf("attachments too large in total (%d bytes at %s)", totalAttachmentBytes, att.Filename)
			}

			fileData, err := os.ReadFile(att.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", att.Path, err)
			}

			mimeType := mime.TypeByExtension(filepath.Ext(att.Filename))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}

			encodedName := mimeEncodeHeader(att.Filename)
			buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			buf.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", mimeType, encodedName))
			buf.WriteString("Content-Transfer-Encoding: base64\r\n")
			buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", encodedName))
			buf.WriteString("\r\n")
			buf.WriteString(base64Wrap(fileData))
			buf.WriteString("\r\n")
		}

		buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		// Simple message, no attachments
		buf.WriteString(fmt.Sprintf("Content-Type: %s; charset=UTF-8\r\n", bodyType))
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		buf.WriteString("\r\n")
		buf.WriteString(base64Wrap([]byte(bodyContent)))
	}

	return buf.Bytes(), nil
}

// renderBody returns the body part's content type and content, applying the
// ARRICKS-24 signature when present. Gmail stores signatures as HTML
// fragments and NEVER inserts them into API-created drafts (only into
// messages composed in its own UI — why Affixa grew its own signature
// feature), so a signed plain-text body is promoted to HTML: escaped,
// newlines to <br>, signature appended in Gmail's own gmail_signature div.
// Without a signature, behavior is byte-identical to the pre-ARRICKS-24
// output.
func renderBody(msg *MailMessage) (contentType, content string) {
	contentType = "text/plain"
	if msg.BodyFormat == "html" {
		contentType = "text/html"
	}
	if msg.Signature == "" {
		return contentType, msg.Body
	}
	body := msg.Body
	if msg.BodyFormat != "html" {
		body = strings.ReplaceAll(html.EscapeString(body), "\n", "<br>\r\n")
	}
	return "text/html", body + "<br><br><div class=\"gmail_signature\">" + msg.Signature + "</div>"
}

// SendAsEntry is the subset of the Gmail sendAs settings resource the app
// reads (ARRICKS-24). Signature is an HTML fragment; may be empty.
type SendAsEntry struct {
	SendAsEmail string `json:"sendAsEmail"`
	IsDefault   bool   `json:"isDefault"`
	Signature   string `json:"signature"`
}

// GetPrimarySignature fetches the account's default sendAs signature.
// Requires the gmail.settings.basic scope (ARRICKS-24); a token from a
// pre-3.8 sign-in returns 403 until the user re-consents — callers treat
// that as "no signature", never as a draft-blocking error.
func (gc *GmailClient) GetPrimarySignature() (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/settings/sendAs", gc.baseURL), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+gc.token)

	resp, err := gc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch sendAs settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return "", fmt.Errorf("token expired")
	}
	if resp.StatusCode == 403 {
		return "", fmt.Errorf("signature fetch forbidden — sign out and back in once to grant the settings-read permission added in 3.8")
	}
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gmail API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		SendAs []SendAsEntry `json:"sendAs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("failed to parse sendAs response: %w", err)
	}
	for _, e := range out.SendAs {
		if e.IsDefault {
			return e.Signature, nil
		}
	}
	if len(out.SendAs) > 0 {
		return out.SendAs[0].Signature, nil
	}
	return "", nil
}

// validateHeaderAddress rejects addresses that cannot be written into a
// header line safely: ASCII control characters (0x00-0x1F, 0x7F) enable
// CRLF header injection; anything else is passed through untouched, since
// scanners produce plain ASCII addresses and over-rejecting valid-but-odd
// addresses would discard the whole message. ARRICKS-12 (R4).
func validateHeaderAddress(addr string) error {
	for _, c := range addr {
		if c < 0x20 || c == 0x7F {
			return fmt.Errorf("recipient address contains control characters")
		}
	}
	return nil
}

// formatRecipients formats a list of recipients for an email header
func formatRecipients(recipients []Recipient) string {
	parts := make([]string, len(recipients))
	for i, r := range recipients {
		if r.Name != "" {
			parts[i] = fmt.Sprintf("%s <%s>", mimeEncodeHeader(r.Name), r.Address)
		} else {
			parts[i] = r.Address
		}
	}
	return strings.Join(parts, ", ")
}

// mimeEncodeHeader encodes a string for use in email headers (RFC 2047)
func mimeEncodeHeader(s string) string {
	// Check if encoding is needed
	needsEncoding := false
	for _, c := range s {
		if c > 126 || c < 32 {
			needsEncoding = true
			break
		}
	}
	if !needsEncoding {
		return s
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	return fmt.Sprintf("=?UTF-8?B?%s?=", encoded)
}

// base64Wrap encodes data as base64 with 76-char line wrapping
func base64Wrap(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var buf strings.Builder
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		buf.WriteString(encoded[i:end])
		buf.WriteString("\r\n")
	}
	return buf.String()
}

// Base64URLEncode encodes bytes to base64url without padding
func Base64URLEncode(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.TrimRight(s, "=")
	return s
}
