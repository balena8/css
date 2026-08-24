param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$QrCode = "",
    [int]$TotalBatches = 3,
    [int]$ReceiptsPerBatch = 3,
    [int]$TimeoutSec = 900
)

$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "helpers\ReceiptTestHelpers.ps1")

$resultsDir = New-ReceiptTestResultsDirectory -RootPath $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($QrCode)) {
    $QrCode = Get-DefaultReceiptQrCode
}

$results = @()

Write-Host "Starting sequential batch receipt load test..."
Write-Host "BaseUrl: $BaseUrl"
Write-Host "TotalBatches: $TotalBatches"
Write-Host "ReceiptsPerBatch: $ReceiptsPerBatch"

for ($i = 1; $i -le $TotalBatches; $i++) {
    $userId = "batch-user-$i"
    $qrCodes = New-RepeatedQrCodes -QrCode $QrCode -Count $ReceiptsPerBatch
    $body = New-BatchReceiptBody -UserId $userId -QrCodes $qrCodes

    Write-Host "Sending batch $i/$TotalBatches for $userId..."

    $result = Invoke-ReceiptRequest `
        -Url "$BaseUrl/receipts/parse/batch" `
        -Body $body `
        -TimeoutSec $TimeoutSec `
        -RequestNumber $i `
        -UserId $userId `
        -Kind "batch" `
        -ReceiptCount $ReceiptsPerBatch

    $results += $result

    Write-Host "Batch $i finished with status=$($result.status) durationMs=$($result.durationMs)"
}

$summary = New-ReceiptTestSummary `
    -Results $results `
    -TestName "load-batch-sequential" `
    -Mode "batch" `
    -Execution "sequential" `
    -TotalRequests 0 `
    -TotalBatches $TotalBatches `
    -ReceiptsPerBatch $ReceiptsPerBatch

$outputPath = Save-ReceiptTestSummary `
    -Summary $summary `
    -ResultsDir $resultsDir `
    -FileName "load-batch-sequential-results.json"

$summary | ConvertTo-Json -Depth 100

Write-Host "Saved result to: $outputPath"

if ($summary.failedCount -gt 0) {
    exit 1
}