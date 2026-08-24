param(
    [string]$BaseUrl = "http://localhost:8080",
    [string]$QrCode = "",
    [int]$TotalRequests = 50,
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

Write-Host "Starting parallel single receipt load test..."
Write-Host "BaseUrl: $BaseUrl"
Write-Host "TotalRequests: $TotalRequests"

for ($i = 1; $i -le $TotalRequests; $i++) {
    $requestNumber = $i
    $userId = "single-user-$i"

    $job = Start-Job -ScriptBlock {
        param(
            [string]$HelpersPath,
            [string]$BaseUrl,
            [string]$QrCode,
            [string]$UserId,
            [int]$RequestNumber,
            [int]$TimeoutSec
        )

        . $HelpersPath

        $body = New-SingleReceiptBody -UserId $UserId -QrCode $QrCode

        Invoke-ReceiptRequest `
            -Url "$BaseUrl/receipts/parse" `
            -Body $body `
            -TimeoutSec $TimeoutSec `
            -RequestNumber $RequestNumber `
            -UserId $UserId `
            -Kind "single" `
            -ReceiptCount 1
    } -ArgumentList $helpersPath, $BaseUrl, $QrCode, $userId, $requestNumber, $TimeoutSec

    $jobs += $job

    Write-Host "Started single job $requestNumber for $userId"
}

Write-Host "Waiting for all single jobs..."

Wait-Job -Job $jobs | Out-Null

$results = @()

foreach ($job in $jobs) {
    $results += Receive-Job -Job $job
}

Remove-Job -Job $jobs

$results = $results | Sort-Object requestNumber

$summary = New-ReceiptTestSummary `
    -Results $results `
    -TestName "load-single-parallel" `
    -Mode "single" `
    -Execution "parallel" `
    -TotalRequests $TotalRequests `
    -TotalBatches 0 `
    -ReceiptsPerBatch 0

$outputPath = Save-ReceiptTestSummary `
    -Summary $summary `
    -ResultsDir $resultsDir `
    -FileName "load-single-parallel-results.json"

$summary | ConvertTo-Json -Depth 100

Write-Host "Saved result to: $outputPath"

if ($summary.failedCount -gt 0) {
    exit 1
}