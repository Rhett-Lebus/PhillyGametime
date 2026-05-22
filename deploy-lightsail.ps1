param(
    [Parameter(Mandatory = $true)]
    [string]$StaticIp,

    [Parameter(Mandatory = $true)]
    [string]$PemPath,

    [string]$RemoteUser = "ubuntu",
    [string]$RemoteDir = "/home/ubuntu/philly-gametime",
    [string]$ServiceName = "philly-gametime",
    [switch]$Restart
)

$ErrorActionPreference = "Stop"

function Set-PrivateKeyPermissions {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    $currentIdentity = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name

    Write-Host "Restricting permissions on PEM file..." -ForegroundColor Cyan
    & icacls $Path /inheritance:r | Out-Null
    & icacls $Path /remove:g "BUILTIN\Users" "NT AUTHORITY\Authenticated Users" "Everyone" | Out-Null
    & icacls $Path /grant:r "${currentIdentity}:R" | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to set permissions on PEM file."
    }
}

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$pemFullPath = (Resolve-Path -Path $PemPath).Path
$previousGoos = $env:GOOS
$previousGoarch = $env:GOARCH

if (-not (Test-Path -Path $pemFullPath -PathType Leaf)) {
    throw "PEM file not found: $PemPath"
}

$scp = Get-Command scp -ErrorAction SilentlyContinue
if (-not $scp) {
    throw "scp was not found in PATH. Install OpenSSH client or run this from a shell that has scp."
}

$ssh = Get-Command ssh -ErrorAction SilentlyContinue
if (-not $ssh) {
    throw "ssh was not found in PATH. Install OpenSSH client or run this from a shell that has ssh."
}

$sshOptions = @(
    "-o", "BatchMode=yes",
    "-o", "StrictHostKeyChecking=accept-new",
    "-o", "ConnectTimeout=10",
    "-o", "ServerAliveInterval=15",
    "-o", "ServerAliveCountMax=4"
)

$scpOptions = @(
    # Windows OpenSSH defaults to SFTP mode for scp; force legacy SCP mode
    # because some Lightsail images close the connection during SFTP uploads.
    "-O"
) + $sshOptions

Push-Location $repoRoot
try {
    Set-PrivateKeyPermissions -Path $pemFullPath

    Write-Host "Building Linux binary..." -ForegroundColor Cyan
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    go build -o philly-gametime .
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed."
    }
    $env:GOOS = $previousGoos
    $env:GOARCH = $previousGoarch

    Write-Host "Creating remote directory..." -ForegroundColor Cyan
    & $ssh.Source @sshOptions -i $pemFullPath "$RemoteUser@$StaticIp" "mkdir -p $RemoteDir"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create remote directory."
    }

    $remoteBinaryTemp = "$RemoteDir/philly-gametime.next"
    $remoteBinaryPath = "$RemoteDir/philly-gametime"

    Write-Host "Uploading binary to temporary path..." -ForegroundColor Cyan
    & $scp.Source @scpOptions -i $pemFullPath .\philly-gametime "${RemoteUser}@${StaticIp}:${remoteBinaryTemp}"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to upload binary. If SSH login works but SCP fails, check the server's SSH/SFTP configuration."
    }

    Write-Host "Replacing remote binary..." -ForegroundColor Cyan
    & $ssh.Source @sshOptions -i $pemFullPath "$RemoteUser@$StaticIp" "mv -f $remoteBinaryTemp $remoteBinaryPath && chmod +x $remoteBinaryPath"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to replace remote binary."
    }

    Write-Host "Uploading templates..." -ForegroundColor Cyan
    & $scp.Source @scpOptions -i $pemFullPath -r .\templates "${RemoteUser}@${StaticIp}:${RemoteDir}/"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to upload templates."
    }

    Write-Host "Uploading static assets..." -ForegroundColor Cyan
    & $scp.Source @scpOptions -i $pemFullPath -r .\static "${RemoteUser}@${StaticIp}:${RemoteDir}/"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to upload static assets."
    }

    Write-Host "Upload complete." -ForegroundColor Green
    if ($Restart) {
        Write-Host "Restarting $ServiceName..." -ForegroundColor Cyan
        & $ssh.Source @sshOptions -i $pemFullPath "$RemoteUser@$StaticIp" "sudo systemctl restart $ServiceName && sudo systemctl status $ServiceName --no-pager"
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to restart $ServiceName."
        }
    } else {
        Write-Host "Next steps on the server:" -ForegroundColor Yellow
        Write-Host "sudo systemctl restart $ServiceName"
        Write-Host "sudo systemctl status $ServiceName"
    }
}
finally {
    $env:GOOS = $previousGoos
    $env:GOARCH = $previousGoarch
    Pop-Location
}
