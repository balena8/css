param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$QrCode = "",
    [string]$UserId = "batch-user-1",
    [int]$ReceiptsPerBatch = 3,
    [int]$TimeoutSec = 900
)

$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "helpers\ReceiptTestHelpers.ps1")

$resultsDir = New-ReceiptTestResultsDirectory -RootPath $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($QrCode)) {
    $QrCode = Get-DefaultReceiptQrCode
}

$qrCodes = New-RepeatedQrCodes -QrCode $QrCode -Count $ReceiptsPerBatch
$body = New-BatchReceiptBody -UserId $UserId -QrCodes $qrCodes

Write-Host "Sending batch smoke receipt request..."
Write-Host "BaseUrl: $BaseUrl"
Write-Host "UserId: $UserId"
Write-Host "ReceiptsPerBatch: $ReceiptsPerBatch"

$result = Invoke-ReceiptRequest `
    -Url "$BaseUrl/receipts/parse/batch" `
    -Body $body `
    -TimeoutSec $TimeoutSec `
    -RequestNumber 1 `
    -UserId $UserId `
    -Kind "batch" `
    -ReceiptCount $ReceiptsPerBatch

$outputPath = Save-ReceiptTestResult `
    -Result $result `
    -ResultsDir $resultsDir `
    -FileName "smoke-batch-result.json"

$result | ConvertTo-Json -Depth 100

Write-Host "Saved result to: $outputPath"

if ($result.status -ne "success") {
    exit 1
}