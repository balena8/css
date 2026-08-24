function New-ReceiptTestResultsDirectory {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RootPath
    )

    $resultsDir = Join-Path $RootPath "results"

    if (!(Test-Path $resultsDir)) {
        New-Item -ItemType Directory -Path $resultsDir | Out-Null
    }

    return $resultsDir
}

function Get-DefaultReceiptQrCode {
    return "https://cabinet.tax.gov.ua/cashregs/check?date=20260429&time=222006&id=696582&sm=437.40&fn=3000909908"
}

function New-RepeatedQrCodes {
    param(
        [Parameter(Mandatory = $true)]
        [string]$QrCode,

        [Parameter(Mandatory = $true)]
        [int]$Count
    )

    $qrCodes = @()

    for ($i = 1; $i -le $Count; $i++) {
        $qrCodes += $QrCode
    }

    return $qrCodes
}

function New-SingleReceiptBody {
    param(
        [Parameter(Mandatory = $true)]
        [string]$UserId,

        [Parameter(Mandatory = $true)]
        [string]$QrCode
    )

    return @{
        user_id = $UserId
        qr_code = $QrCode
    } | ConvertTo-Json -Depth 20
}

function New-BatchReceiptBody {
    param(
        [Parameter(Mandatory = $true)]
        [string]$UserId,

        [Parameter(Mandatory = $true)]
        [string[]]$QrCodes
    )

    return @{
        user_id = $UserId
        qr_codes = $QrCodes
    } | ConvertTo-Json -Depth 20
}

function Invoke-ReceiptRequest {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Url,

        [Parameter(Mandatory = $true)]
        [string]$Body,

        [Parameter(Mandatory = $true)]
        [int]$TimeoutSec,

        [Parameter(Mandatory = $true)]
        [int]$RequestNumber,

        [Parameter(Mandatory = $true)]
        [string]$UserId,

        [Parameter(Mandatory = $true)]
        [string]$Kind,

        [Parameter(Mandatory = $true)]
        [int]$ReceiptCount
    )

    $start = Get-Date

    try {
        $response = Invoke-RestMethod `
            -Method Post `
            -Uri $Url `
            -ContentType "application/json" `
            -Body $Body `
            -TimeoutSec $TimeoutSec

        $end = Get-Date
        $durationMs = [Math]::Round(($end - $start).TotalMilliseconds, 2)

        return [PSCustomObject]@{
            requestNumber = $RequestNumber
            userId = $UserId
            kind = $Kind
            receiptCount = $ReceiptCount
            status = "success"
            durationMs = $durationMs
            response = $response
            error = $null
        }
    }
    catch {
        $end = Get-Date
        $durationMs = [Math]::Round(($end - $start).TotalMilliseconds, 2)

        return [PSCustomObject]@{
            requestNumber = $RequestNumber
            userId = $UserId
            kind = $Kind
            receiptCount = $ReceiptCount
            status = "failed"
            durationMs = $durationMs
            response = $null
            error = $_.Exception.Message
        }
    }
}

function New-ReceiptTestSummary {
    param(
        [Parameter(Mandatory = $true)]
        [object[]]$Results,

        [Parameter(Mandatory = $true)]
        [string]$TestName,

        [Parameter(Mandatory = $true)]
        [string]$Mode,

        [Parameter(Mandatory = $true)]
        [string]$Execution,

        [Parameter(Mandatory = $true)]
        [int]$TotalRequests,

        [Parameter(Mandatory = $true)]
        [int]$TotalBatches,

        [Parameter(Mandatory = $true)]
        [int]$ReceiptsPerBatch
    )

    $resultsArray = @($Results)

    $successCount = ($resultsArray | Where-Object { $_.status -eq "success" }).Count
    $failedCount = ($resultsArray | Where-Object { $_.status -eq "failed" }).Count

    $averageDurationMs = 0
    $minDurationMs = 0
    $maxDurationMs = 0

    if ($resultsArray.Count -gt 0) {
        $durationStats = $resultsArray | Measure-Object -Property durationMs -Average -Minimum -Maximum

        $averageDurationMs = [Math]::Round($durationStats.Average, 2)
        $minDurationMs = [Math]::Round($durationStats.Minimum, 2)
        $maxDurationMs = [Math]::Round($durationStats.Maximum, 2)
    }

    $totalReceipts = 0

    if ($Mode -eq "single") {
        $totalReceipts = $TotalRequests
    }
    else {
        $totalReceipts = $TotalBatches * $ReceiptsPerBatch
    }

    return [PSCustomObject]@{
        testName = $TestName
        mode = $Mode
        execution = $Execution
        totalRequests = $TotalRequests
        totalBatches = $TotalBatches
        receiptsPerBatch = $ReceiptsPerBatch
        totalReceipts = $totalReceipts
        successCount = $successCount
        failedCount = $failedCount
        averageDurationMs = $averageDurationMs
        minDurationMs = $minDurationMs
        maxDurationMs = $maxDurationMs
        generatedAt = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
        results = $resultsArray
    }
}

function Save-ReceiptTestResult {
    param(
        [Parameter(Mandatory = $true)]
        [object]$Result,

        [Parameter(Mandatory = $true)]
        [string]$ResultsDir,

        [Parameter(Mandatory = $true)]
        [string]$FileName
    )

    $outputPath = Join-Path $ResultsDir $FileName

    $Result |
        ConvertTo-Json -Depth 100 |
        Out-File $outputPath -Encoding UTF8

    return $outputPath
}

function Save-ReceiptTestSummary {
    param(
        [Parameter(Mandatory = $true)]
        [object]$Summary,

        [Parameter(Mandatory = $true)]
        [string]$ResultsDir,

        [Parameter(Mandatory = $true)]
        [string]$FileName
    )

    $outputPath = Join-Path $ResultsDir $FileName

    $Summary |
        ConvertTo-Json -Depth 100 |
        Out-File $outputPath -Encoding UTF8

    return $outputPath
}