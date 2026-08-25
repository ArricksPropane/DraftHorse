//go:build windows

package main

// ARRICKS-20 tests. Registry seams are redirected into a sandbox HKCU
// subtree; the SendTo file check uses a temp APPDATA. The HKCR resolution
// check is NOT sandboxed (it reads the real merged classes view), so the
// association-repair assertions tolerate an intact machine: on a healthy
// Windows box .MAPIMail already resolves and no assoc repair happens.

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func sandboxSendToSeams(t *testing.T) {
	t.Helper()
	origExt, origFileExts := mapiMailExtHKCUPath, mapiMailFileExtsPath
	mapiMailExtHKCUPath = `Software\DraftHorse-test\ARRICKS20\Classes\.MAPIMail`
	mapiMailFileExtsPath = `Software\DraftHorse-test\ARRICKS20\FileExts\.MAPIMail`
	t.Cleanup(func() {
		for _, p := range []string{
			mapiMailFileExtsPath + `\UserChoice`,
			mapiMailFileExtsPath,
			mapiMailExtHKCUPath,
			`Software\DraftHorse-test\ARRICKS20\Classes`,
			`Software\DraftHorse-test\ARRICKS20\FileExts`,
			`Software\DraftHorse-test\ARRICKS20`,
			`Software\DraftHorse-test`,
		} {
			_ = registry.DeleteKey(registry.CURRENT_USER, p)
		}
		mapiMailExtHKCUPath, mapiMailFileExtsPath = origExt, origFileExts
	})
}

func TestEnsureSendTo_CreatesMissingMenuFile(t *testing.T) {
	sandboxSendToSeams(t)
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)

	repairs, err := ensureSendToMailRecipient()
	if err != nil {
		t.Fatalf("ensureSendToMailRecipient: %v", err)
	}
	p := filepath.Join(appData, sendToDirFromAppData, sendToMailRecipient)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("SendTo file not created: %v", err)
	}
	if !containsRepair(repairs, "sendto-file") {
		t.Errorf("expected sendto-file repair, got %v", repairs)
	}

	// Second run: idempotent — the file repair must not repeat.
	repairs, err = ensureSendToMailRecipient()
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if containsRepair(repairs, "sendto-file") {
		t.Errorf("file repair repeated on intact state: %v", repairs)
	}
}

func TestEnsureSendTo_RemovesStaleUserChoice(t *testing.T) {
	sandboxSendToSeams(t)
	t.Setenv("APPDATA", t.TempDir())

	// Plant a stale UserChoice the way an uninstalled client leaves one.
	uc, _, err := registry.CreateKey(registry.CURRENT_USER, mapiMailFileExtsPath+`\UserChoice`, registry.SET_VALUE)
	if err != nil {
		t.Fatalf("plant UserChoice: %v", err)
	}
	if err := uc.SetStringValue("ProgId", "Outlook.File.MAPIMail.15"); err != nil {
		t.Fatalf("set ProgId: %v", err)
	}
	uc.Close()

	repairs, err := ensureSendToMailRecipient()
	if err != nil {
		t.Fatalf("ensureSendToMailRecipient: %v", err)
	}
	if !containsRepair(repairs, "stale-userchoice") {
		t.Errorf("expected stale-userchoice repair, got %v", repairs)
	}
	if k, err := registry.OpenKey(registry.CURRENT_USER, mapiMailFileExtsPath+`\UserChoice`, registry.QUERY_VALUE); err == nil {
		k.Close()
		t.Error("stale UserChoice key still present")
	}
}

func containsRepair(repairs []string, want string) bool {
	for _, r := range repairs {
		if r == want {
			return true
		}
	}
	return false
}
