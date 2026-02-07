package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	gmailAPIBase = "https://www.googleapis.com/gmail/v1/users/me"
	maxFileSize  = 25 * 1024 * 1024 // 25MB Gmail limit
)

// GmailClient handles Gmail API operations
type GmailClient struct {
	httpClient *http.Client
	token      string
}

// NewGmailClient creates a new Gmail API client with the given OAuth token
func NewGmailClient(token string) *GmailClient {
	return &GmailClient{
		httpClient: &http.Client{},
		token:      token,
	}
}

// DraftResponse represents a Gmail API draft response
type DraftResponse struct {
	ID      string `json:"id"`
	Message struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
		Raw      string `json:"raw"`
	} `json:"message"`
}

// GetDraft retrieves a draft by ID, including the raw RFC 2822 message
func (gc *GmailClient) GetDraft(draftID string) (*DraftResponse, error) {
	url := fmt.Sprintf("%s/drafts/%s?format=raw", gmailAPIBase, draftID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+gc.token)

	resp, err := gc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch draft: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("token expired")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gmail API error (%d): %s", resp.StatusCode, string(body))
	}

	var draft DraftResponse
	if err := json.NewDecoder(resp.Body).Decode(&draft); err != nil {
		return nil, fmt.Errorf("failed to parse draft response: %w", err)
	}

	return &draft, nil
}

// UpdateDraftWithAttachments rebuilds a draft's message to include attachments
func (gc *GmailClient) UpdateDraftWithAttachments(draftID string, rawMessage string, attachments []AttachmentUpload, progressFn func(current, total int, filename string)) error {
	// Decode the raw message from base64url
	originalMsg, err := base64URLDecode(rawMessage)
	if err != nil {
		return fmt.Errorf("failed to decode raw message: %w", err)
	}

	// Build new multipart MIME message with attachments
	mimeMsg, err := buildMIMEWithAttachments(string(originalMsg), attachments, progressFn)
	if err != nil {
		return fmt.Errorf("failed to build MIME message: %w", err)
	}

	// Base64url encode the new message
	encodedMsg := base64URLEncode(mimeMsg)

	// Update the draft via Gmail API
	updateBody := map[string]interface{}{
		"message": map[string]interface{}{
			"raw": encodedMsg,
		},
	}

	bodyJSON, err := json.Marshal(updateBody)
	if err != nil {
		return fmt.Errorf("failed to marshal update body: %w", err)
	}

	url := fmt.Sprintf("%s/drafts/%s", gmailAPIBase, draftID)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to create update request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+gc.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := gc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update draft: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("token expired")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Gmail API error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// buildMIMEWithAttachments constructs a multipart/mixed MIME message
func buildMIMEWithAttachments(originalMessage string, attachments []AttachmentUpload, progressFn func(current, total int, filename string)) ([]byte, error) {
	var buf bytes.Buffer
	boundary := "go_mapi_boundary_" + fmt.Sprintf("%d", os.Getpid())

	// Parse original headers and body
	headers, body := splitHeadersBody(originalMessage)

	// Extract Content-Type from original headers (default to text/plain)
	originalContentType := "text/plain; charset=UTF-8"
	var otherHeaders []string
	for _, h := range headers {
		lower := strings.ToLower(h)
		if strings.HasPrefix(lower, "content-type:") {
			originalContentType = strings.TrimSpace(h[len("Content-Type:"):])
		} else {
			otherHeaders = append(otherHeaders, h)
		}
	}

	// Write non-content-type headers
	for _, h := range otherHeaders {
		buf.WriteString(h)
		buf.WriteString("\r\n")
	}

	// Set multipart/mixed content type
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	buf.WriteString("\r\n")

	// Part 1: Original message body
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString(fmt.Sprintf("Content-Type: %s\r\n", originalContentType))
	buf.WriteString("\r\n")
	buf.WriteString(body)
	buf.WriteString("\r\n")

	// Attachment parts
	total := len(attachments)
	for i, att := range attachments {
		if progressFn != nil {
			progressFn(i+1, total, att.Filename)
		}

		// Validate file
		info, err := os.Stat(att.Path)
		if err != nil {
			return nil, fmt.Errorf("attachment not found: %s: %w", att.Path, err)
		}
		if info.Size() > maxFileSize {
			return nil, fmt.Errorf("attachment too large (%d bytes, max %d): %s", info.Size(), maxFileSize, att.Filename)
		}

		// Read file
		fileData, err := os.ReadFile(att.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read attachment %s: %w", att.Path, err)
		}

		// Determine MIME type
		mimeType := mime.TypeByExtension(filepath.Ext(att.Filename))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		// Write attachment part
		buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		buf.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", mimeType, att.Filename))
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", att.Filename))
		buf.WriteString("\r\n")

		// Base64 encode file content with line wrapping (76 chars)
		encoded := base64.StdEncoding.EncodeToString(fileData)
		for j := 0; j < len(encoded); j += 76 {
			end := j + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			buf.WriteString(encoded[j:end])
			buf.WriteString("\r\n")
		}
	}

	// Close boundary
	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return buf.Bytes(), nil
}

// splitHeadersBody splits an RFC 2822 message into headers and body
func splitHeadersBody(message string) (headers []string, body string) {
	// Normalize line endings
	message = strings.ReplaceAll(message, "\r\n", "\n")

	// Find the blank line that separates headers from body
	idx := strings.Index(message, "\n\n")
	if idx == -1 {
		// No body
		return strings.Split(message, "\n"), ""
	}

	headerPart := message[:idx]
	body = message[idx+2:]

	// Parse headers (handle folded headers)
	var currentHeader string
	for _, line := range strings.Split(headerPart, "\n") {
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			// Continuation of previous header
			currentHeader += "\r\n" + line
		} else {
			if currentHeader != "" {
				headers = append(headers, currentHeader)
			}
			currentHeader = line
		}
	}
	if currentHeader != "" {
		headers = append(headers, currentHeader)
	}

	return headers, body
}

// base64URLDecode decodes a base64url-encoded string (no padding)
func base64URLDecode(s string) ([]byte, error) {
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	return base64.StdEncoding.DecodeString(s)
}

// base64URLEncode encodes bytes to base64url without padding
func base64URLEncode(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.TrimRight(s, "=")
	return s
}
