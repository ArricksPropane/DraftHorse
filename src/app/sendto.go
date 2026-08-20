package main

// ARRICKS-20 — "Send to → Mail recipient" plumbing self-heal.
//
// Explorer's SendTo menu entry is a per-user FILE
// (%APPDATA%\Microsoft\Windows\SendTo\Mail Recipient.MAPIMail) whose
// .MAPIMail extension resolves through HKCR to the shell's sendmail handler
// (CLSID {9E56BE60-C50F-11CF-9A2C-00A0C90A90CE}), which is what issues the
// Simple MAPI call this whole product intercepts. Outlook uninstalls and
// debloat tools are known to delete the file, orphan a per-user
// FileExts\.MAPIMail UserChoice, or strip the extension association —
// observed on the validation PC after removing Outlook: the menu entry
// vanished entirely. All three surfaces are per-user-repairable without
// elevation, so the ARRICKS-13 guard cadence (startup + hourly) heals them.

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const sendmailCLSIDRef = `CLSID\{9E56BE60-C50F-11CF-9A2C-00A0C90A90CE}`

// Test seams: redirected into sandboxes by sendto_test.go. Production never
// mutates them.
var (
	mapiMailExtHKCUPath   = `Software\Classes\.MAPIMail`
	mapiMailFileExtsPath  = `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.MAPIMail`
	sendToDirFromAppData  = `Microsoft\Windows\SendTo`
	sendToMailRecipient   = "Mail Recipient.MAPIMail"
)

// ensureSendToMailRecipient repairs the three per-user surfaces. Returns the
// list of repairs applied (empty = everything was already intact). Errors on
// individual surfaces don't stop the others; the first error is returned
// alongside whatever was repaired.
func ensureSendToMailRecipient() ([]string, error) {
	var repairs []string
	var firstErr error

	// 1. The SendTo menu file. Content is irrelevant (the CLSID handler
	// never reads it); existence puts "Mail Recipient" back in the menu.
	if appData := os.Getenv("APPDATA"); appData != "" {
		p := filepath.Join(appData, sendToDirFromAppData, sendToMailRecipient)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if mkErr := os.MkdirAll(filepath.Dir(p), 0o755); mkErr == nil {
				if f, cErr := os.Create(p); cErr == nil {
					f.Close()
					repairs = append(repairs, "sendto-file")
				} else if firstErr == nil {
					firstErr = cErr
				}
			} else if firstErr == nil {
				firstErr = mkErr
			}
		}
	}

	// 2. The .MAPIMail extension association. HKCR is the merged view; if
	// nothing resolves the extension to the sendmail CLSID, write the
	// per-user layer (HKCU\Software\Classes merges into HKCR, no elevation).
	if !mapiMailExtResolves() {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, mapiMailExtHKCUPath, registry.SET_VALUE)
		if err == nil {
			if sErr := k.SetStringValue("", sendmailCLSIDRef); sErr == nil {
				repairs = append(repairs, "mapimail-assoc")
			} else if firstErr == nil {
				firstErr = sErr
			}
			k.Close()
		} else if firstErr == nil {
			firstErr = err
		}
	}

	// 3. A stale per-user UserChoice for .MAPIMail (left by an uninstalled
	// client) overrides the machine association and breaks the entry even
	// when everything else is intact. Deleting the key restores default
	// resolution; best-effort — some Windows builds ACL-protect it.
	ucPath := mapiMailFileExtsPath + `\UserChoice`
	if k, err := registry.OpenKey(registry.CURRENT_USER, ucPath, registry.QUERY_VALUE); err == nil {
		k.Close()
		if dErr := registry.DeleteKey(registry.CURRENT_USER, ucPath); dErr == nil {
			repairs = append(repairs, "stale-userchoice")
		} else if firstErr == nil {
			firstErr = dErr
		}
	}

	return repairs, firstErr
}

// mapiMailExtResolves reports whether HKCR\.MAPIMail resolves to the
// sendmail handler (from either the machine or per-user classes layer).
func mapiMailExtResolves() bool {
	k, err := registry.OpenKey(registry.CLASSES_ROOT, `.MAPIMail`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("")
	return err == nil && v != ""
}
