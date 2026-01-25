# test-dll-registration.ps1 - Test DLL registration only (no UI)
# Writes results to $env:TEMP\go-mapi-registration-test.log

$ErrorActionPreference = "Stop"
$ProjectRoot = "C:\go-mapi"
$DllPath = "$ProjectRoot\src\interceptor\build\bin\go-mapi.dll"
$OutputFile = "C:\output\registration-test.log"

# Helper to log to both console and file
function Log($msg) {
    Write-Host $msg
    Add-Content -Path $OutputFile -Value $msg
}

# Clear previous log
"" | Set-Content $OutputFile

Log "=== DLL Registration Test ==="
Log "Timestamp: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
Log ""

# Step 1: Verify DLL exists
Log "[1/3] Checking DLL exists..."
if (-not (Test-Path $DllPath)) {
    Log "FAILED: DLL not found at $DllPath"
    exit 1
}
Log "OK: Found $DllPath"

# Step 2: Register DLL
Log ""
Log "[2/3] Registering DLL..."
try {
    & "$ProjectRoot\scripts\register-mapi-dll.ps1" -BuildPath "$ProjectRoot\src\interceptor\build\bin" 2>&1 | ForEach-Object { Log $_ }
    Log "OK: DLL registered"
} catch {
    Log "FAILED: Registration error: $_"
    exit 1
}

# Step 3: Verify registry
Log ""
Log "[3/3] Verifying registry..."
$regPath = "HKLM:\SOFTWARE\Clients\Mail\go-mapi\DLLPath"
$defaultMail = (Get-ItemProperty "HKLM:\SOFTWARE\Clients\Mail" -ErrorAction SilentlyContinue)."(Default)"
$dllRegValue = (Get-ItemProperty $regPath -ErrorAction SilentlyContinue)."(Default)"

Log "  Default mail client: $defaultMail"
Log "  DLL path in registry: $dllRegValue"

if ($defaultMail -ne "go-mapi") {
    Log "FAILED: Default mail client is '$defaultMail', expected 'go-mapi'"
    exit 1
}
Log "OK: Default mail client = go-mapi"

if ($dllRegValue -ne $DllPath) {
    Log "FAILED: DLL path in registry is '$dllRegValue', expected '$DllPath'"
    exit 1
}
Log "OK: DLL path registered correctly"

Log ""
Log "=== DLL REGISTRATION TEST PASSED ==="
exit 0
