# test-drop-email.ps1
# Drop a test email JSON into %TEMP%\go-mapi\ to simulate a MAPI intercept.
# Usage:
#   .\scripts\test-drop-email.ps1                                          # Simple email
#   .\scripts\test-drop-email.ps1 -WithAttachment -AttachmentPath "C:\file.pdf"
#   .\scripts\test-drop-email.ps1 -Subject "Custom subject" -To "user@example.com"
#   .\scripts\test-drop-email.ps1 -CC "cc@example.com" -BCC "bcc@example.com"

param(
    [string]$Subject = "go-mapi test email",
    [string]$Body = "This is a test email dropped by test-drop-email.ps1.",
    [string]$To = "test@example.com",
    [string]$ToName = "Test User",
    [string]$CC = "",
    [string]$BCC = "",
    [switch]$WithAttachment,
    [string]$AttachmentPath = "",
    [switch]$Html
)

$ErrorActionPreference = "Stop"

$goMapiDir = Join-Path $env:TEMP "go-mapi"
if (-not (Test-Path $goMapiDir)) {
    New-Item -ItemType Directory -Path $goMapiDir -Force | Out-Null
}

# Build recipients
$toRecipients = @(@{ name = $ToName; address = $To })
$ccRecipients = @()
$bccRecipients = @()

if ($CC) {
    $ccRecipients = @(@{ name = ""; address = $CC })
}
if ($BCC) {
    $bccRecipients = @(@{ name = ""; address = $BCC })
}

# Build attachments
$attachments = @()
if ($WithAttachment) {
    if (-not $AttachmentPath) {
        # Create a small test file
        $testFile = Join-Path $env:TEMP "go-mapi-test-attachment.txt"
        "This is a test attachment created by test-drop-email.ps1." | Out-File -FilePath $testFile -Encoding UTF8
        $AttachmentPath = $testFile
        Write-Host "Created test attachment: $testFile"
    }

    if (-not (Test-Path $AttachmentPath)) {
        Write-Error "Attachment file not found: $AttachmentPath"
        exit 1
    }

    $fileInfo = Get-Item $AttachmentPath
    $attachments = @(@{
        filename = $fileInfo.Name
        path     = $fileInfo.FullName
        size     = $fileInfo.Length
    })
}

# Build the email JSON
$timestamp = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ss.fffZ")
$bodyFormat = if ($Html) { "html" } else { "plain" }

$email = @{
    version    = 1
    timestamp  = $timestamp
    subject    = $Subject
    body       = $Body
    bodyFormat = $bodyFormat
    recipients = @{
        to  = $toRecipients
        cc  = $ccRecipients
        bcc = $bccRecipients
    }
    attachments = $attachments
    originApp   = "test-drop-email.ps1"
}

$json = $email | ConvertTo-Json -Depth 5

# Generate unique filename
$ts = (Get-Date).ToString("yyyyMMdd_HHmmss")
$rand = -join ((0..5) | ForEach-Object { '{0:x}' -f (Get-Random -Maximum 16) })
$filename = "msg_${ts}_${rand}.json"
$filePath = Join-Path $goMapiDir $filename

# Write the file
$json | Out-File -FilePath $filePath -Encoding UTF8 -NoNewline
Write-Host ""
Write-Host "Dropped test email:" -ForegroundColor Green
Write-Host "  File:    $filePath"
Write-Host "  Subject: $Subject"
Write-Host "  To:      $To"
if ($CC)  { Write-Host "  CC:      $CC" }
if ($BCC) { Write-Host "  BCC:     $BCC" }
if ($attachments.Count -gt 0) {
    Write-Host "  Attach:  $($attachments[0].filename) ($($attachments[0].size) bytes)"
}
Write-Host ""
