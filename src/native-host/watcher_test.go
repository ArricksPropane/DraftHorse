package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateMailMessage_Valid(t *testing.T) {
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		Subject:    "Test",
		Body:       "Test body",
		BodyFormat: "plain",
		Recipients: Recipients{
			To: []Recipient{{Name: "John", Address: "john@example.com"}},
		},
	}

	if err := validateMailMessage(mail); err != nil {
		t.Errorf("validateMailMessage() error = %v, want nil", err)
	}
}

func TestValidateMailMessage_ValidHTML(t *testing.T) {
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		Subject:    "Test",
		Body:       "<p>Test body</p>",
		BodyFormat: "html",
		Recipients: Recipients{
			To: []Recipient{{Address: "test@example.com"}},
		},
	}

	if err := validateMailMessage(mail); err != nil {
		t.Errorf("validateMailMessage() error = %v, want nil", err)
	}
}

func TestValidateMailMessage_MissingVersion(t *testing.T) {
	mail := &MailMessage{
		Version:    0, // Missing/zero
		Timestamp:  "2024-01-01T00:00:00Z",
		BodyFormat: "plain",
	}

	err := validateMailMessage(mail)
	if err == nil {
		t.Error("validateMailMessage() expected error for missing version")
	}
}

func TestValidateMailMessage_MissingTimestamp(t *testing.T) {
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "", // Missing
		BodyFormat: "plain",
	}

	err := validateMailMessage(mail)
	if err == nil {
		t.Error("validateMailMessage() expected error for missing timestamp")
	}
}

func TestValidateMailMessage_InvalidBodyFormat(t *testing.T) {
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		BodyFormat: "invalid", // Not plain or html
	}

	err := validateMailMessage(mail)
	if err == nil {
		t.Error("validateMailMessage() expected error for invalid bodyFormat")
	}
}

func TestValidateMailMessage_EmptyBodyFormat(t *testing.T) {
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		BodyFormat: "",
	}

	err := validateMailMessage(mail)
	if err == nil {
		t.Error("validateMailMessage() expected error for empty bodyFormat")
	}
}

func TestValidateMailMessage_ToRecipientMissingAddress(t *testing.T) {
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		BodyFormat: "plain",
		Recipients: Recipients{
			To: []Recipient{{Name: "John", Address: ""}}, // Missing address
		},
	}

	err := validateMailMessage(mail)
	if err == nil {
		t.Error("validateMailMessage() expected error for recipient missing address")
	}
}

func TestValidateMailMessage_CCRecipientMissingAddress(t *testing.T) {
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		BodyFormat: "plain",
		Recipients: Recipients{
			To: []Recipient{{Address: "to@example.com"}},
			CC: []Recipient{{Name: "CC Person", Address: ""}},
		},
	}

	err := validateMailMessage(mail)
	if err == nil {
		t.Error("validateMailMessage() expected error for CC recipient missing address")
	}
}

func TestValidateMailMessage_BCCRecipientMissingAddress(t *testing.T) {
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		BodyFormat: "plain",
		Recipients: Recipients{
			To:  []Recipient{{Address: "to@example.com"}},
			BCC: []Recipient{{Name: "BCC Person", Address: ""}},
		},
	}

	err := validateMailMessage(mail)
	if err == nil {
		t.Error("validateMailMessage() expected error for BCC recipient missing address")
	}
}

func TestValidateMailMessage_MultipleRecipients(t *testing.T) {
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		Subject:    "Test",
		Body:       "Body",
		BodyFormat: "plain",
		Recipients: Recipients{
			To:  []Recipient{{Address: "to1@example.com"}, {Address: "to2@example.com"}},
			CC:  []Recipient{{Address: "cc@example.com"}},
			BCC: []Recipient{{Address: "bcc@example.com"}},
		},
	}

	if err := validateMailMessage(mail); err != nil {
		t.Errorf("validateMailMessage() error = %v, want nil", err)
	}
}

func TestValidateMailMessage_NoRecipients(t *testing.T) {
	// Email with no recipients is valid (recipients are optional per the code)
	mail := &MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		BodyFormat: "plain",
		Recipients: Recipients{},
	}

	if err := validateMailMessage(mail); err != nil {
		t.Errorf("validateMailMessage() error = %v, want nil", err)
	}
}

func TestGenerateID_DifferentContent(t *testing.T) {
	data1 := []byte(`{"subject": "test1"}`)
	data2 := []byte(`{"subject": "test2"}`)

	id1 := generateID(data1, "file1.json")
	id2 := generateID(data2, "file2.json")

	if id1 == id2 {
		t.Error("generateID() should produce different IDs for different content")
	}
}

func TestGenerateID_SameContentDifferentFilename(t *testing.T) {
	data := []byte(`{"subject": "test"}`)

	id1 := generateID(data, "file1.json")
	id2 := generateID(data, "file2.json")

	// Note: generateID includes time.Now() so IDs will always differ
	// This test just verifies it doesn't crash
	if id1 == "" || id2 == "" {
		t.Error("generateID() should not return empty string")
	}
}

func TestGenerateID_Format(t *testing.T) {
	data := []byte(`{"test": true}`)
	id := generateID(data, "test.json")

	// SHA256 produces 64 hex characters
	if len(id) != 64 {
		t.Errorf("generateID() length = %d, want 64", len(id))
	}

	// Should be valid hex
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateID() contains non-hex character: %c", c)
		}
	}
}

// Integration-style tests for EmailWatcher

func TestEmailWatcher_Creation(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")

	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: io.Discard,
	}

	ew, err := NewEmailWatcher(watchDir, nm)
	if err != nil {
		t.Fatalf("NewEmailWatcher() error = %v", err)
	}
	defer ew.Stop()

	// Check directories were created
	if _, err := os.Stat(watchDir); os.IsNotExist(err) {
		t.Error("watch directory not created")
	}
	if _, err := os.Stat(filepath.Join(watchDir, "processed")); os.IsNotExist(err) {
		t.Error("processed directory not created")
	}
	if _, err := os.Stat(filepath.Join(watchDir, "errors")); os.IsNotExist(err) {
		t.Error("errors directory not created")
	}
}

func TestEmailWatcher_GetEmails_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")

	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: io.Discard,
	}

	ew, err := NewEmailWatcher(watchDir, nm)
	if err != nil {
		t.Fatalf("NewEmailWatcher() error = %v", err)
	}
	defer ew.Stop()

	emails := ew.GetEmails()
	if len(emails) != 0 {
		t.Errorf("GetEmails() length = %d, want 0", len(emails))
	}
}

func TestEmailWatcher_ProcessExistingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")
	os.MkdirAll(watchDir, 0755)

	// Create a valid email file before starting watcher
	email := MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		Subject:    "Existing Email",
		Body:       "This was here before",
		BodyFormat: "plain",
		Recipients: Recipients{
			To: []Recipient{{Address: "test@example.com"}},
		},
	}
	data, _ := json.Marshal(email)
	os.WriteFile(filepath.Join(watchDir, "existing.json"), data, 0644)

	// Capture output
	output := new(bytes.Buffer)
	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: output,
	}

	ew, err := NewEmailWatcher(watchDir, nm)
	if err != nil {
		t.Fatalf("NewEmailWatcher() error = %v", err)
	}

	// Start will process existing files
	if err := ew.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ew.Stop()

	// Check email was loaded
	emails := ew.GetEmails()
	if len(emails) != 1 {
		t.Errorf("GetEmails() length = %d, want 1", len(emails))
	}

	// Verify subject
	for _, mail := range emails {
		if mail.Subject != "Existing Email" {
			t.Errorf("Subject = %v, want %v", mail.Subject, "Existing Email")
		}
	}
}

func TestEmailWatcher_InvalidFileMovedToErrors(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")
	os.MkdirAll(watchDir, 0755)

	// Create an invalid email file (missing required fields)
	os.WriteFile(filepath.Join(watchDir, "invalid.json"), []byte(`{"subject": "No version"}`), 0644)

	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: io.Discard,
	}

	ew, err := NewEmailWatcher(watchDir, nm)
	if err != nil {
		t.Fatalf("NewEmailWatcher() error = %v", err)
	}

	if err := ew.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ew.Stop()

	// File should be moved to errors
	errorsDir := filepath.Join(watchDir, "errors")
	if _, err := os.Stat(filepath.Join(errorsDir, "invalid.json")); os.IsNotExist(err) {
		t.Error("invalid file was not moved to errors directory")
	}

	// Original should be gone
	if _, err := os.Stat(filepath.Join(watchDir, "invalid.json")); !os.IsNotExist(err) {
		t.Error("invalid file still exists in watch directory")
	}
}

func TestEmailWatcher_MarkProcessed(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")
	os.MkdirAll(watchDir, 0755)

	// Create a valid email file
	email := MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		Subject:    "To Process",
		BodyFormat: "plain",
	}
	data, _ := json.Marshal(email)
	os.WriteFile(filepath.Join(watchDir, "process-me.json"), data, 0644)

	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: io.Discard,
	}

	ew, err := NewEmailWatcher(watchDir, nm)
	if err != nil {
		t.Fatalf("NewEmailWatcher() error = %v", err)
	}

	if err := ew.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ew.Stop()

	// Get the email ID
	emails := ew.GetEmails()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}

	var emailID string
	for id := range emails {
		emailID = id
	}

	// Mark as processed
	if err := ew.MarkProcessed(emailID); err != nil {
		t.Fatalf("MarkProcessed() error = %v", err)
	}

	// Should be moved to processed directory
	processedDir := filepath.Join(watchDir, "processed")
	if _, err := os.Stat(filepath.Join(processedDir, "process-me.json")); os.IsNotExist(err) {
		t.Error("file was not moved to processed directory")
	}

	// Email should be removed from map
	emails = ew.GetEmails()
	if len(emails) != 0 {
		t.Errorf("GetEmails() length = %d, want 0 after processing", len(emails))
	}
}

func TestEmailWatcher_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")
	os.MkdirAll(watchDir, 0755)

	// Create a valid email file
	email := MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		Subject:    "To Delete",
		BodyFormat: "plain",
	}
	data, _ := json.Marshal(email)
	os.WriteFile(filepath.Join(watchDir, "delete-me.json"), data, 0644)

	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: io.Discard,
	}

	ew, err := NewEmailWatcher(watchDir, nm)
	if err != nil {
		t.Fatalf("NewEmailWatcher() error = %v", err)
	}

	if err := ew.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ew.Stop()

	// Get the email ID
	emails := ew.GetEmails()
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}

	var emailID string
	for id := range emails {
		emailID = id
	}

	// Delete
	if err := ew.Delete(emailID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// File should be gone
	if _, err := os.Stat(filepath.Join(watchDir, "delete-me.json")); !os.IsNotExist(err) {
		t.Error("file was not deleted")
	}

	// Email should be removed from map
	emails = ew.GetEmails()
	if len(emails) != 0 {
		t.Errorf("GetEmails() length = %d, want 0 after delete", len(emails))
	}
}

func TestEmailWatcher_MarkProcessed_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")

	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: io.Discard,
	}

	ew, err := NewEmailWatcher(watchDir, nm)
	if err != nil {
		t.Fatalf("NewEmailWatcher() error = %v", err)
	}
	defer ew.Stop()

	err = ew.MarkProcessed("nonexistent-id")
	if err == nil {
		t.Error("MarkProcessed() expected error for nonexistent ID")
	}
}

func TestEmailWatcher_Delete_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")

	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: io.Discard,
	}

	ew, err := NewEmailWatcher(watchDir, nm)
	if err != nil {
		t.Fatalf("NewEmailWatcher() error = %v", err)
	}
	defer ew.Stop()

	err = ew.Delete("nonexistent-id")
	if err == nil {
		t.Error("Delete() expected error for nonexistent ID")
	}
}

func TestEmailWatcher_IgnoresNonJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := filepath.Join(tmpDir, "watch")
	os.MkdirAll(watchDir, 0755)

	// Create non-JSON files
	os.WriteFile(filepath.Join(watchDir, "readme.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(watchDir, "data.xml"), []byte("<data/>"), 0644)

	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: io.Discard,
	}

	ew, err := NewEmailWatcher(watchDir, nm)
	if err != nil {
		t.Fatalf("NewEmailWatcher() error = %v", err)
	}

	if err := ew.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer ew.Stop()

	// Should have no emails
	emails := ew.GetEmails()
	if len(emails) != 0 {
		t.Errorf("GetEmails() length = %d, want 0", len(emails))
	}

	// Non-JSON files should still exist (not moved to errors)
	if _, err := os.Stat(filepath.Join(watchDir, "readme.txt")); os.IsNotExist(err) {
		t.Error("non-JSON file was incorrectly processed")
	}
}
