//go:build !gomapi_debug_browser

package webviewloader

// debugBrowserArgs is the production no-op. ARRICKS-12 (R11): the
// GOMAPI_DEBUG_BROWSER_ARGS hook let any process able to set an environment
// variable inject arbitrary Chromium switches (e.g.
// --remote-debugging-port, which exposes CDP control of the authenticated
// WebView) into release builds. The hook now only exists in builds compiled
// with -tags gomapi_debug_browser; release binaries compile this no-op and
// ignore the variable entirely.
func debugBrowserArgs(existing string) string {
	return existing
}
