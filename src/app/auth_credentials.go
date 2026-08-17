package main

import "os"

// Injected at build time via:
//   -ldflags "-X 'main.oauthClientID=...' -X 'main.oauthClientSecret=...'"
// `var` (not const) is REQUIRED — -X only overwrites string vars. Mirrors the
// pattern used for `Version` in main.go (line 15).
var (
	oauthClientID     = ""
	oauthClientSecret = ""
)

// init lets `wails dev` pick up credentials from environment variables
// (populated from .env.local via scripts/dev-wails.ps1) without requiring
// the developer to repeat the -ldflags dance each run.
//
// ARRICKS-12 (R12): ldflags-injected values now WIN over the environment.
// The old order let any machine-local GOMAPI_OAUTH_* env var silently
// replace the OAuth client identity baked into a release binary — an
// unprivileged persistence vector for redirecting the auth flow. Dev builds
// are unaffected: they have no ldflags values, so the vars still apply.
//
// Per CONTEXT D-09, the env var names are the same as the GitHub secrets
// used by CI in Phase 10: GOMAPI_OAUTH_CLIENT_ID, GOMAPI_OAUTH_CLIENT_SECRET.
func init() {
	if oauthClientID == "" {
		if v := os.Getenv("GOMAPI_OAUTH_CLIENT_ID"); v != "" {
			oauthClientID = v
		}
	}
	if oauthClientSecret == "" {
		if v := os.Getenv("GOMAPI_OAUTH_CLIENT_SECRET"); v != "" {
			oauthClientSecret = v
		}
	}
}
