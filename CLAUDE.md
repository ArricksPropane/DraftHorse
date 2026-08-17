# CLAUDE.md — Arrick's Propane fork of go-mapi

Read this before changing anything. It exists so the reasoning behind this fork
survives whoever set it up.

Upstream's own project/stack reference is preserved at
`docs/UPSTREAM-CLAUDE.md`. Per-change rationale is in `PATCHES.md`.

## What this is

A fork of `marcfargas/go-mapi`, a Windows Simple MAPI → Gmail bridge. It
registers a DLL as the system default mail client so "Send to → Mail recipient"
in any Windows application creates a Gmail draft with the attachments in place.

Fork additions on top of that: created drafts open in the browser
(ARRICKS-08, `src/app/draftlink.go`, tray-toggleable), and the app registers
as a `mailto:` handler through the Windows Default Apps model (ARRICKS-09,
`go-mapi.mailto` ProgID + Capabilities/RegisteredApplications in the
installer). A mailto click runs `go-mapi.exe --mailto "%1"`, which opens
Gmail web compose prefilled from the URL and exits — no draft API call, no
auth, still nothing ever sent. Fleet-wide default via Intune:
`docs/mailto-default-associations.xml`.

A default-mail guard (ARRICKS-13, `src/app/defaultmail.go`) checks at startup
and hourly that the Simple MAPI default still resolves to this app, and
self-heals a stolen default through an unelevated HKCU mirror (the stub reads
HKCU\Software\Clients\Mail before HKLM — proven end-to-end by installer-smoke
test 30). The mailto default CANNOT be set programmatically (hash-protected
UserChoice + the UCPD driver — deliberate Windows policy, do not fight it);
the app only detects it and deep-links the user to Settings > Default apps.

**Branding (ARRICKS-11):** the app's display name is **DraftHorse** — it only
ever creates drafts; nothing sends. Display surfaces (installer UI, ARP
DisplayName, Start Menu shortcut `DraftHorse.lnk`, Default Apps
ApplicationName, the `Clients\Mail\go-mapi` subkey's `(Default)` value, tray,
toasts, frontend) say DraftHorse. Every **identifier** stays `go-mapi`: the
`Clients\Mail\go-mapi` key name and the `Clients\Mail` `(Default)` resolver
value (the mapi32 stub opens the subkey named by that string), installed
binary/DLL names, AUMID `com.marcfargas.gomapi`, `go-mapi.mailto` ProgID,
credential target, queue/config paths, firewall rule, uninstall key path,
Intune detection. Do not rename identifiers as part of branding work — the
split is deliberate (upgrade continuity + everything above was CI-verified
under these names). One deliberate exception (ARRICKS-14): the installer
FILENAME is `DraftHorse-setup.exe` — it's a download-facing display surface,
not an installed identifier. Release binaries older than that change carry
the old `go-mapi-setup.exe` stable-download URL baked in; their "Download
installer" button 404s once newer releases ship (their release-page link
still works). The GCP OAuth consent-screen app name must match the
DraftHorse copy in `PreAuthModal.svelte`.

**Why a fork rather than upstream binaries:** it replaces Affixa, which retires
**31 January 2027**. Owning the source is the entire point — the fork is what
makes us independent of a single upstream maintainer.

**Primary use case: scanner "Scan to Email."** ScanSnap and Epson document
scanners on ~12 Windows 11 PCs, Google Workspace tenant `arrickspropane.com`.
When judging any change, ask what it does to a scan-to-email workflow first.

## Architecture

| Component | Language | Path | Role |
|---|---|---|---|
| MAPI interceptor | C++17 | `src/interceptor/` | Loaded in-process by every MAPI-calling app. Writes email JSON + copies attachments into the queue. |
| Shared core | Go | `internal/mapi/` | Queue watcher, validation, Gmail client, MIME builder |
| Desktop app | Go + Svelte 5 (Wails/WebView2) | `src/app/` | Tray, auth, draft creation |
| Installer | NSIS | `src/installer/` | Mail-client registration, both bitnesses |

Queue: `%LOCALAPPDATA%\go-mapi\queue\`. Tokens: Windows Credential Manager
(`go-mapi` / `oauth-tokens`). **Nothing is ever sent** — the only Gmail endpoint
called is `POST /gmail/v1/users/me/drafts`.

## House rules

These encode decisions that cost real analysis. Do not undo them casually.

1. **Never reintroduce the silent auto-updater.** Upstream ships a Scheduled
   Task running as SYSTEM that downloads binaries and swaps them over
   `%ProgramFiles%`, verified only against a SHA-256 manifest served from the
   same URL as the binaries — that detects corruption, not a compromised
   release. We removed it (`updates_silent*.go`, NSIS `RegisterScheduledTask`,
   the `/AUTOUPDATE=` parser). `--update-check-silent` survives as an explicit
   no-op on purpose, so a task left behind by a prior upstream install cannot
   do anything. **Keep `un.RemoveScheduledTask`** in the uninstaller for the
   same reason.
2. **`gitHubOwner` stays `egkrateia247`** (`src/app/updates.go`). Upstream
   treats that constant as a security control; for a fork, leaving it unchanged
   means any surviving update path pulls a third party's binaries over our
   signed builds.
3. **Always build both x86 and x64 interceptor DLLs.** Scanner software is
   usually 32-bit while Office is 64-bit. Shipping one bitness is the classic
   way this breaks.
4. **Nothing touching the filesystem, registry, or shell APIs goes in
   `DllMain`.** It runs under the loader lock and this DLL loads into
   `explorer.exe`. Queue directories are created lazily at write time.
5. **Attachment basenames are always sanitized** via
   `message_converter::SanitizeFilename` before hitting the filesystem. Not
   theoretical: a colon in a scanner filename previously failed `CopyFileW` and
   discarded the *entire email*.
6. **Least privilege on OAuth scopes.** `gmail.compose` + userinfo only. We
   dropped `gmail.send` because nothing sends. Keep the GCP consent screen set
   to **Internal** user type — that is what bounds exposure of the client
   secret baked into the binary.
7. **Never log email bodies, subjects, or recipient addresses.** `logging.go`
   states this contract; upstream honors it and so do we.

## Build

Requires Windows. The toolchain is pinned and fussy — `build.ps1` hardcodes the
triple-prefixed clang path, so other MinGW distributions will not work.

```powershell
# one-time
scoop install mingw-mstorsjo-llvm-ucrt   # bundles cmake + ninja
scoop bucket add extras; scoop install nsis
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

# build
npm ci
npm run build:interceptor        # x64 AND x86 — both must succeed
cd src/app
wails build -platform windows/amd64 -ldflags "-X main.Version=$VERSION -X main.oauthClientID=$env:GOMAPI_OAUTH_CLIENT_ID -X main.oauthClientSecret=$env:GOMAPI_OAUTH_CLIENT_SECRET"
```

OAuth credentials live in `.env.local` (gitignored) for dev and are injected via
ldflags for release. They are never committed.

**Always verify the x86 export table before shipping.** On 32-bit,
`STDAPICALLTYPE` is `__stdcall`, so the real symbol is `_MAPISendMail@20` while
the `.def` lists the undecorated name — a known MinGW/lld footgun. A silently
broken x86 export table breaks 32-bit scanner software while everything else
looks fine:

```powershell
llvm-nm --extern-only src\interceptor\build-x86\bin\go-mapi.dll | Select-String MAPI
```

## Test

```powershell
npm run test              # Go + Vitest
npm run check             # go vet + svelte-check
npm run test:interceptor  # DLL harness — includes the MAPISendDocuments case
go test -race ./internal/mapi/... ./src/app/...
```

The C++ unit tests link the same translation unit as the DLL, so they are
meaningful. The DLL harness was repaired in this fork (it had been watching the
wrong directory since upstream moved the queue) and is still not wired into
CTest — run it explicitly.

## State of the fork

Six commits on top of upstream `b90fcb0` (v3.0.0):

1. `DllMain` loader-lock fix
2. Attachment basename sanitization (+ the full-path fallback bug)
3. `MAPISendDocuments` implemented — was a stub returning success while
   silently discarding mail; *the* scanner-critical fix
4. DLL test harness repaired, `MAPISendDocuments` covered
5. Silent auto-updater removed, origin repointed
6. `gmail.send` scope dropped

### Known-unverified

Written and reviewed but **never compiled** — the work was done on Linux with
no Go module proxy. Treat the first successful Windows build as the real check:

- C++ touching Windows APIs: `FsUtils::RemoveAttachmentsDirForStem`
  (`FindFirstFileW` enumeration), the `test_utils.cpp` `SHGetFolderPathW`
  change, `test_send_documents.cpp`
- The Go build in its entirety
- `installer.Tests.ps1` (brace balance checked only)

Verified: pure C++ logic under `clang++ -Wall -Wextra`, 30 passing assertions;
`makensis` compiles the patched installer clean; `gofmt -e` clean; no dangling
references to deleted symbols.

### R-series hardening (ARRICKS-12) — done

The follow-up items originally listed here landed as one series:

- Attachment paths are constrained to the queue dir at watcher load
  (`pathWithinDir`, `internal/mapi/watcher.go`) — a tampered JSON can no
  longer point a draft at an arbitrary local file
- Control characters in recipient addresses are rejected at MIME build
  (CRLF header injection; error text never echoes the address per rule 7)
- Startup sweep for orphaned stem directories (15-minute age floor — the
  DLL copies attachments BEFORE writing JSON, so young dirs may be a write
  in flight)
- `nRecipCount`/`nFileCount` capped before element access; SEH
  `__try/__except` around conversion where the toolchain defines `__SEH__`
- `MaxFileSize` 18MB + cumulative `MaxTotalAttachmentSize` (base64 inflation
  math is at the constants in `gmail.go`)
- Auto mode caps consecutive permanent failures at 5, then backlog-skips the
  row; refused-connection/DNS errors now classify as "network" and never
  count toward the cap
- `GOMAPI_DEBUG_BROWSER_ARGS` only exists under `-tags gomapi_debug_browser`
- ldflags OAuth credentials win over environment variables

## Deployment

All-users install, `DraftHorse-setup.exe /S`, deployed via Intune. Detection rule:
`HKLM\SOFTWARE\Clients\Mail\go-mapi` → `DLLPath`. Sign the binaries first, then
the installer that wraps them, always timestamped (`signtool /tr`).

Firewall, if egress is filtered: `accounts.google.com`, `oauth2.googleapis.com`,
`www.googleapis.com`. `ENTERPRISE.md` now carries the correct allowlist (the
upstream version listed `gmail.googleapis.com`, which broke sign-in — the
fork rewrote that doc; keep the two lists in sync).

Archive the signed installer, its SHA-256, the commit hash, and the toolchain
versions with every tagged release. That archive is the independence.
