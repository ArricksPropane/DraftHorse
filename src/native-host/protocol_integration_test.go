package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestProtocolFixtures validates that the Go implementation can correctly
// read and write messages matching the shared protocol fixtures.
// This ensures compatibility between Go and TypeScript implementations.

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	// Go up from src/native-host to repo root, then into tests/protocol-fixtures
	path := filepath.Join("..", "..", "tests", "protocol-fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return data
}

func TestFixture_ReadyMessage(t *testing.T) {
	fixture := loadFixture(t, "ready-message.json")

	var msg OutgoingMessage
	if err := json.Unmarshal(fixture, &msg); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	if msg.Type != MsgTypeReady {
		t.Errorf("type = %v, want %v", msg.Type, MsgTypeReady)
	}
	if msg.Version != "1.0.0" {
		t.Errorf("version = %v, want %v", msg.Version, "1.0.0")
	}
}

func TestFixture_EmailMessage(t *testing.T) {
	fixture := loadFixture(t, "email-message.json")

	var msg OutgoingMessage
	if err := json.Unmarshal(fixture, &msg); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	if msg.Type != MsgTypeEmail {
		t.Errorf("type = %v, want %v", msg.Type, MsgTypeEmail)
	}
	if msg.ID != "abc123def456" {
		t.Errorf("id = %v, want %v", msg.ID, "abc123def456")
	}
	if msg.Data == nil {
		t.Fatal("data is nil")
	}
	if msg.Data.Subject != "Test Email Subject" {
		t.Errorf("subject = %v, want %v", msg.Data.Subject, "Test Email Subject")
	}
	if msg.Data.BodyFormat != "plain" {
		t.Errorf("bodyFormat = %v, want %v", msg.Data.BodyFormat, "plain")
	}
	if len(msg.Data.Recipients.To) != 1 {
		t.Errorf("to recipients count = %v, want 1", len(msg.Data.Recipients.To))
	}
	if len(msg.Data.Recipients.CC) != 1 {
		t.Errorf("cc recipients count = %v, want 1", len(msg.Data.Recipients.CC))
	}
	if len(msg.Data.Attachments) != 1 {
		t.Errorf("attachments count = %v, want 1", len(msg.Data.Attachments))
	}
}

func TestFixture_RemovedMessage(t *testing.T) {
	fixture := loadFixture(t, "removed-message.json")

	var msg OutgoingMessage
	if err := json.Unmarshal(fixture, &msg); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	if msg.Type != MsgTypeRemoved {
		t.Errorf("type = %v, want %v", msg.Type, MsgTypeRemoved)
	}
	if msg.ID != "abc123def456" {
		t.Errorf("id = %v, want %v", msg.ID, "abc123def456")
	}
}

func TestFixture_ErrorMessage(t *testing.T) {
	fixture := loadFixture(t, "error-message.json")

	var msg OutgoingMessage
	if err := json.Unmarshal(fixture, &msg); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	if msg.Type != MsgTypeError {
		t.Errorf("type = %v, want %v", msg.Type, MsgTypeError)
	}
	if msg.Error == "" {
		t.Error("error message is empty")
	}
}

func TestFixture_ListCommand(t *testing.T) {
	fixture := loadFixture(t, "list-command.json")

	var msg IncomingMessage
	if err := json.Unmarshal(fixture, &msg); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	if msg.Type != MsgTypeList {
		t.Errorf("type = %v, want %v", msg.Type, MsgTypeList)
	}
}

func TestFixture_ProcessCommand(t *testing.T) {
	fixture := loadFixture(t, "process-command.json")

	var msg IncomingMessage
	if err := json.Unmarshal(fixture, &msg); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	if msg.Type != MsgTypeProcess {
		t.Errorf("type = %v, want %v", msg.Type, MsgTypeProcess)
	}
	if msg.ID != "abc123def456" {
		t.Errorf("id = %v, want %v", msg.ID, "abc123def456")
	}
}

func TestFixture_DeleteCommand(t *testing.T) {
	fixture := loadFixture(t, "delete-command.json")

	var msg IncomingMessage
	if err := json.Unmarshal(fixture, &msg); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	if msg.Type != MsgTypeDelete {
		t.Errorf("type = %v, want %v", msg.Type, MsgTypeDelete)
	}
	if msg.ID != "abc123def456" {
		t.Errorf("id = %v, want %v", msg.ID, "abc123def456")
	}
}

func TestFixture_ShutdownCommand(t *testing.T) {
	fixture := loadFixture(t, "shutdown-command.json")

	var msg IncomingMessage
	if err := json.Unmarshal(fixture, &msg); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	if msg.Type != MsgTypeShutdown {
		t.Errorf("type = %v, want %v", msg.Type, MsgTypeShutdown)
	}
}

// TestNativeMessagingFormat verifies the Native Messaging wire format
// (4-byte little-endian length prefix + JSON body)
func TestNativeMessagingFormat_Write(t *testing.T) {
	buf := new(bytes.Buffer)
	nm := &NativeMessaging{
		reader: bytes.NewReader([]byte{}),
		writer: buf,
	}

	err := nm.SendReady("1.0.0")
	if err != nil {
		t.Fatalf("SendReady() error = %v", err)
	}

	data := buf.Bytes()

	// First 4 bytes should be length prefix (little-endian)
	if len(data) < 4 {
		t.Fatal("output too short for length prefix")
	}

	var length uint32
	binary.Read(bytes.NewReader(data[:4]), binary.LittleEndian, &length)

	// Verify length matches actual body
	if int(length) != len(data)-4 {
		t.Errorf("length prefix = %d, body length = %d", length, len(data)-4)
	}

	// Verify body is valid JSON
	var msg map[string]interface{}
	if err := json.Unmarshal(data[4:], &msg); err != nil {
		t.Errorf("body is not valid JSON: %v", err)
	}
}

func TestNativeMessagingFormat_Read(t *testing.T) {
	// Create a valid native messaging format message
	body := []byte(`{"type":"list"}`)
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(len(body)))
	buf.Write(body)

	nm := &NativeMessaging{
		reader: buf,
		writer: bytes.NewBuffer(nil),
	}

	msg, err := nm.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if msg.Type != MsgTypeList {
		t.Errorf("type = %v, want %v", msg.Type, MsgTypeList)
	}
}
