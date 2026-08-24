param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$QrCode = "",
    [int]$TotalRequests = 5,
    [int]$TimeoutSec = 900
)

$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "helpers\ReceiptTestHelpers.ps1")

$resultsDir = New-ReceiptTestResultsDirectory -RootPath $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($QrCode)) {
    $QrCode = Get-DefaultReceiptQrCode
}

$results = @()

Write-Host "Starting sequential single receipt load test..."
Write-Host "BaseUrl: $BaseUrl"
Write-Host "TotalRequests: $TotalRequests"

for ($i = 1; $i -le $TotalRequests; $i++) {
    $userId = "single-user-$i"
    $body = New-SingleReceiptBody -UserId $userId -QrCode $QrCode

    Write-Host "Sending request $i/$TotalRequests for $userId..."

    $result = Invoke-ReceiptRequest `
        -Url "$BaseUrl/receipts/parse" `
        -Body $body `
        -TimeoutSec $TimeoutSec `
        -RequestNumber $i `
        -UserId $userId `
        -Kind "single" `
        -ReceiptCount 1

    $results += $result

    Write-Host "Request $i finished with status=$($result.status) durationMs=$($result.durationMs)"
}

$summary = New-ReceiptTestSummary `
    -Results $results `
    -TestName "load-single-sequential" `
    -Mode "single" `
    -Execution "sequential" `
    -TotalRequests $TotalRequests `
    -TotalBatches 0 `
    -ReceiptsPerBatch 0

$outputPath = Save-ReceiptTestSummary `
    -Summary $summary `
    -ResultsDir $resultsDir `
    -FileName "load-single-sequential-results.json"

$summary | ConvertTo-Json -Depth 100

Write-Host "Saved result to: $outputPath"

if ($summary.failedCount -gt 0) {
    exit 1
}