# Envia as OCs pendentes de PCO pelo Brevo (integracao real).
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
Set-Location (Join-Path $repo "baleryan")
Write-Host "Enviando PCO pendentes via Brevo..."
go test -tags=integracao ./interno/modulos/administrativo/ -run TestEnviarPCOPendentes -count=1 -timeout 5m -v
