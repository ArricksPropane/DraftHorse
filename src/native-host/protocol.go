package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Message types for Native Messaging protocol
const (
	MsgTypeEmail    = "email"    // Host → Extension: new email detected
	MsgTypeRemoved  = "removed"  // Host → Extension: email file removed
	MsgTypeReady    = "ready"    // Host → Extension: host is ready
	MsgTypeError    = "error"    // Host → Extension: error occurred
	MsgTypeProcess  = "process"  // Extension → Host: mark as processed
	MsgTypeDelete   = "delete"   // Extension → Host: delete email
	MsgTypeList     = "list"     // Extension → Host: request current emails
	MsgTypeShutdown = "shutdown" // Extension → Host: graceful shutdown

	// Attachment upload flow
	MsgTypeUploadAttachments = "upload-attachments" // Extension → Host: upload files to draft
	MsgTypeUploadComplete    = "upload-complete"    // Host → Extension: upload succeeded
	MsgTypeUploadError       = "upload-error"       // Host → Extension: upload failed
	MsgTypeUploadProgress    = "upload-progress"    // Host → Extension: upload progress
)

// OutgoingMessage is sent from host to extension
type OutgoingMessage struct {
	Type    string       `json:"type"`
	ID      string       `json:"id,omitempty"`
	Data    *MailMessage `json:"data,omitempty"`
	Error   string       `json:"error,omitempty"`
	Version string       `json:"version,omitempty"`

	// Upload fields
	DraftID  string `json:"draftId,omitempty"`
	Current  int    `json:"current,omitempty"`
	Total    int    `json:"total,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// IncomingMessage is received from extension
type IncomingMessage struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`

	// Upload-attachments fields
	DraftID     string             `json:"draftId,omitempty"`
	MessageID   string             `json:"messageId,omitempty"`
	Token       string             `json:"token,omitempty"`
	Attachments []AttachmentUpload `json:"attachments,omitempty"`
}

// AttachmentUpload describes a file to upload to a Gmail draft
type AttachmentUpload struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
}

// MailMessage represents an intercepted email
type MailMessage struct {
	Version    int         `json:"version"`
	Timestamp  string      `json:"timestamp"`
	Subject    string      `json:"subject"`
	Body       string      `json:"body"`
	BodyFormat string      `json:"bodyFormat"`
	Recipients Recipients  `json:"recipients"`
	Attachments []Attachment `json:"attachments"`
	OriginApp  string      `json:"originApp"`
}

// Recipients contains email recipients by type
type Recipients struct {
	To  []Recipient `json:"to"`
	CC  []Recipient `json:"cc"`
	BCC []Recipient `json:"bcc"`
}

// Recipient represents a single email recipient
type Recipient struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Attachment represents an email attachment
type Attachment struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
}

// NativeMessaging handles Chrome Native Messaging protocol
// Protocol: 4-byte length prefix (little-endian) + JSON message
type NativeMessaging struct {
	reader io.Reader
	writer io.Writer
}

// NewNativeMessaging creates a new Native Messaging handler
func NewNativeMessaging() *NativeMessaging {
	return &NativeMessaging{
		reader: os.Stdin,
		writer: os.Stdout,
	}
}

// Read reads a message from the extension
func (nm *NativeMessaging) Read() (*IncomingMessage, error) {
	// Read 4-byte length prefix (little-endian)
	var length uint32
	if err := binary.Read(nm.reader, binary.LittleEndian, &length); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	// Sanity check: max message size 1MB
	if length > 1024*1024 {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	// Read message body
	body := make([]byte, length)
	if _, err := io.ReadFull(nm.reader, body); err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	// Parse JSON
	var msg IncomingMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	return &msg, nil
}

// Write sends a message to the extension
func (nm *NativeMessaging) Write(msg *OutgoingMessage) error {
	// Serialize to JSON
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Write 4-byte length prefix (little-endian)
	length := uint32(len(body))
	if err := binary.Write(nm.writer, binary.LittleEndian, length); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	// Write message body
	if _, err := nm.writer.Write(body); err != nil {
		return fmt.Errorf("failed to write message body: %w", err)
	}

	return nil
}

// SendEmail sends an email message to the extension
func (nm *NativeMessaging) SendEmail(id string, mail *MailMessage) error {
	return nm.Write(&OutgoingMessage{
		Type: MsgTypeEmail,
		ID:   id,
		Data: mail,
	})
}

// SendRemoved notifies extension that an email was removed
func (nm *NativeMessaging) SendRemoved(id string) error {
	return nm.Write(&OutgoingMessage{
		Type: MsgTypeRemoved,
		ID:   id,
	})
}

// SendReady notifies extension that host is ready
func (nm *NativeMessaging) SendReady(version string) error {
	return nm.Write(&OutgoingMessage{
		Type:    MsgTypeReady,
		Version: version,
	})
}

// SendError sends an error message to the extension
func (nm *NativeMessaging) SendError(errMsg string) error {
	return nm.Write(&OutgoingMessage{
		Type:  MsgTypeError,
		Error: errMsg,
	})
}

// SendUploadComplete notifies extension that attachment upload succeeded
func (nm *NativeMessaging) SendUploadComplete(draftID string) error {
	return nm.Write(&OutgoingMessage{
		Type:    MsgTypeUploadComplete,
		DraftID: draftID,
	})
}

// SendUploadError notifies extension that attachment upload failed
func (nm *NativeMessaging) SendUploadError(draftID string, errMsg string) error {
	return nm.Write(&OutgoingMessage{
		Type:    MsgTypeUploadError,
		DraftID: draftID,
		Error:   errMsg,
	})
}

// SendUploadProgress sends upload progress to extension
func (nm *NativeMessaging) SendUploadProgress(draftID string, current, total int, filename string) error {
	return nm.Write(&OutgoingMessage{
		Type:     MsgTypeUploadProgress,
		DraftID:  draftID,
		Current:  current,
		Total:    total,
		Filename: filename,
	})
}
