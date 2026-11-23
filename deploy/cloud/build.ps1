param(
    [ValidatePattern('^[a-zA-Z0-9][a-zA-Z0-9_.-]*$')]
    [string]$Tag = ('cloud-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
)
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$oldBase = $env:NOTE_WEB_BASE
Push-Location $repo
try {
    # Public cloud serves the app at the domain root, not /p/note/.
    $env:NOTE_WEB_BASE = '/'
    $lines = & (Join-Path $repo 'deploy/server/build.ps1')
    $lines | Write-Host
    $stage = ($lines | Select-Object -Last 1).ToString().Trim()
    if (!(Test-Path -LiteralPath (Join-Path $stage 'note-api'))) { throw 'Missing build output' }
    docker build -t "note-api:$Tag" $stage
    if ($LASTEXITCODE -ne 0) { throw 'Docker build failed' }
    $out = Join-Path $repo "build/cloud/$Tag"
    if (Test-Path -LiteralPath $out) { throw "Release already exists: $out" }
    New-Item -ItemType Directory -Path $out -Force | Out-Null
    foreach ($name in @('compose.yaml', 'compose.gateway.yaml', 'Caddyfile', '.env.example', 'backup.sh', 'install-docker.sh', 'README.md')) {
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot $name) -Destination $out
    }
    docker save -o (Join-Path $out 'images.tar') "note-api:$Tag"
    if ($LASTEXITCODE -ne 0) { throw 'Docker export failed' }
    [IO.File]::WriteAllText((Join-Path $out 'image.txt'), "note-api:$Tag`n")
    $names = @('images.tar', 'compose.yaml', 'compose.gateway.yaml', 'Caddyfile', '.env.example', 'backup.sh', 'install-docker.sh', 'README.md', 'image.txt')
    $sums = foreach ($name in $names) {
        (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $out $name)).Hash.ToLowerInvariant() + "  $name"
    }
    [IO.File]::WriteAllText((Join-Path $out 'SHA256SUMS'), ($sums -join "`n") + "`n")
    Write-Output "Upload directory: $out"
} finally {
    $env:NOTE_WEB_BASE = $oldBase
    Pop-Location
}
