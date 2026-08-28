#Requires -RunAsAdministrator
#Requires -Version 5.1
<#
.SYNOPSIS
  Trust the Arrick's Propane code-signing certificate on this machine.
  Run once per fleet machine (ScreenConnect-friendly, non-interactive)
  BEFORE or alongside the DraftHorse install.

.DESCRIPTION
  Imports the PUBLIC certificate (DraftHorse-signing.cer — contains no
  private key) into two LocalMachine stores:

    Trusted Root Certification Authorities — a self-signed cert is its own
      chain, so Authenticode validation needs it here to succeed at all.
    Trusted Publishers — lets Windows treat the publisher as known
      (UAC shows "Arrick's Propane" instead of Unknown publisher).

  Idempotent: re-running is a no-op. Note this does NOT grant SmartScreen
  reputation — a downloaded installer may still need "More info → Run
  anyway" once; that is expected for self-signed certs.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\trust-signing-cert.ps1 -CertPath .\DraftHorse-signing.cer
#>
[CmdletBinding()]
param(
    [string]$CertPath = (Join-Path $PSScriptRoot 'DraftHorse-signing.cer')
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $CertPath)) {
    throw "Certificate not found: $CertPath"
}
$cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2($CertPath)
if ($cert.HasPrivateKey) {
    throw 'This file contains a PRIVATE key — never distribute the PFX to fleet machines. Use the .cer.'
}

foreach ($store in @('Root', 'TrustedPublisher')) {
    $existing = Get-ChildItem "Cert:\LocalMachine\$store" | Where-Object Thumbprint -eq $cert.Thumbprint
    if ($existing) {
        Write-Host "already trusted in LocalMachine\${store}: $($cert.Thumbprint)"
    } else {
        Import-Certificate -FilePath $CertPath -CertStoreLocation "Cert:\LocalMachine\$store" | Out-Null
        Write-Host "imported into LocalMachine\${store}: $($cert.Thumbprint)" -ForegroundColor Green
    }
}
Write-Host ("done: {0} (expires {1})" -f $cert.Subject, $cert.NotAfter) -ForegroundColor Cyan
