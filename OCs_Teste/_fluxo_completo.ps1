# Fluxo completo das 30 OCs - inserir, ler, vistas, envio PCO (integracao real).
$ErrorActionPreference = "Stop"
$repo = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$envFile = Join-Path $repo "baleryan\.env"
if (-not (Test-Path $envFile)) {
    Write-Host "Nao achei $envFile"
    exit 1
}
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
$poppler = Get-ChildItem -Path "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Recurse -Filter "pdftotext.exe" -ErrorAction SilentlyContinue | Select-Object -First 1
if ($poppler) {
    $env:Path = $poppler.DirectoryName + ";" + $env:Path
    Write-Host "pdftotext:" $poppler.FullName
} else {
    Write-Host "Aviso: pdftotext nao encontrado - teste pode pular."
}
Set-Location (Join-Path $repo "baleryan")
Write-Host "Rodando fluxo completo..."
go test -tags=integracao ./interno/modulos/administrativo/ -run TestFluxoCompleto30OCs -count=1 -timeout 15m -v
