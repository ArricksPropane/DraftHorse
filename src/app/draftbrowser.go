package main

// ARRICKS-21 — open drafts in a dedicated, isolated browser profile.
//
// The fleet model is delegated per-location mailboxes: the app is signed
// into the location account, while the staff member's own browser is
// signed in as THEM (the location mailbox is only reachable through Gmail's
// delegation hop, which has no URL). A draft link aimed at the user's
// default browser therefore lands in the wrong account — Gmail's dead
// "Temporary Error (404)" page in practice. Every Windows 11 PC already
// ships a Chromium (Edge), and Edge/Chrome honor --user-data-dir, so the
// app launches the draft in its OWN profile under DraftHorse's LOCALAPPDATA:
// IT signs that profile into the location account once (the AccountChooser
// link prompts for exactly that on first run), and from then on every draft
// opens in the right account regardless of what the user's daily browser
// is doing. No bundled browser: Microsoft/Google patch it, Google sign-in
// sees a normal browser (embedded WebViews are blocked at sign-in).

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// errNoChromiumBrowser signals "fall back to the default browser".
var errNoChromiumBrowser = errors.New("no Edge or Chrome found via App Paths")

// launchDraftInDedicatedBrowser is the seam draftlink.go calls; tests swap it.
var launchDraftInDedicatedBrowser = launchDedicatedBrowser

// dedicatedBrowserProfileDir is the per-user isolated profile. Lives beside
// the queue under %LOCALAPPDATA%\DraftHorse; the uninstaller removes it (it
// holds the location account's session cookies).
func dedicatedBrowserProfileDir() string {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "DraftHorse", "browser-profile")
	}
	if cacheDir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cacheDir, "DraftHorse", "browser-profile")
	}
	return filepath.Join(".", "DraftHorse", "browser-profile")
}

// dedicatedBrowserArgs is the pure argument builder (tested). --no-first-run
// and --no-default-browser-check suppress the fresh-profile onboarding pages.
//
// ARRICKS-28: --new-window is deliberately ABSENT (it was here through
// ARRICKS-21..27). Chromium treats a profile as a singleton: launching a
// second time with the same --user-data-dir hands the URL to the already-
// running instance instead of starting a new browser. With --new-window that
// handoff spawned a fresh window per scan, so a day of scanning buried the
// desktop in Gmail windows. Without it, the URL opens as a tab in the profile's
// existing window and Chromium raises that window to the foreground — one
// window, correct account, and the tab is still a fresh server fetch of the
// Drafts list (the ARRICKS-27 requirement: a raised-but-unreloaded tab would
// show the PRE-scan list and hide the very draft we just made).
//
// Tabs still accumulate inside that one window. Closing them would require
// driving the browser over CDP (--remote-debugging-port), which means an
// unauthenticated local debug port on the profile holding the location
// mailbox's session cookies — explicitly rejected 2026-08-25. One window with
// several tabs is the accepted trade.
func dedicatedBrowserArgs(profileDir, url string) []string {
	return []string{
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		url,
	}
}

// findChromiumBrowser resolves msedge.exe (preferred — guaranteed on
// Windows 11) or chrome.exe through the App Paths registry, machine then
// per-user. App Paths is a WOW64 shared key, so one read covers both views.
func findChromiumBrowser() (string, bool) {
	for _, exe := range []string{"msedge.exe", "chrome.exe"} {
		for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
			k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\App Paths\`+exe, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			p, _, err := k.GetStringValue("")
			k.Close()
			if err != nil || p == "" {
				continue
			}
			if _, err := os.Stat(p); err == nil {
				return p, true
			}
		}
	}
	return "", false
}

// launchDedicatedBrowser starts the browser detached (no wait) in the
// dedicated profile. Returns errNoChromiumBrowser when neither browser is
// installed so the caller can fall back to the default browser.
func launchDedicatedBrowser(url string) error {
	exe, ok := findChromiumBrowser()
	if !ok {
		return errNoChromiumBrowser
	}
	profile := dedicatedBrowserProfileDir()
	if err := os.MkdirAll(profile, 0o700); err != nil {
		return err
	}
	cmd := exec.Command(exe, dedicatedBrowserArgs(profile, url)...)
	return cmd.Start()
}
