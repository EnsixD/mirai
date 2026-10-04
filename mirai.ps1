param([Parameter(Position=0)][string]$Command = 'start', [Parameter(ValueFromRemainingArguments=$true)][string[]]$CommandArgs)
$ErrorActionPreference = 'Stop'
$projectRoot = $PSScriptRoot
$localRoot = Join-Path $projectRoot '.local'
$pgBin = Join-Path $localRoot 'runtime\pgsql\bin'
$pgData = Join-Path $localRoot 'postgres-data'
$goExe = Join-Path $localRoot 'runtime\go\bin\go.exe'
$panelExe = Join-Path $projectRoot 'bin\mirai.exe'
$configPath = Join-Path $localRoot 'config.json'
New-Item -ItemType Directory -Force -Path $localRoot | Out-Null
if (!(Test-Path $configPath)) {
    $secretBytes = New-Object byte[] 24
    $random = [Security.Cryptography.RandomNumberGenerator]::Create()
    $random.GetBytes($secretBytes)
    $random.Dispose()
    @{ databasePassword = [Convert]::ToBase64String($secretBytes) } | ConvertTo-Json | Set-Content -Encoding UTF8 $configPath
}
$localConfig = Get-Content -Raw $configPath | ConvertFrom-Json
$env:MIRAI_DATA_DIR = Join-Path $localRoot 'panel-data'
$env:MIRAI_PANEL_LISTEN = '127.0.0.1:2053'
$env:MIRAI_DEV = '1'
$env:MIRAI_NODE_SOCKET = 'off'
$dbPasswordEscaped = [Uri]::EscapeDataString($localConfig.databasePassword)
$env:MIRAI_DATABASE_URL = "postgresql://mirai:${dbPasswordEscaped}@127.0.0.1:55432/mirai?sslmode=disable"
$env:PGPASSWORD = $localConfig.databasePassword
$env:PATH = "$pgBin;$env:PATH"

function Invoke-Checked([string]$Exe, [string[]]$Arguments) {
    & $Exe @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Exe failed with exit code $LASTEXITCODE" }
}
function Start-Database {
    if (!(Test-Path (Join-Path $pgData 'PG_VERSION'))) {
        $pwFile = Join-Path $localRoot 'postgres-init-password.txt'
        [IO.File]::WriteAllText($pwFile, $localConfig.databasePassword)
        try { Invoke-Checked (Join-Path $pgBin 'initdb.exe') @('-D', $pgData, '-U', 'mirai', '--encoding=UTF8', '--locale=C', '--auth=scram-sha-256', "--pwfile=$pwFile") }
        finally { Remove-Item -LiteralPath $pwFile -ErrorAction SilentlyContinue }
    }
    & (Join-Path $pgBin 'pg_ctl.exe') status -D $pgData *> $null
    if ($LASTEXITCODE -ne 0) {
        Invoke-Checked (Join-Path $pgBin 'pg_ctl.exe') @('start', '-D', $pgData, '-l', (Join-Path $localRoot 'postgres.log'), '-o', '-h 127.0.0.1 -p 55432', '-w')
    }
    $dbExists = & (Join-Path $pgBin 'psql.exe') -h 127.0.0.1 -p 55432 -U mirai -d postgres -Atc "SELECT 1 FROM pg_database WHERE datname='mirai'"
    if ($LASTEXITCODE -ne 0) { throw 'Cannot connect to the local database' }
    if ($dbExists -ne '1') { Invoke-Checked (Join-Path $pgBin 'createdb.exe') @('-h', '127.0.0.1', '-p', '55432', '-U', 'mirai', 'mirai') }
}
function Build-Panel {
    Push-Location (Join-Path $projectRoot 'web')
    try {
        Invoke-Checked 'node' @('node_modules/typescript/bin/tsc', '-b')
        Invoke-Checked 'node' @('node_modules/vite/bin/vite.js', 'build')
    } finally { Pop-Location }
    New-Item -ItemType Directory -Force -Path (Join-Path $projectRoot 'bin') | Out-Null
    Push-Location $projectRoot
    try { Invoke-Checked $goExe @('build', '-o', $panelExe, './cmd/mirai') }
    finally { Pop-Location }
}
function Stop-Panel {
    $pidFile = Join-Path $localRoot 'panel.pid'
    if (Test-Path $pidFile) {
        $panelProcess = Get-Process -Id ([int](Get-Content $pidFile)) -ErrorAction SilentlyContinue
        if ($panelProcess -and $panelProcess.Path -eq $panelExe) { Stop-Process -Id $panelProcess.Id }
        Remove-Item -LiteralPath $pidFile
    }
}

switch ($Command) {
    'build' { Build-Panel; exit }
    'db-start' { Start-Database; exit }
    'url' { Write-Output 'http://127.0.0.1:5173/login'; exit }
    'status' {
        foreach ($port in @(55432, 2053, 5173)) {
            $listener = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
            if ($listener) { Write-Output "Port ${port}: running (PID $($listener[0].OwningProcess))" }
            else { Write-Output "Port ${port}: stopped" }
        }
        exit
    }
    'stop' {
        Stop-Panel
        $webPidFile = Join-Path $localRoot 'web.pid'
        if (Test-Path $webPidFile) {
            $webProcess = Get-Process -Id ([int](Get-Content $webPidFile)) -ErrorAction SilentlyContinue
            if ($webProcess -and $webProcess.ProcessName -eq 'node') { Stop-Process -Id $webProcess.Id }
            Remove-Item -LiteralPath $webPidFile
        }
        Invoke-Checked (Join-Path $pgBin 'pg_ctl.exe') @('stop', '-D', $pgData, '-m', 'fast', '-w')
        exit
    }
    'restart' { Stop-Panel; $Command = 'start' }
}
Start-Database
if (!(Test-Path $panelExe)) { Build-Panel }
if ($Command -eq 'start') {
    $initialized = Join-Path $localRoot 'initialized'
    if (!(Test-Path $initialized)) {
        $loginOutput = & $panelExe admin bootstrap --public-host 127.0.0.1 --port 2053 --admin-path dev-admin-path-0000 --sub-path mirai-local-sub --username admin --lang ru
        if ($LASTEXITCODE -ne 0) { throw 'Administrator bootstrap failed' }
        $loginOutput = $loginOutput -replace 'https://127.0.0.1:2053/dev-admin-path-0000/', 'http://127.0.0.1:5173/login'
        $loginOutput | Set-Content -Encoding UTF8 (Join-Path $localRoot 'login.txt')
        $loginOutput | Write-Output
        New-Item -ItemType File -Path $initialized | Out-Null
    }
    if (!(Get-NetTCPConnection -LocalPort 2053 -State Listen -ErrorAction SilentlyContinue)) {
        $panelProcess = Start-Process -FilePath $panelExe -ArgumentList 'serve' -WorkingDirectory $projectRoot -WindowStyle Hidden -RedirectStandardOutput (Join-Path $localRoot 'panel.stdout.log') -RedirectStandardError (Join-Path $localRoot 'panel.stderr.log') -PassThru
        $panelProcess.Id | Set-Content (Join-Path $localRoot 'panel.pid')
    }
    if (!(Get-NetTCPConnection -LocalPort 5173 -State Listen -ErrorAction SilentlyContinue)) {
        $webProcess = Start-Process -FilePath (Get-Command node.exe).Source -ArgumentList 'node_modules/vite/bin/vite.js --host 127.0.0.1 --port 5173 --strictPort' -WorkingDirectory (Join-Path $projectRoot 'web') -WindowStyle Hidden -RedirectStandardOutput (Join-Path $localRoot 'web.stdout.log') -RedirectStandardError (Join-Path $localRoot 'web.stderr.log') -PassThru
        $webProcess.Id | Set-Content (Join-Path $localRoot 'web.pid')
    }
    Write-Output 'Mirai: http://127.0.0.1:5173/login'
    Write-Output 'Login details: .local/login.txt'
} else {
    if ($Command -eq 'backup' -or $Command -eq 'restore') { $CommandArgs = @($Command) + $CommandArgs; $Command = 'database' }
    if ($Command -in @('cert', 'node', 'targets', 'inbound')) { $CommandArgs = @($Command) + $CommandArgs; $Command = 'admin' }
    Invoke-Checked $panelExe (@($Command) + $CommandArgs)
}
