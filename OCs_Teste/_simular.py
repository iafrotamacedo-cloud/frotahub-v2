#!/usr/bin/env python3
"""Simula a passagem das 30 OCs pelo leitor (ExtrairDoTexto + filtros).

Como este Windows não tem poppler instalado, reconstruímos linhas no estilo
`pdftotext -layout` a partir das posições das palavras no PDF (PyMuPDF) e
passamos o texto para o mesmo código Go de produção.
"""
from __future__ import annotations

import json
import subprocess
import sys
import tempfile
from collections import defaultdict
from pathlib import Path

import pymupdf

RAIZ = Path(__file__).resolve().parent
GABARITO = RAIZ / "_gabarito.json"
SIMULAR_GO = RAIZ / "simular"


def pdf_para_texto_layout(caminho: Path) -> str:
    doc = pymupdf.open(caminho)
    paginas: list[str] = []
    for page in doc:
        linhas_por_y: dict[float, list[tuple[float, float, str]]] = defaultdict(list)
        for x0, y0, x1, _y1, palavra, *_resto in page.get_text("words"):
            linhas_por_y[round(y0, 1)].append((x0, x1, palavra))
        linhas: list[str] = []
        for y in sorted(linhas_por_y):
            pedacos = sorted(linhas_por_y[y], key=lambda t: t[0])
            if not pedacos:
                continue
            buf = pedacos[0][2]
            cursor = pedacos[0][1]
            for x0, x1, palavra in pedacos[1:]:
                gap = max(0.0, x0 - cursor)
                # Espaços extras preservam colunas — o leitor para o "Nome:"
                # no `\s{2,}` antes da coluna "Endereco:".
                espacos = max(1, int(round(gap / 3.5)))
                buf += " " * espacos + palavra
                cursor = x1
            linhas.append(buf)
        paginas.append("\n".join(linhas))
    return "\n".join(paginas)


def rodar_go(texto: str) -> dict:
    with tempfile.NamedTemporaryFile("w", encoding="utf-8", suffix=".txt", delete=False) as tmp:
        tmp.write(texto)
        caminho = tmp.name
    try:
        proc = subprocess.run(
            ["go", "run", ".", caminho],
            cwd=SIMULAR_GO,
            capture_output=True,
            text=True,
            encoding="utf-8",
            check=False,
        )
    finally:
        Path(caminho).unlink(missing_ok=True)
    if proc.returncode != 0:
        raise RuntimeError(proc.stderr.strip() or proc.stdout.strip())
    return json.loads(proc.stdout)


def quase_igual(a: float, b: float, tol: float = 0.02) -> bool:
    return abs(a - b) <= tol


def motivo_bate(esperado: str, motivos: list[str]) -> bool:
    if not esperado:
        return len(motivos) == 0
    texto = " ".join(motivos).lower()
    if "fornecedor" in esperado.lower() and "fornecedor" in texto:
        ok_forn = True
    else:
        ok_forn = "fornecedor" not in esperado.lower()
    if "03720882" in esperado or "faturamento" in esperado.lower():
        ok_cnpj = "03720882" in texto or "faturamento" in texto
    else:
        ok_cnpj = True
    return ok_forn and ok_cnpj and len(motivos) > 0


def conferir(esperado: dict, obtido: dict) -> list[str]:
    falhas: list[str] = []
    if obtido.get("erro"):
        falhas.append(f"erro de leitura: {obtido['erro']}")
        return falhas
    checks = [
        ("status", esperado["esperado"], obtido["status"]),
        ("numero", esperado["numero"], obtido["numero"]),
        ("fornecedor_nome", esperado["fornecedor_nome"], obtido["fornecedor_nome"]),
        ("fornecedor_cnpj", esperado["fornecedor_cnpj"], obtido["fornecedor_cnpj"]),
        ("comprador_cnpj", esperado["comprador_cnpj"], obtido["comprador_cnpj"]),
        ("n_itens", esperado["n_itens"], obtido["n_itens"]),
    ]
    for campo, want, got in checks:
        if want != got:
            falhas.append(f"{campo}: esperado {want!r}, veio {got!r}")
    for campo in ("subtotal", "desconto", "total"):
        if not quase_igual(float(esperado[campo]), float(obtido[campo])):
            falhas.append(
                f"{campo}: esperado {esperado[campo]}, veio {obtido[campo]}"
            )
    if esperado["esperado"] == "REJEITADA" and not motivo_bate(esperado.get("motivo", ""), obtido.get("motivos") or []):
        falhas.append(
            f"motivo: gabarito={esperado.get('motivo')!r}, motor={obtido.get('motivos')!r}"
        )
    return falhas


def main() -> int:
    gabarito = json.loads(GABARITO.read_text(encoding="utf-8"))
    resultados: list[dict] = []
    ok = 0
    print(f"Simulando {len(gabarito)} OCs contra {GABARITO.name}\n")
    print(f"{'Arquivo':<45} {'Status':<12} {'Itens':>5} {'Total':>10}  Resultado")
    print("-" * 95)
    for caso in gabarito:
        pdf = RAIZ / caso["arquivo"]
        texto = pdf_para_texto_layout(pdf)
        obtido = rodar_go(texto)
        falhas = conferir(caso, obtido)
        passou = len(falhas) == 0
        if passou:
            ok += 1
        resultados.append(
            {
                "arquivo": caso["arquivo"],
                "numero": caso["numero"],
                "passou": passou,
                "falhas": falhas,
                "obtido": obtido,
            }
        )
        marca = "OK" if passou else "FALHOU"
        total = obtido.get("total", 0)
        n_itens = obtido.get("n_itens", 0)
        status = obtido.get("status", obtido.get("erro", "?"))
        print(
            f"{caso['arquivo']:<45} {status:<12} {n_itens:>5} {total:>10.2f}  {marca}"
        )
        if falhas:
            for f in falhas:
                print(f"    -> {f}")
            if obtido.get("motivos"):
                print(f"    motivos motor: {obtido['motivos']}")
    print("-" * 95)
    print(f"Resumo: {ok}/{len(gabarito)} OK, {len(gabarito) - ok} falha(s)")
    relatorio = RAIZ / "_resultado_simulacao.json"
    relatorio.write_text(
        json.dumps(resultados, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(f"Detalhes gravados em {relatorio.name}")
    return 0 if ok == len(gabarito) else 1


if __name__ == "__main__":
    sys.exit(main())
