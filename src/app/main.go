package main

import (
	"embed"
	"os"

	"github.com/pkg/browser"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

var Version = "0.0.0-dev" // overridden via -ldflags "-X main.Version=..."

func main() {
	// ARRICKS-06: the silent-update dispatch that used to live here has been
	// removed along with updates_silent.go. Upstream registered a Scheduled
	// Task running as SYSTEM (S-1-5-18, HighestAvailable) that re-invoked this
	// binary with --update-check-silent; that routine downloaded binaries from
	// a hardcoded GitHub repo and MoveFileEx'd them over %ProgramFiles%.
	//
	// Its only integrity control was a SHA-256 manifest fetched from the same
	// release URL as the binaries, which detects corruption but not a
	// compromised or malicious release. No Authenticode verification was
	// performed at any point. Upstream's own planning notes flag this as a
	// requirement that was never implemented.
	//
	// The flag is retained as an explicit no-op so that a Scheduled Task left
	// behind by a previously-installed upstream build cannot do anything if it
	// ever fires against our binary. Updates here are deliberate: IT builds,
	// signs, and deploys a new version through Intune.
	if len(os.Args) >= 2 && os.Args[1] == "--update-check-silent" {
		os.Exit(0)
	}

	// ARRICKS-09: mailto protocol handler. Registered by the installer as
	// `go-mapi.exe --mailto "%1"` (go-mapi.mailto ProgID). Opens Gmail web
	// compose prefilled from the URL and exits — deliberately BEFORE the
	// single-instance gate so a mailto click neither raises the tray app's
	// window nor boots a second full instance. See mailto.go.
	if len(os.Args) >= 2 && os.Args[1] == "--mailto" {
		arg := ""
		if len(os.Args) >= 3 {
			arg = os.Args[2]
		}
		os.Exit(runMailtoHandler(arg))
	}

	raised, siErr := acquireSingleInstance()
	if siErr != nil {
		logError("single-instance: %v", siErr)
		// Fail-open: proceed to run anyway so the user doesn't lose access.
	}
	if raised {
		// Another instance owns the mutex; we signaled its named event. Exit now.
		// Use os.Exit to be explicit: no Wails init, no defers needing to run, fastest path out.
		logInfo("second instance detected — signalled first instance, exiting")
		os.Exit(0)
	}
	defer releaseSingleInstance()

	// D-08: Fail fast if the WebView2 Evergreen runtime is missing. Running
	// without it, the Wails window would either show blank or the process
	// would crash inside Wails's WebView2 init. Show a native MessageBox
	// (the only UI primitive we can use — Wails itself is what's broken),
	// point the user at Microsoft's download page, and exit cleanly.
	// Guard is skipped under `bindings` (wailsbindings.exe) like checkOAuthCredentials.
	if err := checkWebView2(); err != nil {
		logError("FATAL: %s", err.Error())
		showWebView2MissingDialog()
		_ = browser.OpenURL("https://developer.microsoft.com/en-us/microsoft-edge/webview2/")
		os.Exit(1)
	}

	// D-10: Fail fast if OAuth credentials were not injected. A release build
	// with empty client_id silently cannot sign anyone in — louder now is kinder.
	// Guard is skipped under the `bindings` build tag (wailsbindings.exe) so that
	// Wails can generate TypeScript bindings without needing real credentials at
	// dev time. In production and wails dev, the guard is always active.
	// The check itself returns an error (testable); main owns the os.Exit(1).
	if err := checkOAuthCredentials(); err != nil {
		logError("FATAL: %s", err.Error())
		os.Exit(1)
	}

	app := NewApp()

	// Note: HideWindowOnClose is intentionally NOT set. With it true, Wails routes the X
	// button straight to f.WindowHide() without invoking OnBeforeClose — that bypasses our
	// visibility tracking (Bug A) and the intentionalQuit gate (Bug B). Instead we let
	// the X button fire OnBeforeClose, and our beforeClose hides the window AND updates
	// visibility (return true = prevent the actual close).
	err := wails.Run(&options.App{
		Title:         "go-mapi",
		Width:         480,
		Height:        600,
		MinWidth:      360,
		MinHeight:     400,
		Assets:        assets,
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		Bind:          []interface{}{app},
		StartHidden:   true,
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
