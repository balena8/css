param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$QrCode = "",
    [int]$TotalBatches = 10,
    [int]$ReceiptsPerBatch = 3,
    [int]$TimeoutSec = 900
)

$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "helpers\ReceiptTestHelpers.ps1")

$helpersPath = Join-Path $PSScriptRoot "helpers\ReceiptTestHelpers.ps1"
$resultsDir = New-ReceiptTestResultsDirectory -RootPath $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($QrCode)) {
    $QrCode = Get-DefaultReceiptQrCode
}

$jobs = @()

Write-Host "Starting parallel batch receipt load test..."
Write-Host "BaseUrl: $BaseUrl"
Write-Host "TotalBatches: $TotalBatches"
Write-Host "ReceiptsPerBatch: $ReceiptsPerBatch"
Write-Host "TotalReceipts: $($TotalBatches * $ReceiptsPerBatch)"

for ($i = 1; $i -le $TotalBatches; $i++) {
    $batchNumber = $i
    $userId = "batch-user-$i"

    $job = Start-Job -ScriptBlock {
        param(
            [string]$HelpersPath,
            [string]$BaseUrl,
            [string]$QrCode,
            [string]$UserId,
            [int]$BatchNumber,
            [int]$ReceiptsPerBatch,
            [int]$TimeoutSec
        )

        . $HelpersPath

        $qrCodes = New-RepeatedQrCodes -QrCode $QrCode -Count $ReceiptsPerBatch
        $body = New-BatchReceiptBody -UserId $UserId -QrCodes $qrCodes

        Invoke-ReceiptRequest `
            -Url "$BaseUrl/receipts/parse/batch" `
            -Body $body `
            -TimeoutSec $TimeoutSec `
            -RequestNumber $BatchNumber `
            -UserId $UserId `
            -Kind "batch" `
            -ReceiptCount $ReceiptsPerBatch
    } -ArgumentList $helpersPath, $BaseUrl, $QrCode, $userId, $batchNumber, $ReceiptsPerBatch, $TimeoutSec

    $jobs += $job

    Write-Host "Started batch job $batchNumber for $userId"
}

Write-Host "Waiting for all batch jobs..."

Wait-Job -Job $jobs | Out-Null

$results = @()

foreach ($job in $jobs) {
    $results += Receive-Job -Job $job
}

Remove-Job -Job $jobs

$results = $results | Sort-Object requestNumber

$summary = New-ReceiptTestSummary `
    -Results $results `
    -TestName "load-batch-parallel" `
    -Mode "batch" `
    -Execution "parallel" `
    -TotalRequests 0 `
    -TotalBatches $TotalBatches `
    -ReceiptsPerBatch $ReceiptsPerBatch

$outputPath = Save-ReceiptTestSummary `
    -Summary $summary `
    -ResultsDir $resultsDir `
    -FileName "load-batch-parallel-results.json"

$summary | ConvertTo-Json -Depth 100

Write-Host "Saved result to: $outputPath"

if ($summary.failedCount -gt 0) {
    exit 1
}