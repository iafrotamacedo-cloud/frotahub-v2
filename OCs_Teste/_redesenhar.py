# -*- coding: utf-8 -*-
"""Regrava as 30 OCs de teste no layout da Ordem de Compra 019731 (Obra Prima).

Lê cada PDF atual (PyMuPDF, colunas no estilo pdftotext -layout), extrai com
o mesmo ExtrairDoTexto de produção, veste cabeçalho/endereços no padrão da
019731 e gera de novo com DesenharOC. Os números, CNPJs (inclusive vazios e
errados) e itens do gabarito se mantêm.
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
from collections import defaultdict
from pathlib import Path

import pymupdf

RAIZ = Path(__file__).resolve().parent
GABARITO = RAIZ / "_gabarito.json"


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
                espacos = max(1, int(round(gap / 3.5)))
                buf += " " * espacos + palavra
                cursor = x1
            linhas.append(buf)
        paginas.append("\n".join(linhas))
    return "\n".join(paginas)


def main() -> int:
    gabarito = json.loads(GABARITO.read_text(encoding="utf-8"))
    print(f"Redesenhando {len(gabarito)} OCs no layout da 019731\n")
    with tempfile.TemporaryDirectory() as tmp:
        pasta = Path(tmp)
        jobs: list[dict] = []
        faltando = 0
        for i, caso in enumerate(gabarito):
            pdf = RAIZ / caso["arquivo"]
            if not pdf.exists():
                print(f"  FALHOU {caso['arquivo']}: arquivo ausente")
                faltando += 1
                continue
            txt = pasta / f"{i:02d}.txt"
            js = pasta / f"{i:02d}.json"
            txt.write_text(pdf_para_texto_layout(pdf), encoding="utf-8")
            js.write_text(json.dumps(caso, ensure_ascii=False), encoding="utf-8")
            jobs.append(
                {
                    "texto": txt.name,
                    "caso": js.name,
                    "destino": str(pdf),
                    "arquivo": caso["arquivo"],
                }
            )
        (pasta / "jobs.json").write_text(
            json.dumps(jobs, ensure_ascii=False), encoding="utf-8"
        )
        env = {**os.environ, "OC_LOTE": str(pasta)}
        proc = subprocess.run(
            ["go", "test", "./interno/modulos/administrativo",
             "-run", "^TestLoteOCsTeste$", "-count=1", "-v"],
            cwd=RAIZ.parent / "baleryan",
            capture_output=True,
            text=True,
            encoding="utf-8",
            env=env,
        )
        if proc.stdout:
            print(proc.stdout, end="" if proc.stdout.endswith("\n") else "\n")
        if proc.returncode != 0:
            err = (proc.stderr or "").strip()
            if err:
                print(err)
            print("\nResumo: lote Go falhou")
            return 1
        ok = len(jobs)
        print(f"\nResumo: {ok}/{len(gabarito)}")
        return 0 if faltando == 0 and ok == len(gabarito) else 1


if __name__ == "__main__":
    sys.exit(main())
