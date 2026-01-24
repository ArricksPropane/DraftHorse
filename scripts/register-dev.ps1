# register-dev.ps1
# Registers the go-mapi DLL from the build output directory
# Usage: .\register-dev.ps1 -BuildPath "C:\dev\go-mapi\build\bin"

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
    New-Item -Path $regPath -Force | Out-Null
    New-ItemProperty -Path $regPath -Name "(Default)" -Value "go-mapi" -PropertyType String -Force | Out-Null
    
    $dllRegPath = Join-Path $regPath "DLLPath"
    New-Item -Path $dllRegPath -Force | Out-Null
    New-ItemProperty -Path $dllRegPath -Name "(Default)" -Value $dllPath -PropertyType String -Force | Out-Null
    
    Write-Host "Successfully registered go-mapi DLL: $dllPath"
} catch {
    Write-Error "Failed to register DLL: $_"
    exit 1
}
