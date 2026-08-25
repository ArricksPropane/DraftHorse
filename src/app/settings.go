//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// AppSettings is the persisted per-user settings for DraftHorse. Phase 9 ships
// `Mode`; Phase 11 adds the flat update-check fields. Future phases may add
// flat fields — do NOT nest. Marshaled to %APPDATA%\DraftHorse\settings.json
// via saveSettings (atomic, crash-safe).
//
// Pause state is INTENTIONALLY not persisted (D-15) — resets on every app
// start to prevent "I paused a month ago and forgot" silent failure mode.
//
// Update-check fields (Phase 11, D-05/D-07/D-08):
//   - UpdateChecksEnabled: user opt-in toggle (REL-05). ARRICKS-06: defaults
//     to false — this deployment updates through Intune, so the notify check
//     is strictly opt-in (see defaultAppSettings).
//   - LastUpdateCheck: RFC3339 timestamp of the most recent check attempt
//     (success OR failure). Empty string on first run / never-checked.
//     Consumed by the cadence gate and by tray/frontend "Last checked" status
//     (D-07). Background goroutines MUST NOT call saveSettings directly —
//     persistence is routed through an App-owned guarded writer (Task 2)
//     that preserves the single-writer atomic-save invariant.
type AppSettings struct {
	Mode                string `json:"mode"`                         // "manual" | "auto-draft"
	UpdateChecksEnabled bool   `json:"update_checks_enabled"`        // ARRICKS-06 default off
	LastUpdateCheck     string `json:"last_update_check,omitempty"`  // RFC3339, "" = never checked
	OpenDraftInBrowser  bool   `json:"open_draft_in_browser"`        // ARRICKS-08 default on
	// ARRICKS-21: open drafts in DraftHorse's own isolated Edge/Chrome
	// profile (signed into the location account once by IT) instead of the
	// user's default browser. Default on — the fleet model is delegated
	// per-location mailboxes, where the user's own browser session is the
	// wrong account by design.
	DraftBrowserDedicated bool `json:"draft_browser_dedicated"`
	// ARRICKS-23: milliseconds to wait between draft creation and the
	// browser open. Gmail's web client syncs API-created drafts on its own
	// schedule — open the compose view too fast and it renders without the
	// attachment chip until reopened (same behavior Affixa had). Default
	// 4000; field-tunable via settings.json without a rebuild; 0 disables.
	DraftOpenDelayMs int `json:"draft_open_delay_ms"`
	// V4 two-account support: the account slot (0 or 1) that scans draft
	// to. Set from the tray radio rows or the window switcher; every other
	// value clamps to 0 so a hand-edited file cannot point at nothing.
	ActiveAccount int `json:"active_account"`
}

const defaultMode = "manual"

// defaultAppSettings returns the settings used on first run, corrupt file,
// or any other fallback path. Keeping this one builder means every fallback
// code path gets the same defaults — notably ARRICKS-06 UpdateChecksEnabled=false.
func defaultAppSettings() AppSettings {
	return AppSettings{
		Mode:                defaultMode,
		// ARRICKS-06: default off. Upstream defaults this on, which contacts
		// api.github.com every 24h from every user session. We deploy
		// deliberately through Intune, so the check has nothing useful to
		// tell us and is one less outbound dependency on 12 machines.
		UpdateChecksEnabled: false,
		// ARRICKS-08: after a draft is created, open it in the default
		// browser. Default on — the scan-to-email flow ends with the user
		// finishing the draft in Gmail, so surfacing it immediately saves
		// the hunt through mail.google.com. Toggleable from the tray menu.
		OpenDraftInBrowser: true,
		// ARRICKS-21: see the field comment. Falls back to the default
		// browser automatically when no Edge/Chrome is found.
		DraftBrowserDedicated: true,
		// ARRICKS-23: see the field comment.
		DraftOpenDelayMs: 4000,
	}
}

// settingsPath returns the full path to settings.json.
// Lives under the per-user appDataDir() (see paths.go) — RDS-safe per-user scope.
func settingsPath() string {
	return filepath.Join(appDataDir(), "settings.json")
}

// loadSettings reads %APPDATA%\DraftHorse\settings.json. Returns defaults on
// any read/parse error (first-run missing file, corrupt file, unknown Mode
// value). Corrupt files are NOT moved aside in Phase 9 — D-15 scope means
// the only persisted field is Mode, and resetting to "manual" on a corrupt
// read is the safe default (fail-closed — no auto-draft without a valid
// mode setting).
func loadSettings() AppSettings {
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		// ENOENT or any other read error → defaults. First-run is the
		// common case; no log to avoid noise.
		return defaultAppSettings()
	}
	// Intermediate shape with a *bool for the update toggle so we can tell
	// "field absent from JSON" (partial/old file) from "field explicitly
	// set to false" (user opted out). Absent → default true per D-08.
	var raw struct {
		Mode                string `json:"mode"`
		UpdateChecksEnabled *bool  `json:"update_checks_enabled"`
		LastUpdateCheck     string `json:"last_update_check"`
		OpenDraftInBrowser  *bool  `json:"open_draft_in_browser"`
		DraftBrowserDedicated *bool `json:"draft_browser_dedicated"`
		DraftOpenDelayMs      *int  `json:"draft_open_delay_ms"`
		ActiveAccount         *int  `json:"active_account"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		logError("settings: parse error, using defaults: %v", err)
		return defaultAppSettings()
	}
	out := defaultAppSettings()
	if raw.Mode == "manual" || raw.Mode == "auto-draft" {
		out.Mode = raw.Mode
	}
	// raw.Mode ∉ {"manual","auto-draft"}: keep defaultAppSettings().Mode.
	if raw.UpdateChecksEnabled != nil {
		out.UpdateChecksEnabled = *raw.UpdateChecksEnabled
	}
	// Absent → default true (pre-ARRICKS-08 settings files); explicit
	// false = user opted out via the tray toggle.
	if raw.OpenDraftInBrowser != nil {
		out.OpenDraftInBrowser = *raw.OpenDraftInBrowser
	}
	// ARRICKS-21: absent (pre-3.7 settings files) → default true.
	if raw.DraftBrowserDedicated != nil {
		out.DraftBrowserDedicated = *raw.DraftBrowserDedicated
	}
	// V4: absent (pre-4.0 files) → slot 0; anything but 0/1 clamps to 0.
	if raw.ActiveAccount != nil && *raw.ActiveAccount == 1 {
		out.ActiveAccount = 1
	}
	// ARRICKS-23: absent → default 4000ms; clamp field-typos to [0, 60s].
	if raw.DraftOpenDelayMs != nil {
		d := *raw.DraftOpenDelayMs
		if d < 0 {
			d = 0
		}
		if d > 60000 {
			d = 60000
		}
		out.DraftOpenDelayMs = d
	}
	out.LastUpdateCheck = raw.LastUpdateCheck
	return out
}

// saveSettings writes s atomically. Pattern (RESEARCH §3):
//  1. Marshal to JSON.
//  2. MkdirAll the target dir.
//  3. Create tmp file in SAME directory (same-volume required for atomic rename).
//  4. Write + Sync + Close the tmp file.
//  5. windows.MoveFileEx(tmp, dest, REPLACE_EXISTING|WRITE_THROUGH) — atomic on NTFS.
//  6. On any error, best-effort unlink tmp.
//
// SINGLE-WRITER INVARIANT: only call from Wails bindings (UI thread). Never
// from a goroutine (automode, watcher, auth refresh). D-13 + RESEARCH §3.
func saveSettings(s AppSettings) error {
	path := settingsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("settings: mkdir: %w", err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("settings: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "settings-*.tmp")
	if err != nil {
		return fmt.Errorf("settings: create tmp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// Best-effort cleanup on any error path. If rename succeeded, tmp
		// is already gone and this Remove is a no-op (ENOENT).
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("settings: write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("settings: sync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("settings: close tmp: %w", err)
	}
	if err := moveFileAtomic(tmpPath, path); err != nil {
		return fmt.Errorf("settings: atomic rename: %w", err)
	}
	return nil
}

// moveFileAtomic wraps windows.MoveFileEx with REPLACE_EXISTING + WRITE_THROUGH.
// On Windows, os.Rename is NOT atomic when the target exists (issue #8914).
// MoveFileEx IS atomic on NTFS for same-volume source/dest. WRITE_THROUGH
// forces a physical disk commit before return (crash-safe at the cost of
// ~1-5ms latency per save — acceptable for UI-triggered writes).
func moveFileAtomic(src, dst string) error {
	srcW, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	dstW, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(
		srcW,
		dstW,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}
