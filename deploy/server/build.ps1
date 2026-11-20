$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$out = Join-Path $repo 'build/server'
New-Item -ItemType Directory -Force $out | Out-Null
Push-Location (Join-Path $repo 'web')
try {
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' }
} finally { Pop-Location }
$oldOS, $oldArch, $oldCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
Push-Location $repo
try {
    $env:GOOS = 'linux'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    go build -trimpath -ldflags='-s -w' -o (Join-Path $out 'note-api') ./cmd/api
    if ($LASTEXITCODE -ne 0) { throw 'Backend build failed' }
} finally {
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $oldOS, $oldArch, $oldCGO
    Pop-Location
}
# robocopy /MIR is intentionally avoided; this build uses a fresh staging folder.
$stage = Join-Path $out ('release-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory $stage | Out-Null
Copy-Item -LiteralPath (Join-Path $out 'note-api') -Destination $stage
Copy-Item -LiteralPath (Join-Path $repo 'web/dist') -Destination (Join-Path $stage 'web') -Recurse
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'Dockerfile') -Destination $stage
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'compose.yaml') -Destination $stage
Write-Output $stage
