# rev 1 — sobe o motor localmente, lendo baleryan/.env
#
# O motor em Go não lê ".env" sozinho (projeto sem dependência externa
# nenhuma, de propósito). Este script só empresta as variáveis do .env pro
# processo do `go run` — não grava nada, não muda nada fora desta janela.
#
# Uso: copie .env.example para .env, preencha com valores de verdade, e rode
# este script (não `go run ./cmd/baleryan` direto — ele não veria as
# variáveis).

$envFile = Join-Path $PSScriptRoot ".env"
if (-not (Test-Path $envFile)) {
    Write-Host "Não achei $envFile — copie .env.example para .env e preencha antes de rodar." -ForegroundColor Yellow
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

Write-Host "Variáveis carregadas de $envFile — subindo o motor..." -ForegroundColor Green
Set-Location $PSScriptRoot
go run ./cmd/baleryan
