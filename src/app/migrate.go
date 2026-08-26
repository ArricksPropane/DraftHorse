package main

// V4 per-user migration — go-mapi → DraftHorse (docs/V4-PLAN.md Phase 1).
//
// 4.0 renamed every installed identifier. The installer scrubs MACHINE scope
// (HKLM keys, Program Files, firewall) because it runs elevated; everything
// per-user lands here, because only the app runs as each user:
//
//   1. %APPDATA%\go-mapi      → %APPDATA%\DraftHorse      (settings, log)
//   2. %LOCALAPPDATA%\go-mapi → %LOCALAPPDATA%\DraftHorse (queue, browser
//      profile — the move preserves the IT-signed Edge session's cookies)
//   3. Credential Manager: service "go-mapi" → "DraftHorse"
//   4. Stale HKCU heal mirror Software\Clients\Mail\go-mapi — deleted, or
//      the mapi32 stub would resolve it FIRST (HKCU outranks HKLM) and load
//      a DLL path the installer just removed, breaking Send To for this
//      user. The ARRICKS-13 guard then heals the (Default) pointer itself.
//
// Idempotent by construction: every step is "if old exists and new doesn't".
// A fresh 4.0 install finds nothing and falls straight through. Failures are
// logged and skipped — a half-migrated user signs in again and re-pairs; a
// blocked migration must never stop the app from starting.
//
// Called as the FIRST statement of startup(), before the tray, watcher, log
// writes, or auth bootstrap can touch the new paths.

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/zalando/go-keyring"
	"golang.org/x/sys/windows/registry"
)

const legacyName = "go-mapi" // the pre-4.0 identifier; never used for new state

// migrateLegacyState runs all per-user migration steps. Errors are collected
// into the log only after the APPDATA move so the log lands in the NEW dir.
func migrateLegacyState() {
	// Order matters: APPDATA first (the log file lives there), then
	// LOCALAPPDATA, then the pieces that can log freely.
	var notes []string
	if appData := os.Getenv("APPDATA"); appData != "" {
		notes = append(notes, migrateDir(filepath.Join(appData, legacyName), filepath.Join(appData, "DraftHorse"))...)
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		notes = append(notes, migrateDir(filepath.Join(localAppData, legacyName), filepath.Join(localAppData, "DraftHorse"))...)
	}
	notes = append(notes, migrateKeyringEntry()...)
	notes = append(notes, removeLegacyHealMirror()...)
	notes = append(notes, removeLegacyApplicationsKey()...)
	for _, n := range notes {
		logInfo("migrate: %s", n)
	}
}

// migrateDir moves old → new. Fast path is a single rename (same volume, so
// the queue and browser profile move atomically). If the new dir already
// exists — a log write or fresh component beat us to it — entries are merged
// individually, existing new-side entries win, and the old dir is removed
// only if it empties out. Returns human-readable notes; empty slice = no-op.
func migrateDir(oldDir, newDir string) []string {
	if _, err := os.Stat(oldDir); err != nil {
		return nil // nothing to migrate
	}
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		if err := os.Rename(oldDir, newDir); err == nil {
			return []string{"moved " + oldDir + " -> " + newDir}
		}
		// Rename can fail on a locked file (dedicated Edge still open);
		// fall through to the per-entry merge, which moves what it can.
	}
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return []string{"cannot read " + oldDir + ": " + err.Error()}
	}
	notes := []string{}
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return []string{"cannot create " + newDir + ": " + err.Error()}
	}
	moved, skipped := 0, 0
	for _, e := range entries {
		src, dst := filepath.Join(oldDir, e.Name()), filepath.Join(newDir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			skipped++ // new side already has it — never clobber
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			skipped++
			continue
		}
		moved++
	}
	if rest, err := os.ReadDir(oldDir); err == nil && len(rest) == 0 {
		_ = os.Remove(oldDir)
	}
	if moved > 0 || skipped > 0 {
		notes = append(notes, filepath.Base(oldDir)+" merge: moved "+strconv.Itoa(moved)+", left "+strconv.Itoa(skipped))
	}
	return notes
}

// migrateKeyringEntry moves the OAuth token blob to the new credential
// target. Runs before bootstrapAuth, so a migrated token is picked up in the
// same session — the user never sees a signed-out state.
func migrateKeyringEntry() []string {
	if _, err := keyring.Get(keyringService, keyringUser); err == nil {
		return nil // new target already populated — old entry is stale; leave it
	}
	secret, err := keyring.Get(legacyName, keyringUser)
	if err != nil {
		return nil // no legacy entry either — fresh machine
	}
	if err := keyring.Set(keyringService, keyringUser, secret); err != nil {
		return []string{"credential migration failed (sign-in will re-pair): " + err.Error()}
	}
	_ = keyring.Delete(legacyName, keyringUser)
	return []string{"credentials moved to target " + keyringService}
}

// removeLegacyApplicationsKey deletes the per-user browsed-app registration
// Windows auto-creates under Software\Classes\Applications when a user ever
// picks the exe via "look for another app on this PC". Left behind, it keeps
// a dead "go-mapi" row (old icon, old name) in every Default Apps picker
// next to the real DraftHorse entry — found by Dave on the first 4.0
// migration retest. The key can have subkeys (shell\open\command,
// SupportedTypes), so delete depth-first.
func removeLegacyApplicationsKey() []string {
	base := `Software\Classes\Applications\go-mapi.exe`
	for _, sub := range []string{
		base + `\shell\open\command`,
		base + `\shell\open`,
		base + `\shell`,
		base + `\SupportedTypes`,
		base + `\DefaultIcon`,
		base,
	} {
		_ = registry.DeleteKey(registry.CURRENT_USER, sub)
	}
	// Report only if the base key is actually gone vs was never there —
	// DeleteKey's error can't distinguish, so probe.
	if k, err := registry.OpenKey(registry.CURRENT_USER, base, registry.QUERY_VALUE); err == nil {
		k.Close()
		return []string{`stale HKCU Applications\go-mapi.exe survived (subkey layout unexpected)`}
	}
	return nil
}

// removeLegacyHealMirror deletes the pre-4.0 HKCU self-heal mirror. The
// (Default) client pointer is NOT rewritten here — the ARRICKS-13 guard
// owns that and runs right after startup.
func removeLegacyHealMirror() []string {
	err := registry.DeleteKey(registry.CURRENT_USER, mapiClientsMailPath+`\`+legacyName)
	if err != nil {
		return nil // absent on fresh machines — the overwhelmingly common case
	}
	return []string{"removed stale HKCU heal mirror " + legacyName}
}
