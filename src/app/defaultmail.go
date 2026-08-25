package main

// ARRICKS-13 — default mail app guard.
//
// Two different "defaults" exist, with very different rules:
//
//  1. The Simple MAPI client (HKLM/HKCU Software\Clients\Mail (Default)).
//     This is what scanner software and Explorer "Send to → Mail recipient"
//     resolve through, and it is plain registry data — checkable and
//     settable. The installer claims the HKLM value, but a later
//     Outlook/Thunderbird install can silently steal it and break every
//     scan-to-email flow until someone reinstalls. This guard detects the
//     theft and self-heals through the per-user layer: HKCU\Software\Clients
//     takes precedence over HKLM in the stub's resolution and is writable
//     without elevation. The heal writes a complete self-consistent mirror
//     (client subkey + DLLPath + the (Default) pointer) because the stub
//     opens the client subkey in whichever hive it read the pointer from.
//     HKCU\SOFTWARE is not WOW64-redirected, so one write serves both
//     bitnesses, and the REG_EXPAND_SZ %ProgramFiles% value keeps the
//     per-bitness DLL routing from ARRICKS-10. Installer-smoke test 30
//     proves the HKCU override end-to-end through the real mapi32 stub.
//
//  2. The mailto: handler (Settings > Default apps / UserChoice). Windows 11
//     hash-protects UserChoice and the UCPD driver blocks programmatic
//     writes even by hash-forgers — deliberately. We only DETECT this one
//     and deep-link the user to the exact Settings page where one click
//     finishes the job. Fleet-wide, the Intune associations XML remains the
//     zero-touch path (docs/mailto-default-associations.xml).

import (
	"time"

	"github.com/pkg/browser"
	"golang.org/x/sys/windows/registry"
)

// mapiClientsMailPath is a var only as a test seam: the write-shape test
// redirects it into a sandbox HKCU subtree. Production never mutates it.
var mapiClientsMailPath = `Software\Clients\Mail`

const (
	// V4: identifier and display are both DraftHorse (the ARRICKS-11
	// go-mapi identifier split ended at 4.0 — see CLAUDE.md Branding and
	// docs/V4-PLAN.md). These MUST stay in lockstep with the installer's
	// PRODUCT_NAME writes; the mapi32 stub resolves the subkey named by
	// the Clients\Mail (Default) value.
	mapiClientName = "DraftHorse"
	mapiClientDisplay   = "DraftHorse"
	mapiDLLPathExpand   = `%ProgramFiles%\DraftHorse\DraftHorse.dll`

	mailtoUserChoicePath = `Software\Microsoft\Windows\Shell\Associations\UrlAssociations\mailto\UserChoice`
	mailtoProgID         = "DraftHorse.mailto"

	// Win11 deep link straight to this app's Default Apps page (machine
	// registration → display name). Unknown parameters degrade gracefully
	// to the Default Apps landing page, which also covers Win10.
	defaultAppsSettingsURI = "ms-settings:defaultapps?registeredAppMachine=DraftHorse"

	defaultMailGuardInterval = 1 * time.Hour
)

// openSettingsURL is the launch seam; tests swap it to capture the URI.
var openSettingsURL = browser.OpenURL

// DefaultsStatus is the frontend-facing snapshot of both defaults.
type DefaultsStatus struct {
	// MapiDefault: the effective Simple MAPI client is this app (after any
	// self-heal this session).
	MapiDefault bool `json:"mapiDefault"`
	// MailtoDefault: the mailto UserChoice ProgId is this app's.
	MailtoDefault bool `json:"mailtoDefault"`
}

// readMapiEffectiveClient returns the client name the stub would resolve:
// the HKCU (Default) pointer when present and non-empty, else HKLM's.
func readMapiEffectiveClient() string {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(root, mapiClientsMailPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("")
		k.Close()
		if err == nil && v != "" {
			return v
		}
	}
	return ""
}

// hkcuMirrorIntact reports whether the per-user client subkey carries the
// DLLPath the self-heal writes. A pointer without a resolvable subkey would
// break stubs that resolve the subkey in the same hive as the pointer.
func hkcuMirrorIntact() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, mapiClientsMailPath+`\`+mapiClientName, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("DLLPath")
	return err == nil && v != ""
}

// mailClientNeedsRepair is the pure decision: repair when the effective
// client is someone else, or when the HKCU pointer names us but the mirror
// subkey is missing (a half-written or half-deleted state).
func mailClientNeedsRepair(effective, hkcuPtr string, mirrorIntact bool) bool {
	if effective != mapiClientName {
		return true
	}
	if hkcuPtr == mapiClientName && !mirrorIntact {
		return true
	}
	return false
}

func readHKCUMailPointer() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, mapiClientsMailPath, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue("")
	if err != nil {
		return ""
	}
	return v
}

// repairMailClientDefault writes the per-user mirror + pointer. Unelevated
// by design. Values mirror the installer's HKLM registration (ARRICKS-10).
func repairMailClientDefault() error {
	sub, _, err := registry.CreateKey(registry.CURRENT_USER, mapiClientsMailPath+`\`+mapiClientName, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer sub.Close()
	if err := sub.SetStringValue("", mapiClientDisplay); err != nil {
		return err
	}
	if err := sub.SetExpandStringValue("DLLPath", mapiDLLPathExpand); err != nil {
		return err
	}
	parent, _, err := registry.CreateKey(registry.CURRENT_USER, mapiClientsMailPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.SetStringValue("", mapiClientName)
}

// ensureDefaultMailClient checks and, when needed, self-heals the Simple
// MAPI default. Returns whether a repair was applied.
func ensureDefaultMailClient() (bool, error) {
	if !mailClientNeedsRepair(readMapiEffectiveClient(), readHKCUMailPointer(), hkcuMirrorIntact()) {
		return false, nil
	}
	if err := repairMailClientDefault(); err != nil {
		return false, err
	}
	return true, nil
}

// readMailtoDefault reports whether this app owns the mailto UserChoice.
func readMailtoDefault() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, mailtoUserChoicePath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("ProgId")
	return err == nil && v == mailtoProgID
}

// GetDefaultsStatus is a Wails binding: the frontend's status row queries
// it on mount and window-show. Read-only; the guard goroutine owns writes.
func (a *App) GetDefaultsStatus() DefaultsStatus {
	return DefaultsStatus{
		MapiDefault:   readMapiEffectiveClient() == mapiClientName,
		MailtoDefault: readMailtoDefault(),
	}
}

// OpenDefaultAppsSettings is a Wails binding: deep-links the user to
// Settings > Default apps at this app's page, where a single click sets the
// mailto default — the only Windows-supported path (see file comment).
func (a *App) OpenDefaultAppsSettings() error {
	return openSettingsURL(defaultAppsSettingsURI)
}

// startDefaultMailGuard runs the check at startup and then hourly until
// shutdown. Repairs are logged; no toast — a self-heal that worked is not
// news the user can act on, and the hourly cadence would make noise if a
// competing client kept re-claiming (visible in app.log if so).
func (a *App) startDefaultMailGuard() {
	check := func() {
		repaired, err := ensureDefaultMailClient()
		if err != nil {
			logError("defaultmail: self-heal failed: %v", err)
		} else if repaired {
			logInfo("defaultmail: MAPI client default was not %q — repaired via HKCU mirror", mapiClientName)
		}
		// ARRICKS-20: same cadence heals the SendTo menu plumbing (file +
		// extension association + stale UserChoice) — see sendto.go.
		sendToRepairs, stErr := ensureSendToMailRecipient()
		if stErr != nil {
			logError("sendto: self-heal failed: %v", stErr)
		}
		if len(sendToRepairs) > 0 {
			logInfo("sendto: repaired %v", sendToRepairs)
		}
	}
	check()
	go func() {
		tick := time.NewTicker(defaultMailGuardInterval)
		defer tick.Stop()
		for {
			select {
			case <-a.shutdownCtx.Done():
				return
			case <-tick.C:
				check()
			}
		}
	}()
}
