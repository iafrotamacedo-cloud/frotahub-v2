# Retoma OCs presas em "lendo" (relê com o motor corrigido).
$ErrorActionPreference = "Stop"
$repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$envFile = Join-Path $repo "baleryan\.env"
if (-not (Test-Path $envFile)) { Write-Host "Nao achei $envFile"; exit 1 }
Get-Content $envFile | ForEach-Object {
    $linha = $_.Trim()
    if ($linha -eq "" -or $linha.StartsWith("#")) { return }
    $partes = $linha -split "=", 2
    if ($partes.Count -ne 2) { return }
    $chave = $partes[0].Trim()
    $valor = $partes[1].Trim().Trim('"')
    if ($valor -eq "") { return }
    [System.Environment]::SetEnvironmentVariable($chave, $valor, "Process")
}
$poppler = Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Recurse -Filter "pdftotext.exe" -ErrorAction SilentlyContinue | Select-Object -First 1
if ($poppler) {
    $env:Path = (Split-Path $poppler.FullName) + ";" + $env:Path
}
Set-Location (Join-Path $repo "baleryan")
Write-Host "Recuperando OCs presas em lendo..."
go test -tags=integracao ./interno/modulos/administrativo/ -run TestRecuperarPresasEmLendo -count=1 -timeout 10m -v
