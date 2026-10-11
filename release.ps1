<#
    release.ps1 — Calcula el proximo tag vX.Y.Z y publica el release del launcher.

    El CI (.github/workflows/release.yml) se dispara al EMPUJAR un tag "v*" y toma la
    version del propio nombre del tag (no hay archivo de version que sincronizar).
    Este script busca el ultimo tag vX.Y.Z (local + remoto), calcula el siguiente,
    empuja la rama actual y crea+empuja el tag. Eso arranca el build y el auto-update.

    Uso (desde C:\Users\Walter\Desktop\launcher):
        .\release.ps1              # bump de PATCH  (v0.2.6 -> v0.2.7)   [default]
        .\release.ps1 -Minor       # bump de MINOR  (v0.2.6 -> v0.3.0)
        .\release.ps1 -Major       # bump de MAJOR  (v0.2.6 -> v1.0.0)
        .\release.ps1 -DryRun      # solo muestra que haria, sin tocar nada
        .\release.ps1 -Force       # no pide confirmacion
        .\release.ps1 -Version 0.2.9   # forzar un numero exacto (sin el "v")
#>
[CmdletBinding()]
param(
    [switch]$Minor,
    [switch]$Major,
    [switch]$DryRun,
    [switch]$Force,
    [string]$Version
)

$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot

function Die($msg) { Write-Host "ERROR: $msg" -ForegroundColor Red; exit 1 }

# --- Sanidad del repo ---
git rev-parse --is-inside-work-tree *> $null; if (-not $?) { Die "no es un repo git" }
$branch = (git rev-parse --abbrev-ref HEAD).Trim()
if ($branch -ne 'main') { Write-Host "AVISO: estas en la rama '$branch' (no 'main')." -ForegroundColor Yellow }

# Working tree limpio EN LO TRACKEADO (el release sale del commit actual). Los archivos sin
# trackear no bloquean (no entran al release).
if ((git status --porcelain --untracked-files=no).Length -gt 0) {
    Die "hay cambios trackeados sin commitear. Commitea (o descarta) antes de publicar el release."
}

Write-Host "Trayendo tags del remoto..." -ForegroundColor Cyan
git fetch --tags --quiet origin

# --- Determinar la version siguiente ---
if ($Version) {
    if ($Version -notmatch '^\d+\.\d+\.\d+$') { Die "-Version debe ser X.Y.Z (sin la 'v')" }
    $next = $Version
    $last = "(forzado)"
} else {
    # Ultimo tag vX.Y.Z por orden de version (local y remoto ya estan unidos tras el fetch).
    $tags = git tag --list 'v*' | Where-Object { $_ -match '^v\d+\.\d+\.\d+$' }
    if (-not $tags) { Die "no encontre ningun tag vX.Y.Z" }
    $parsed = $tags | ForEach-Object {
        $p = ($_ -replace '^v','').Split('.'); [pscustomobject]@{ Tag=$_; Maj=[int]$p[0]; Min=[int]$p[1]; Pat=[int]$p[2] }
    } | Sort-Object Maj, Min, Pat
    $cur = $parsed[-1]
    $last = $cur.Tag
    if     ($Major) { $next = "$($cur.Maj + 1).0.0" }
    elseif ($Minor) { $next = "$($cur.Maj).$($cur.Min + 1).0" }
    else            { $next = "$($cur.Maj).$($cur.Min).$($cur.Pat + 1)" }
}

$tag = "v$next"

# No pisar un tag existente.
if (git tag --list $tag) { Die "el tag $tag ya existe localmente" }
if ((git ls-remote --tags origin "refs/tags/$tag") -ne $null -and (git ls-remote --tags origin "refs/tags/$tag").Length -gt 0) {
    Die "el tag $tag ya existe en el remoto"
}

$headSha  = (git rev-parse --short HEAD).Trim()
$headDesc = (git log -1 --pretty='%s').Trim()

Write-Host ""
Write-Host "  Ultimo tag : $last"            -ForegroundColor Gray
Write-Host "  Proximo    : $tag"             -ForegroundColor Green
Write-Host "  Commit     : $headSha  $headDesc" -ForegroundColor Gray
Write-Host "  Rama       : $branch"          -ForegroundColor Gray
Write-Host ""

if ($DryRun) { Write-Host "DRY-RUN: no se hizo nada." -ForegroundColor Yellow; exit 0 }

if (-not $Force) {
    $ans = Read-Host "Publicar $tag ? (s/N)"
    if ($ans -notmatch '^[sS]') { Write-Host "Cancelado." -ForegroundColor Yellow; exit 0 }
}

Write-Host "Empujando la rama '$branch'..." -ForegroundColor Cyan
git push origin $branch
if (-not $?) { Die "fallo el push de la rama" }

Write-Host "Creando y empujando el tag $tag..." -ForegroundColor Cyan
git tag -a $tag -m "Release $tag"
git push origin $tag
if (-not $?) { git tag -d $tag *> $null; Die "fallo el push del tag (revertido el tag local)" }

Write-Host ""
Write-Host "LISTO. Tag $tag publicado -> el CI compila y publica el release." -ForegroundColor Green
Write-Host "Seguimiento: https://github.com/patagoniapz/pz-launcher/actions" -ForegroundColor Gray
