[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ServerHost,

    [Parameter(Mandatory = $true)]
    [string]$ServerUser,

    [int]$ServerSshPort = 22,
    [string]$RepoDir = "/workspace/czk/Personal/EchoChat",
    [string]$RemotePython = "python3",
    [string]$ServiceName = "echochat",
    [int]$ServicePort = 8081,
    [string]$WsUrl = "ws://127.0.0.1:8081",
    [string]$WsPath = "/bench/wss",
    [string]$MetricsUrl = "http://127.0.0.1:8081/metrics",
    [string]$PprofUrl = "http://127.0.0.1:8081/debug/pprof/goroutine?debug=1",
    [int[]]$TargetVusList = @(100, 300, 500, 800, 1000),
    [int]$HoldSeconds = 180,
    [int]$PingIntervalSeconds = 30,
    [double]$CollectorInterval = 2,
    [string]$Prefix = "WSKAF",
    [int]$UserPadding = 200,
    [string]$Password = "123456",
    [long]$TelephoneStart = 17630000000,
    [string]$SummaryTrendStats = "avg,min,med,max,p(90),p(95),p(99)",
    [switch]$InsecureSkipTlsVerify
)

$ErrorActionPreference = "Stop"

function Assert-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "required command not found: $Name"
    }
}

function Convert-ToBashSingleQuoted {
    param([string]$Text)
    return "'" + ($Text -replace "'", "'""'""'") + "'"
}

function Invoke-RemoteBash {
    param([string]$Script)
    $quoted = Convert-ToBashSingleQuoted $Script
    & ssh -p $ServerSshPort "$ServerUser@$ServerHost" "bash -lc $quoted"
    if ($LASTEXITCODE -ne 0) {
        throw "remote command failed"
    }
}

function Copy-FromRemote {
    param(
        [string]$RemotePath,
        [string]$LocalPath,
        [switch]$Recursive
    )
    $args = @("-P", $ServerSshPort.ToString())
    if ($Recursive) {
        $args += "-r"
    }
    $args += @("${ServerUser}@${ServerHost}:${RemotePath}", $LocalPath)
    & scp @args
    if ($LASTEXITCODE -ne 0) {
        throw "scp failed for remote path: $RemotePath"
    }
}

function Stop-RemoteCollector {
    param([string]$RemoteStepDir)
    $script = @"
if [ -f '$RemoteStepDir/collector.pid' ]; then
  pid=`$(cat '$RemoteStepDir/collector.pid')
  if [ -n "`$pid" ] && kill -0 "`$pid" 2>/dev/null; then
    kill -TERM "`$pid" 2>/dev/null || true
    wait "`$pid" 2>/dev/null || true
  fi
fi
"@
    Invoke-RemoteBash $script
}

function Get-K6MetricValue {
    param(
        $Summary,
        [string]$MetricName,
        [string]$FieldName
    )
    $metric = $Summary.metrics.PSObject.Properties[$MetricName]
    if (-not $metric) {
        return 0
    }
    $metricValue = $metric.Value
    $direct = $metricValue.PSObject.Properties[$FieldName]
    if ($direct) {
        return [double]$direct.Value
    }
    if ($metricValue.values) {
        $fromValues = $metricValue.values.PSObject.Properties[$FieldName]
        if ($fromValues) {
            return [double]$fromValues.Value
        }
    }
    return 0
}

function Get-CsvMaximum {
    param(
        [string]$CsvPath,
        [string]$Column
    )
    if (-not (Test-Path $CsvPath)) {
        return 0
    }
    $rows = Import-Csv $CsvPath
    if (-not $rows -or $rows.Count -eq 0) {
        return 0
    }
    $values = foreach ($row in $rows) {
        $property = $row.PSObject.Properties[$Column]
        if ($property -and $property.Value -ne "") {
            [double]$property.Value
        }
    }
    if (-not $values) {
        return 0
    }
    return ($values | Measure-Object -Maximum).Maximum
}

Assert-Command "ssh"
Assert-Command "scp"
Assert-Command "k6"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..")).Path
$K6Script = Join-Path $RepoRoot "docs\t_K6\scripts\ws_online_tokens.js"
if (-not (Test-Path $K6Script)) {
    throw "k6 script not found: $K6Script"
}

$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$localRunDir = Join-Path $RepoRoot "docs\t_K6\records\windows_ws_online_kafka_$timestamp"
$null = New-Item -ItemType Directory -Path $localRunDir -Force
$remoteRunDir = "$RepoDir/docs/t_K6/records/windows_remote_ws_online_kafka_$timestamp"
$remoteTokenFile = "$remoteRunDir/ws_tokens.json"
$localTokenFile = Join-Path $localRunDir "ws_tokens.json"
$maxVus = ($TargetVusList | Measure-Object -Maximum).Maximum
$seedUserCount = $maxVus + $UserPadding

Write-Host "Preparing remote benchmark users and token file..."
$prepareScript = @"
set -euo pipefail
systemctl is-active '$ServiceName' >/dev/null
grep -q 'messageMode = "kafka"' '$RepoDir/configs/config_local.toml'
grep -q 'enableBenchmarkRoutes = true' '$RepoDir/configs/config_local.toml'
grep -q 'enableMetrics = true' '$RepoDir/configs/config_local.toml'
grep -q 'enablePprof = true' '$RepoDir/configs/config_local.toml'
mkdir -p '$remoteRunDir'
cd '$RepoDir'
go run ./cmd/echo_chat_seed \
  -prefix '$Prefix' \
  -reset-prefix \
  -user-count $seedUserCount \
  -admin-count 1 \
  -group-count 0 \
  -group-size 1 \
  -friend-span 1 \
  -pair-messages 0 \
  -group-messages 0 \
  -apply-count 0 \
  -password '$Password' \
  -telephone-start $TelephoneStart > '$remoteRunDir/seed_summary.json'
go run ./cmd/echo_chat_ws_tokens -prefix '$Prefix' -count $maxVus -output '$remoteTokenFile'
"@
Invoke-RemoteBash $prepareScript

Write-Host "Downloading token file..."
Copy-FromRemote -RemotePath $remoteTokenFile -LocalPath $localRunDir

$stageRows = New-Object System.Collections.Generic.List[object]

foreach ($target in $TargetVusList) {
    $stepName = "step_{0:D5}" -f $target
    $localStepDir = Join-Path $localRunDir $stepName
    $localServerDir = Join-Path $localStepDir "server"
    $summaryPath = Join-Path $localStepDir "summary.json"
    $stdoutPath = Join-Path $localStepDir "stdout.txt"
    $remoteStepDir = "$remoteRunDir/$stepName"

    $null = New-Item -ItemType Directory -Path $localStepDir -Force
    $null = New-Item -ItemType Directory -Path $localServerDir -Force

    Write-Host "[$stepName] starting remote collector..."
    $startCollectorScript = @"
set -euo pipefail
mkdir -p '$remoteStepDir'
nohup '$RemotePython' '$RepoDir/docs/t_K6/scripts/collect_server_metrics.py' \
  --output-dir '$remoteStepDir' \
  --service-name '$ServiceName' \
  --service-port $ServicePort \
  --metrics-url '$MetricsUrl' \
  --pprof-url '$PprofUrl' \
  --interval $CollectorInterval \
  --label '$stepName' > '$remoteStepDir/collector.stdout' 2>&1 &
echo `$! > '$remoteStepDir/collector.pid'
"@
    Invoke-RemoteBash $startCollectorScript
    Start-Sleep -Seconds 2

    Write-Host "[$stepName] running local k6..."
    $env:WS_URL = $WsUrl
    $env:WS_PATH = $WsPath
    $env:TOKEN_FILE = $localTokenFile
    $env:HOLD_SECONDS = $HoldSeconds.ToString()
    $env:PING_INTERVAL_SECONDS = $PingIntervalSeconds.ToString()

    $k6Args = @(
        "run",
        $K6Script,
        "--address", "127.0.0.1:0",
        "-u", $target.ToString(),
        "-i", $target.ToString(),
        "--summary-export", $summaryPath,
        "--summary-trend-stats", $SummaryTrendStats
    )
    if ($InsecureSkipTlsVerify) {
        $k6Args += "--insecure-skip-tls-verify"
    }

    try {
        & k6 @k6Args 2>&1 | Tee-Object -FilePath $stdoutPath
        $k6ExitCode = $LASTEXITCODE
    }
    finally {
        Write-Host "[$stepName] stopping remote collector..."
        Stop-RemoteCollector -RemoteStepDir $remoteStepDir
    }

    Write-Host "[$stepName] downloading server artifacts..."
    Copy-FromRemote -RemotePath "$remoteStepDir/." -LocalPath $localServerDir -Recursive
    Set-Content -Path (Join-Path $localStepDir "exit_code.txt") -Value $k6ExitCode

    if (Test-Path $summaryPath) {
        $summary = Get-Content -Path $summaryPath -Raw | ConvertFrom-Json
    } else {
        $summary = [PSCustomObject]@{
            metrics = [PSCustomObject]@{}
        }
    }
    $stageRows.Add([PSCustomObject]@{
        step_name = $stepName
        target_vus = $target
        k6_exit_code = $k6ExitCode
        ws_upgrade_success_rate = Get-K6MetricValue -Summary $summary -MetricName "ws_upgrade_success_rate" -FieldName "value"
        ws_early_disconnect_rate = Get-K6MetricValue -Summary $summary -MetricName "ws_early_disconnect_rate" -FieldName "value"
        ws_early_disconnect_count = Get-K6MetricValue -Summary $summary -MetricName "ws_early_disconnect_count" -FieldName "count"
        ws_error_rate = Get-K6MetricValue -Summary $summary -MetricName "ws_error_rate" -FieldName "value"
        ws_connecting_p95_ms = Get-K6MetricValue -Summary $summary -MetricName "ws_connecting" -FieldName "p(95)"
        ws_connecting_p99_ms = Get-K6MetricValue -Summary $summary -MetricName "ws_connecting" -FieldName "p(99)"
        ws_session_p95_ms = Get-K6MetricValue -Summary $summary -MetricName "ws_session_duration_ms" -FieldName "p(95)"
        server_online_peak = Get-CsvMaximum -CsvPath (Join-Path $localServerDir "samples.csv") -Column "online_connections"
        server_goroutines_peak = Get-CsvMaximum -CsvPath (Join-Path $localServerDir "samples.csv") -Column "go_goroutines"
        server_open_fds_peak = Get-CsvMaximum -CsvPath (Join-Path $localServerDir "samples.csv") -Column "process_open_fds"
        server_rss_peak_mb = [math]::Round((Get-CsvMaximum -CsvPath (Join-Path $localServerDir "samples.csv") -Column "rss_kb") / 1024, 3)
        server_cpu_peak_percent = Get-CsvMaximum -CsvPath (Join-Path $localServerDir "samples.csv") -Column "cpu_percent_inst"
    }) | Out-Null

    if ($k6ExitCode -ne 0) {
        Write-Warning "[$stepName] k6 exited with code $k6ExitCode, stopping suite."
        break
    }
}

$stageSummaryPath = Join-Path $localRunDir "stage_summary.csv"
$stageRows | Export-Csv -Path $stageSummaryPath -NoTypeInformation -Encoding utf8

$summaryLines = @(
    "# Windows WS Online Kafka Benchmark",
    "",
    "- Run directory: `$localRunDir`",
    "- Remote run directory: `$remoteRunDir`",
    "- Server target: `$ServerUser@$ServerHost`",
    "- WebSocket url: `$WsUrl$WsPath`",
    "- Hold seconds: `$HoldSeconds`",
    "- Target VUs: `" + ($TargetVusList -join ", ") + "`",
    "- Token prefix: `$Prefix`",
    "",
    "## Stage Summary",
    ""
)

foreach ($row in $stageRows) {
    $summaryLines += "- $($row.step_name): vus=$($row.target_vus), k6_exit=$($row.k6_exit_code), upgrade_rate=$([math]::Round([double]$row.ws_upgrade_success_rate * 100, 2))%, early_disconnect_rate=$([math]::Round([double]$row.ws_early_disconnect_rate * 100, 2))%, online_peak=$($row.server_online_peak), goroutines_peak=$($row.server_goroutines_peak), open_fds_peak=$($row.server_open_fds_peak)"
}
$summaryLines += ""
$summaryLines += "- Stage CSV: `$stageSummaryPath`"

Set-Content -Path (Join-Path $localRunDir "summary.md") -Value ($summaryLines -join [Environment]::NewLine)

Write-Host ""
Write-Host "Benchmark completed."
Write-Host "Local result directory: $localRunDir"
Write-Host "Stage summary: $stageSummaryPath"
