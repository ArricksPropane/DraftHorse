package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Version is set at build time via -ldflags "-X main.Version=..."
// Falls back to "0.0.0-dev" for development builds
var Version = "0.0.0-dev"

const (
	HostName = "com.gomapi.host"
)

var (
	logFile *os.File
)

func main() {
	// Initialize logging
	initLogging()
	defer closeLogging()

	logInfo("go-mapi native host starting (version %s)", Version)

	// Get watch directory
	watchDir := getWatchDir()
	logInfo("watching directory: %s", watchDir)

	// Create native messaging handler
	messaging := NewNativeMessaging()

	// Create email watcher
	watcher, err := NewEmailWatcher(watchDir, messaging)
	if err != nil {
		logError("failed to create watcher: %v", err)
		messaging.SendError(fmt.Sprintf("failed to create watcher: %v", err))
		os.Exit(1)
	}

	// Start watching
	if err := watcher.Start(); err != nil {
		logError("failed to start watcher: %v", err)
		messaging.SendError(fmt.Sprintf("failed to start watcher: %v", err))
		os.Exit(1)
	}
	defer watcher.Stop()

	// Send ready message
	if err := messaging.SendReady(Version); err != nil {
		logError("failed to send ready message: %v", err)
	}

	logInfo("native host ready, waiting for messages")

	// Main message loop
	for {
		msg, err := messaging.Read()
		if err != nil {
			if err == io.EOF {
				logInfo("extension disconnected")
				break
			}
			logError("failed to read message: %v", err)
			continue
		}

		logInfo("received message: type=%s id=%s", msg.Type, msg.ID)

		switch msg.Type {
		case MsgTypeList:
			// Send all current emails
			for id, mail := range watcher.GetEmails() {
				if err := messaging.SendEmail(id, mail); err != nil {
					logError("failed to send email: %v", err)
				}
			}

		case MsgTypeProcess:
			if err := watcher.MarkProcessed(msg.ID); err != nil {
				logError("failed to mark processed: %v", err)
				messaging.SendError(fmt.Sprintf("failed to mark processed: %v", err))
			}

		case MsgTypeDelete:
			if err := watcher.Delete(msg.ID); err != nil {
				logError("failed to delete: %v", err)
				messaging.SendError(fmt.Sprintf("failed to delete: %v", err))
			} else {
				// Confirm deletion
				messaging.SendRemoved(msg.ID)
			}

		case MsgTypeUploadAttachments:
			logInfo("upload-attachments request: draftId=%s, %d files", msg.DraftID, len(msg.Attachments))
			// Run upload in goroutine to not block the message loop
			go handleUploadAttachments(messaging, msg)

		case MsgTypeShutdown:
			logInfo("shutdown requested")
			return

		default:
			logError("unknown message type: %s", msg.Type)
		}
	}

	logInfo("native host exiting")
}

func handleUploadAttachments(messaging *NativeMessaging, msg *IncomingMessage) {
	if msg.Token == "" {
		logError("upload-attachments: missing OAuth token")
		messaging.SendUploadError(msg.DraftID, "Missing OAuth token")
		return
	}
	if msg.DraftID == "" {
		logError("upload-attachments: missing draftId")
		messaging.SendUploadError(msg.DraftID, "Missing draft ID")
		return
	}
	if len(msg.Attachments) == 0 {
		logInfo("upload-attachments: no attachments, sending complete")
		messaging.SendUploadComplete(msg.DraftID)
		return
	}

	client := NewGmailClient(msg.Token)

	// Get the current draft to retrieve the raw message
	logInfo("fetching draft %s", msg.DraftID)
	draft, err := client.GetDraft(msg.DraftID)
	if err != nil {
		logError("failed to get draft: %v", err)
		messaging.SendUploadError(msg.DraftID, fmt.Sprintf("Failed to get draft: %v", err))
		return
	}

	// Progress callback
	progressFn := func(current, total int, filename string) {
		logInfo("uploading attachment %d/%d: %s", current, total, filename)
		messaging.SendUploadProgress(msg.DraftID, current, total, filename)
	}

	// Update draft with attachments
	logInfo("updating draft %s with %d attachments", msg.DraftID, len(msg.Attachments))
	if err := client.UpdateDraftWithAttachments(msg.DraftID, draft.Message.Raw, msg.Attachments, progressFn); err != nil {
		logError("failed to upload attachments: %v", err)
		messaging.SendUploadError(msg.DraftID, fmt.Sprintf("Upload failed: %v", err))
		return
	}

	logInfo("upload complete for draft %s", msg.DraftID)
	messaging.SendUploadComplete(msg.DraftID)
}

func getWatchDir() string {
	// Use TEMP environment variable
	tempDir := os.Getenv("TEMP")
	if tempDir == "" {
		tempDir = os.Getenv("TMP")
	}
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	return filepath.Join(tempDir, "go-mapi")
}

func initLogging() {
	// Log to file in watch directory for debugging
	logDir := getWatchDir()
	os.MkdirAll(logDir, 0755)

	logPath := filepath.Join(logDir, "native-host.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	logFile = f
}

func closeLogging() {
	if logFile != nil {
		logFile.Close()
	}
}

func logInfo(format string, args ...interface{}) {
	if logFile != nil {
		ts := time.Now().Format(time.RFC3339)
		fmt.Fprintf(logFile, "[%s] [INFO] "+format+"\n", append([]interface{}{ts}, args...)...)
		logFile.Sync()
	}
}

func logError(format string, args ...interface{}) {
	if logFile != nil {
		ts := time.Now().Format(time.RFC3339)
		fmt.Fprintf(logFile, "[%s] [ERROR] "+format+"\n", append([]interface{}{ts}, args...)...)
		logFile.Sync()
	}
}
