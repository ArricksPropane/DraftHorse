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
//   5. Windows' browsed-app auto-registration for the old exe
//      (Software\Classes\Applications\go-mapi.exe) — a dead Default Apps
//      row otherwise.
//
// Runs to completion ONCE per user: a marker file in the new APPDATA dir
// short-circuits every later launch (review 2026-08-28 — without it every
// start paid two Credential Manager round-trips and eight registry calls
// forever, all no-ops). Steps are idempotent, so a crash mid-pass simply
// reruns next launch; failures are logged and skipped — a blocked
// migration must never stop the app from starting.
//
// Called as the FIRST statement of startup(), before the tray, watcher, log
// writes, or auth bootstrap can touch the new paths.

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/zalando/go-keyring"
	"golang.org/x/sys/windows/registry"
)

const legacyName = "go-mapi" // the pre-4.0 identifier; never used for new state

// migrationMarker names the file whose presence means "this user is done".
const migrationMarker = ".migrated-v4"

// migrateLegacyState runs all per-user migration steps. Errors are collected
// into the log only after the APPDATA move so the log lands in the NEW dir.
func migrateLegacyState() {
	appData := os.Getenv("APPDATA")
	newAppData := ""
	if appData != "" {
		newAppData = filepath.Join(appData, "DraftHorse")
		if _, err := os.Stat(filepath.Join(newAppData, migrationMarker)); err == nil {
			return // already migrated — the common case for the app's whole life
		}
	}
	// Order matters: APPDATA first (the log file lives there), then
	// LOCALAPPDATA, then the pieces that can log freely.
	var notes []string
	if appData != "" {
		notes = append(notes, migrateDir(filepath.Join(appData, legacyName), newAppData)...)
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		notes = append(notes, migrateDir(filepath.Join(localAppData, legacyName), filepath.Join(localAppData, "DraftHorse"))...)
	}
	notes = append(notes, migrateKeyringEntry(keyringStoreFactory())...)
	notes = append(notes, removeLegacyHealMirror()...)
	notes = append(notes, removeLegacyApplicationsKey()...)
	for _, n := range notes {
		logInfo("migrate: %s", n)
	}
	if newAppData != "" {
		if err := os.MkdirAll(newAppData, 0o700); err == nil {
			_ = os.WriteFile(filepath.Join(newAppData, migrationMarker), []byte("v4\n"), 0o600)
		}
	}
}

// migrateDir moves old → new. Fast path is a single rename (same volume, so
// the queue and browser profile move atomically). If the new dir already
// exists — a log write or the new DLL beat us to it — the trees are merged
// RECURSIVELY: files only the old side has move across, files both sides
// have keep the new copy, and directories present on both sides descend
// rather than being skipped wholesale (review 2026-08-28: a one-level merge
// stranded every pending scan when DraftHorse\queue already existed). The
// old tree is removed only where it empties out.
func migrateDir(oldDir, newDir string) []string {
	if _, err := os.Stat(oldDir); err != nil {
		return nil // nothing to migrate
	}
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		if err := os.Rename(oldDir, newDir); err == nil {
			return []string{"moved " + oldDir + " -> " + newDir}
		}
		// Rename can fail on a locked file (dedicated Edge still open);
		// fall through to the merge, which moves what it can.
	}
	moved, skipped := mergeTree(oldDir, newDir)
	if moved == 0 && skipped == 0 {
		return nil
	}
	return []string{filepath.Base(oldDir) + " merge: moved " + strconv.Itoa(moved) + ", left " + strconv.Itoa(skipped)}
}

// mergeTree is migrateDir's recursive worker. Returns counts of entries
// moved and entries left in place (new side already had a file there, or the
// rename failed).
func mergeTree(oldDir, newDir string) (moved, skipped int) {
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return 0, 1
	}
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return 0, len(entries)
	}
	for _, e := range entries {
		src, dst := filepath.Join(oldDir, e.Name()), filepath.Join(newDir, e.Name())
		dstInfo, err := os.Stat(dst)
		switch {
		case os.IsNotExist(err):
			if err := os.Rename(src, dst); err != nil {
				skipped++
			} else {
				moved++
			}
		case err == nil && dstInfo.IsDir() && e.IsDir():
			m, s := mergeTree(src, dst)
			moved += m
			skipped += s
		default:
			skipped++ // new side already has a file here — never clobber
		}
	}
	if rest, err := os.ReadDir(oldDir); err == nil && len(rest) == 0 {
		_ = os.Remove(oldDir)
	}
	return moved, skipped
}

// migrateKeyringEntry moves the OAuth token blob to the new credential
// target. Runs before bootstrapAuth, so a migrated token is picked up in the
// same session — the user never sees a signed-out state. Takes the store
// through the same seam AuthManager uses, so the e2e build's fake store and
// unit tests never touch the real Credential Manager.
func migrateKeyringEntry(store KeyringStore) []string {
	if _, err := store.Get(keyringService, keyringUser); err == nil {
		return nil // new target already populated — old entry is stale; leave it
	}
	secret, err := store.Get(legacyName, keyringUser)
	if err != nil {
		return nil // no legacy entry either — fresh machine
	}
	if err := store.Set(keyringService, keyringUser, secret); err != nil {
		return []string{"credential migration failed (sign-in will re-pair): " + err.Error()}
	}
	if err := store.Delete(legacyName, keyringUser); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return []string{"credentials copied to target " + keyringService + " (old entry not removed: " + err.Error() + ")"}
	}
	return []string{"credentials moved to target " + keyringService}
}

// removeLegacyHealMirror deletes the pre-4.0 HKCU self-heal mirror. The
// (Default) client pointer is NOT rewritten here — the ARRICKS-13 guard
// owns that and runs right after startup.
func removeLegacyHealMirror() []string {
	removed, err := deleteKeyTree(registry.CURRENT_USER, mapiClientsMailPath+`\`+legacyName)
	if err != nil {
		return []string{"stale HKCU heal mirror " + legacyName + " not removed: " + err.Error()}
	}
	if !removed {
		return nil // absent on fresh machines — the overwhelmingly common case
	}
	return []string{"removed stale HKCU heal mirror " + legacyName}
}

// removeLegacyApplicationsKey deletes the per-user browsed-app registration
// Windows auto-creates under Software\Classes\Applications when a user ever
// picks the exe via "look for another app on this PC". Left behind, it keeps
// a dead "go-mapi" row (old icon, old name) in every Default Apps picker
// next to the real DraftHorse entry — found by Dave on the first 4.0
// migration retest.
func removeLegacyApplicationsKey() []string {
	removed, err := deleteKeyTree(registry.CURRENT_USER, `Software\Classes\Applications\`+legacyName+`.exe`)
	if err != nil {
		return []string{"stale HKCU Applications\\" + legacyName + ".exe not removed: " + err.Error()}
	}
	if !removed {
		return nil
	}
	return []string{"removed stale HKCU Applications\\" + legacyName + ".exe"}
}

// deleteKeyTree removes a registry key and everything under it, whatever its
// layout — Windows writes shell\open\ddeexec, SupportedTypes, DefaultIcon and
// more under Applications keys depending on the build, so a hand-enumerated
// subkey list (the first version of this file) silently left keys behind.
// Returns (false, nil) when the key was never there, distinguishing "absent"
// from "delete failed".
func deleteKeyTree(root registry.Key, path string) (bool, error) {
	k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	subs, err := k.ReadSubKeyNames(-1)
	k.Close()
	if err != nil {
		return false, err
	}
	for _, sub := range subs {
		if _, err := deleteKeyTree(root, path+`\`+sub); err != nil {
			return false, err
		}
	}
	if err := registry.DeleteKey(root, path); err != nil {
		return false, err
	}
	return true, nil
}
