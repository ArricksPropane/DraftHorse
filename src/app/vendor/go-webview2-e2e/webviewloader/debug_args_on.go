//go:build gomapi_debug_browser

package webviewloader

import "os"

// debugBrowserArgs appends GOMAPI_DEBUG_BROWSER_ARGS to the browser argument
// list. E2E-test builds only (-tags gomapi_debug_browser) — see
// debug_args_off.go for why this must never compile into a release binary.
// The env var is read BEFORE preventEnvAndRegistryOverrides fires, matching
// the original Phase 11 plan-06 patch.
func debugBrowserArgs(existing string) string {
	extra := os.Getenv("GOMAPI_DEBUG_BROWSER_ARGS")
	if extra == "" {
		return existing
	}
	if existing == "" {
		return extra
	}
	return existing + " " + extra
}
