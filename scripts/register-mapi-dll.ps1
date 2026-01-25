# register-mapi-dll.ps1
# Registers the go-mapi MAPI DLL as the default Windows mail client
# Usage: .\register-mapi-dll.ps1 -BuildPath "C:\dev\go-mapi\src\interceptor\build\bin"

param(
    [Parameter(Mandatory=$true)]
    [string]$BuildPath
)

# Validate path exists
if (-not (Test-Path $BuildPath)) {
    Write-Error "Build path does not exist: $BuildPath"
    exit 1
}

# Check for DLL
$dllPath = Join-Path $BuildPath "go-mapi.dll"
if (-not (Test-Path $dllPath)) {
    Write-Error "DLL not found at: $dllPath"
    exit 1
}

# Ensure we're running as admin
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Error "This script must be run as Administrator"
    exit 1
}

# Register in registry
$regPath = "HKLM:\SOFTWARE\Clients\Mail\go-mapi"

try {
    # Register go-mapi as a mail client
    New-Item -Path $regPath -Force | Out-Null
    New-ItemProperty -Path $regPath -Name "(Default)" -Value "go-mapi" -PropertyType String -Force | Out-Null

    $dllRegPath = Join-Path $regPath "DLLPath"
    New-Item -Path $dllRegPath -Force | Out-Null
    New-ItemProperty -Path $dllRegPath -Name "(Default)" -Value $dllPath -PropertyType String -Force | Out-Null

    # Set go-mapi as the DEFAULT mail client (required for "Send to -> Mail recipient")
    Set-ItemProperty -Path "HKLM:\SOFTWARE\Clients\Mail" -Name "(Default)" -Value "go-mapi" -Force

    Write-Host "Successfully registered go-mapi DLL: $dllPath"
    Write-Host "Set go-mapi as default mail client"
} catch {
    Write-Error "Failed to register DLL: $_"
    exit 1
}
