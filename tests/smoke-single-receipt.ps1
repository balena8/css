param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$QrCode = "",
    [string]$UserId = "single-user-1",
    [int]$TimeoutSec = 900
)

$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "helpers\ReceiptTestHelpers.ps1")

$resultsDir = New-ReceiptTestResultsDirectory -RootPath $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($QrCode)) {
    $QrCode = Get-DefaultReceiptQrCode
}

$body = New-SingleReceiptBody -UserId $UserId -QrCode $QrCode

Write-Host "Sending single smoke receipt request..."
Write-Host "BaseUrl: $BaseUrl"
Write-Host "UserId: $UserId"

$result = Invoke-ReceiptRequest `
    -Url "$BaseUrl/receipts/parse" `
    -Body $body `
    -TimeoutSec $TimeoutSec `
    -RequestNumber 1 `
    -UserId $UserId `
    -Kind "single" `
    -ReceiptCount 1

$outputPath = Save-ReceiptTestResult `
    -Result $result `
    -ResultsDir $resultsDir `
    -FileName "smoke-single-result.json"

$result | ConvertTo-Json -Depth 100

Write-Host "Saved result to: $outputPath"

if ($result.status -ne "success") {
    exit 1
}