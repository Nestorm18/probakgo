#!/usr/bin/env pwsh
# Deploy probakgo server + clients to test environment
param(
    [switch]$Server,
    [switch]$Clients,
    [switch]$All,
    [switch]$Update,  # Run 'update' subcommand on all machines (pulls from GitHub)
    [switch]$Db       # Copy the SQLite database from the server to the local directory
)

# ── Hosts ─────────────────────────────────────────────────────────────────────
$SERVER_HOST  = "root@192.168.10.222"
$CLIENT1_HOST = "root@192.168.10.230"   # PVE - also runs vzdump
$CLIENT2_HOST = "root@192.168.10.248"   # PBS - also runs the client report

# ── SSH password ──────────────────────────────────────────────────────────────
$pass = Read-Host "SSH password" -AsSecureString
$SSH_PASS = [Runtime.InteropServices.Marshal]::PtrToStringAuto(
    [Runtime.InteropServices.Marshal]::SecureStringToBSTR($pass)
)
$SERVER_PASS  = $SSH_PASS
$CLIENT1_PASS = $SSH_PASS
$CLIENT2_PASS = $SSH_PASS

# Write password to a temp file with UTF-8 encoding so sshpass handles non-ASCII (e.g. ñ)
$tmpPassFile = [System.IO.Path]::GetTempFileName()
[System.IO.File]::WriteAllText($tmpPassFile, $SSH_PASS, [System.Text.Encoding]::UTF8)

if (-not ($Server -or $Clients -or $All -or $Update -or $Db)) { $All = $true }

# ── Helpers ───────────────────────────────────────────────────────────────────
function Set-BuildEnv {
    $env:GOOS        = "linux"
    $env:GOARCH      = "amd64"
    $env:CGO_ENABLED = "0"
}

function Clear-BuildEnv {
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
}

$hasSshpass = $null -ne (Get-Command sshpass -ErrorAction SilentlyContinue)

function Invoke-SSH {
    param([string]$target, [string]$password, [string]$cmd)
    if ($password -and $hasSshpass) {
        sshpass -f $tmpPassFile ssh -o StrictHostKeyChecking=no $target $cmd
    } else {
        ssh $target $cmd
    }
}

function Invoke-SCP {
    param([string]$target, [string]$password, [string]$src, [string]$dst)
    if ($password -and $hasSshpass) {
        sshpass -f $tmpPassFile scp -o StrictHostKeyChecking=no $src "${target}:${dst}"
    } else {
        scp $src "${target}:${dst}"
    }
}

function Invoke-SCPFrom {
    param([string]$target, [string]$password, [string]$src, [string]$dst)
    if ($password -and $hasSshpass) {
        sshpass -f $tmpPassFile scp -o StrictHostKeyChecking=no "${target}:${src}" $dst
    } else {
        scp "${target}:${src}" $dst
    }
}

# ── Deploy server ─────────────────────────────────────────────────────────────
function Deploy-Server {
    Write-Host "`n=== Building server ===" -ForegroundColor Cyan
    Set-BuildEnv
    go build -o probakgo .
    if ($LASTEXITCODE -ne 0) { Clear-BuildEnv; Write-Error "Build failed"; return }
    Clear-BuildEnv

    Write-Host "=== Deploying server to $SERVER_HOST ===" -ForegroundColor Cyan
    Invoke-SCP $SERVER_HOST $SERVER_PASS "probakgo" "/tmp/probakgo"
    if ($LASTEXITCODE -ne 0) { Write-Error "SCP failed"; return }

    Invoke-SSH $SERVER_HOST $SERVER_PASS @'
set -e
mkdir -p /opt/probakgo
mv /tmp/probakgo /opt/probakgo/probakgo
chmod +x /opt/probakgo/probakgo
if systemctl is-active --quiet probakgo 2>/dev/null; then
    systemctl restart probakgo
    echo "Service restarted"
else
    echo "Binary updated at /opt/probakgo/probakgo"
fi
'@
    Write-Host "Server deployed OK" -ForegroundColor Green
}

# ── Deploy PVE client (.230) ──────────────────────────────────────────────────
function Deploy-Client1 {
    Write-Host "`n=== Deploying client to $CLIENT1_HOST (PVE) ===" -ForegroundColor Cyan
    Invoke-SCP $CLIENT1_HOST $CLIENT1_PASS "probakgo-client" "/tmp/probakgo-client"
    if ($LASTEXITCODE -ne 0) { Write-Error "SCP to $CLIENT1_HOST failed"; return }

    Invoke-SSH $CLIENT1_HOST $CLIENT1_PASS @'
set -e
mv /tmp/probakgo-client /opt/probakgo/probakgo-client
chmod +x /opt/probakgo/probakgo-client
echo "Client updated OK"
'@
    Write-Host "Client $CLIENT1_HOST deployed OK" -ForegroundColor Green

    Write-Host "=== Running vzdump on $CLIENT1_HOST ===" -ForegroundColor Cyan
    # Invoke-SSH $CLIENT1_HOST $CLIENT1_PASS "vzdump 101 --storage NAS --mode snapshot"

    # Invoke-SSH $CLIENT1_HOST $CLIENT1_PASS "vzdump 101 --notification-mode auto --node soporte1 --remove 0 --mode snapshot --storage PBS"
    # if ($LASTEXITCODE -eq 0) {
    #     Write-Host "vzdump PBS OK" -ForegroundColor Green
    # } else {
    #     Write-Warning "vzdump PBS returned exit code $LASTEXITCODE"
    # }

    # Write-Host "=== Running PVE report on $CLIENT1_HOST ===" -ForegroundColor Cyan
    # Invoke-SSH $CLIENT1_HOST $CLIENT1_PASS "/opt/probakgo/probakgo-client"
    # if ($LASTEXITCODE -eq 0) {
    #     Write-Host "PVE report OK" -ForegroundColor Green
    # } else {
    #     Write-Warning "PVE report returned exit code $LASTEXITCODE"
    # }
}

# ── Deploy PBS client (.248) ──────────────────────────────────────────────────
function Deploy-Client2 {
    Write-Host "`n=== Deploying client to $CLIENT2_HOST (PBS) ===" -ForegroundColor Cyan
    Invoke-SCP $CLIENT2_HOST $CLIENT2_PASS "probakgo-client" "/tmp/probakgo-client"
    if ($LASTEXITCODE -ne 0) { Write-Error "SCP to $CLIENT2_HOST failed"; return }

    Invoke-SSH $CLIENT2_HOST $CLIENT2_PASS @'
set -e
mv /tmp/probakgo-client /opt/probakgo/probakgo-client
chmod +x /opt/probakgo/probakgo-client
echo "Client updated OK"
'@
    Write-Host "Client $CLIENT2_HOST deployed OK" -ForegroundColor Green

    Write-Host "=== Running PBS report on $CLIENT2_HOST ===" -ForegroundColor Cyan
    Invoke-SSH $CLIENT2_HOST $CLIENT2_PASS "/opt/probakgo/probakgo-client"
    if ($LASTEXITCODE -eq 0) {
        Write-Host "PBS report OK" -ForegroundColor Green
    } else {
        Write-Warning "PBS report returned exit code $LASTEXITCODE"
    }
}

function Deploy-Clients {
    Write-Host "`n=== Building client ===" -ForegroundColor Cyan
    Set-BuildEnv
    go build -o probakgo-client ./client/
    if ($LASTEXITCODE -ne 0) { Clear-BuildEnv; Write-Error "Build failed"; return }
    Clear-BuildEnv

    Deploy-Client1
    Deploy-Client2
}

# ── Update from GitHub Releases ───────────────────────────────────────────────
function Update-All {
    Write-Host "`n=== Running update on server ($SERVER_HOST) ===" -ForegroundColor Cyan
    Invoke-SSH $SERVER_HOST $SERVER_PASS "/opt/probakgo/probakgo update"
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Server update OK" -ForegroundColor Green
    } else {
        Write-Warning "Server update returned exit code $LASTEXITCODE"
    }

    foreach ($pair in @(@($CLIENT1_HOST, $CLIENT1_PASS), @($CLIENT2_HOST, $CLIENT2_PASS))) {
        $t = $pair[0]; $p = $pair[1]
        Write-Host "`n=== Running update on client ($t) ===" -ForegroundColor Cyan
        Invoke-SSH $t $p "/opt/probakgo/probakgo-client update"
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Client $t update OK" -ForegroundColor Green
        } else {
            Write-Warning "Client $t update returned exit code $LASTEXITCODE"
        }
    }
}

# ── Main ──────────────────────────────────────────────────────────────────────
function Fetch-DB {
    Write-Host "`n=== Fetching database from $SERVER_HOST ===" -ForegroundColor Cyan
    Invoke-SCPFrom $SERVER_HOST $SERVER_PASS "/opt/probakgo/probakgo_data.db" ".\probakgo_data.db"
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Database saved to .\probakgo_data.db" -ForegroundColor Green
    } else {
        Write-Warning "SCP from $SERVER_HOST failed"
    }
}

if ($All -or $Server)  { Deploy-Server }
if ($All -or $Clients) { Deploy-Clients }
if ($Update)           { Update-All }
if ($Db)               { Fetch-DB }

Remove-Item $tmpPassFile -ErrorAction SilentlyContinue
Write-Host "`nDone." -ForegroundColor Green
