$BaseUrl = "http://localhost:8080"
$IntervalMilliseconds = 200

while ($true) {
    Clear-Host

    Write-Host "Queue monitor: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss.fff')"
    Write-Host ""

    try {
        $queues = Invoke-RestMethod `
            -Method Get `
            -Uri "$BaseUrl/receipts/queues"

        $queues | ConvertTo-Json -Depth 20
    }
    catch {
        Write-Host "Failed to fetch /receipts/queues"
        Write-Host $_.Exception.Message
    }

    Start-Sleep -Milliseconds $IntervalMilliseconds
}