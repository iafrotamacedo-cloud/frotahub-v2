# -*- coding: utf-8 -*-
"""Gera 30 PDFs de teste de Ordem de Compra, no formato do Obra Prima,
para testar a rotina de leitura do FrotaHub (leitura.go) sem mexer no
sistema de verdade. Descartável — não faz parte do repositório."""
import os
import random
from reportlab.pdfgen import canvas
from reportlab.lib.pagesizes import A4

random.seed(20260911)

OUT = os.path.dirname(os.path.abspath(__file__))

LOJAS = [
    ("DUNAS", "03720882000239", "Fortaleza - CE"),
    ("CAMBEBA", "03720882000743", "Fortaleza - CE"),
    ("RUI B.", "03720882002444", "Fortaleza - CE"),
    ("VILLAS", "03720882003920", "Aquiraz - CE"),
]

CNPJ_ERRADOS = [
    "12345678000199",
    "98765432000188",
    "45612378000123",
    "78912345000167",
    "65498732000145",
]

FORNECEDORES = [
    ("S V COMERCIO DE MATERIAL ELETRICO LTDA", "35088657000137"),
    ("DISTRIBUIDORA CENTRAL DE HIDRAULICA LTDA", "12345098000122"),
    ("FERRAGENS SANTA FE COMERCIO LTDA", "98765123000144"),
    ("MEGA MATERIAIS DE CONSTRUCAO EIRELI", "11222333000144"),
    ("ELETRO CEARA DISTRIBUIDORA LTDA", "22333444000155"),
    ("TUBOS E CONEXOES DO NORDESTE LTDA", "33444555000166"),
]

ITENS_POOL = [
    ("ABRACADEIRA PVC WETZEL 3/4\" - BRANCO", 0.5, 3.0),
    ("BUCHA NYLON S-8 COM PARAFUSO", 0.1, 0.5),
    ("CONDULETE PVC 3/4\" BRANCO - WETZEL", 3.0, 6.0),
    ("CONECTOR BOX PVC 3/4\" BRANCO", 0.5, 1.5),
    ("CURVA 90 GRAUS WETZEL BRANCO 3/4", 1.5, 3.0),
    ("DISJUNTOR DR 25A BIPOLAR SCHNEIDER", 80.0, 150.0),
    ("DISJUNTOR MONOPOLAR 20A SCHNEIDER", 7.0, 12.0),
    ("ELETRODUTO PVC RIGIDO WETZEL 3/4", 10.0, 16.0),
    ("LUVA PVC 3/4\" BRANCO - WETZEL", 0.8, 1.5),
    ("TOMADA BRANCA 2P+T 20A COM TAMPA", 10.0, 18.0),
    ("FITA ISOLANTE 20M PRETA", 5.0, 9.0),
    ("PARAFUSO FRANCES 8X80MM CX C/100", 15.0, 30.0),
    ("TORNEIRA CROMADA PARA PIA", 35.0, 70.0),
    ("LAMPADA LED BULBO 9W BRANCA", 6.0, 14.0),
    ("CABO FLEXIVEL 2,5MM PRETO", 1.0, 2.5),
    ("REGISTRO GAVETA 3/4 BRUTO", 12.0, 25.0),
    ("JOELHO PVC SOLDAVEL 90 GRAUS 25MM", 1.0, 2.5),
    ("SIFAO SANFONADO PARA PIA", 8.0, 16.0),
    ("TE PVC SOLDAVEL 25MM", 1.5, 3.5),
    ("VALVULA DE ESCOA PARA PIA 1 1/2", 10.0, 20.0),
]

QTDS = [1, 2, 3, 4, 5, 6, 8, 10, 12, 15, 20, 25, 30, 50]


def brl(v: float) -> str:
    s = f"{v:,.2f}"
    return s.replace(",", "X").replace(".", ",").replace("X", ".")


def cnpj_fmt(d: str) -> str:
    return f"{d[0:2]}.{d[2:5]}.{d[5:8]}/{d[8:12]}-{d[12:14]}"


def montar_itens(n):
    itens = []
    for _ in range(n):
        nome, lo, hi = random.choice(ITENS_POOL)
        qtd = random.choice(QTDS)
        unit = round(random.uniform(lo, hi), 2)
        subtotal = round(qtd * unit, 2)
        desconto = 0.0
        total = round(subtotal - desconto, 2)
        itens.append((nome, qtd, unit, subtotal, desconto, total))
    return itens


ITENS_POR_PAGINA = 8


def gerar_pdf(caminho, numero, loja_nome, loja_cidade, comprador_cnpj,
              fornecedor_nome, fornecedor_cnpj, itens,
              data_str, previsao_str, cond_pgto, forma_pgto, comprador_interno):
    c = canvas.Canvas(caminho, pagesize=A4)
    largura, altura = A4
    total_paginas = max(1, (len(itens) + ITENS_POR_PAGINA - 1) // ITENS_POR_PAGINA)

    def y(pt_do_topo):
        return altura - pt_do_topo

    def letreiro(pagina):
        c.setFont("Helvetica-Bold", 10)
        c.drawString(50, y(40), "FROTA MACEDO ENGENHARIA LTDA")
        c.setFont("Helvetica", 9)
        c.drawRightString(largura - 50, y(40), data_str)
        c.drawString(50, y(54), "Engenheiro Heitor de Oliveira Albuquerque, 295 - Fortaleza/CE")
        c.drawRightString(largura - 50, y(54), f"Pagina {pagina}/{total_paginas}")
        c.drawString(50, y(68), "85 2181-1386 - frotamacedoengenharia@gmail.com - CNPJ: 27.363.223/0001-70")
        c.setFont("Helvetica-Bold", 12)
        c.drawString(50, y(95), f"ORDEM DE COMPRA {numero}")
        c.setFont("Helvetica", 10)
        c.drawString(50, y(110), f"{loja_nome} - {loja_cidade}")

    def cabecalho_tabela(y0):
        c.setFont("Helvetica-Bold", 8)
        c.drawString(50, y(y0), "N. Item")
        c.drawString(280, y(y0), "Qtd.")
        c.drawString(330, y(y0), "Unit. (R$)")
        c.drawString(400, y(y0), "Subtotal (R$)")
        c.drawString(470, y(y0), "Desc. (R$)")
        c.drawString(520, y(y0), "Total (R$)")
        c.line(50, y(y0 + 4), largura - 50, y(y0 + 4))

    # ---------------- pagina 1: cabecalho completo ----------------
    letreiro(1)
    topo = 150
    c.setFont("Helvetica-Bold", 9)
    c.drawString(50, y(topo), f"DADOS DA ORDEM DE COMPRA      {numero}")
    topo += 22
    c.setFont("Helvetica", 9)
    c.drawString(50, y(topo), f"Data:  {data_str}")
    c.drawString(300, y(topo), f"Previsão da entrega: {previsao_str}")
    topo += 14
    c.drawString(50, y(topo), f"Cond. pgto.:     {cond_pgto}")
    c.drawString(300, y(topo), f"Forma pgto.: {forma_pgto}")
    topo += 14
    c.drawString(50, y(topo), "Observacao:")
    topo += 14
    c.drawString(300, y(topo), f"Comprador: {comprador_interno}")
    topo += 14
    c.setFont("Helvetica-Bold", 9)
    c.drawString(50, y(topo), "RESPONSAVEL PELA COMPRA")
    c.setFont("Helvetica", 9)
    c.drawString(300, y(topo), "Email: compras@frotamacedo.com.br")
    topo += 14
    c.drawString(50, y(topo), "Nome: FROTA MACEDO ENGENHARIA LTDA")

    topo += 26
    c.setFont("Helvetica-Bold", 9)
    c.drawString(50, y(topo), "DADOS DO FATURAMENTO")
    topo += 16
    c.setFont("Helvetica", 9)
    c.drawString(50, y(topo), f"Nome:             {loja_nome}")
    c.drawString(400, y(topo), "Endereco: ROD. CE, KM 19, LOJA 11 -")
    topo += 14
    if comprador_cnpj:
        c.drawString(50, y(topo), f"CNPJ:             {comprador_cnpj}")
    else:
        c.drawString(50, y(topo), "CNPJ:")
    c.drawString(400, y(topo), f"{loja_cidade}, 61700-000")

    topo += 26
    c.setFont("Helvetica-Bold", 9)
    c.drawString(50, y(topo), "DADOS DO FORNECEDOR")
    topo += 16
    c.setFont("Helvetica", 9)
    c.drawString(50, y(topo), f"Nome:        {fornecedor_nome}")
    c.drawString(400, y(topo), "Endereco: Av. Central, 100 -")
    topo += 14
    if fornecedor_cnpj:
        c.drawString(50, y(topo), f"CNPJ:        {fornecedor_cnpj}")
    else:
        c.drawString(50, y(topo), "CNPJ:")
    c.drawString(400, y(topo), "Fortaleza - CE, 60000-000")
    topo += 14
    c.drawString(50, y(topo), "Telefone:")
    topo += 14
    c.drawString(50, y(topo), "Vendedor:    Contato principal")
    topo += 14
    c.drawString(50, y(topo), "E-mail:      contato@fornecedor.com.br")

    topo += 24
    c.setFont("Helvetica-Bold", 9)
    c.drawString(50, y(topo), f"OBRA/CENTRO DE CUSTO: {loja_nome} - {loja_cidade}")
    c.drawString(480, y(topo), "CNO:")

    topo += 24
    cabecalho_tabela(topo)
    linha_y = topo + 14

    n = 1
    itens_na_pagina = 0
    for nome, qtd, unit, subtotal, desconto, total in itens:
        if itens_na_pagina == ITENS_POR_PAGINA:
            c.showPage()
            letreiro((n - 1) // ITENS_POR_PAGINA + 1)
            cabecalho_tabela(150)
            linha_y = 164
            itens_na_pagina = 0
        c.setFont("Helvetica", 8)
        desc = f"{n} {nome}"
        if len(desc) > 42:
            desc = desc[:42]
        c.drawString(50, y(linha_y), desc)
        c.drawRightString(310, y(linha_y), f"{qtd:.2f}".replace(".", ","))
        c.drawRightString(390, y(linha_y), brl(unit))
        c.drawRightString(460, y(linha_y), brl(subtotal))
        c.drawRightString(510, y(linha_y), brl(desconto))
        c.drawRightString(570, y(linha_y), brl(total))
        linha_y += 13
        n += 1
        itens_na_pagina += 1

    subtotal_geral = round(sum(i[3] for i in itens), 2)
    desconto_geral = round(sum(i[4] for i in itens), 2)
    total_geral = round(sum(i[5] for i in itens), 2)

    linha_y += 14
    c.setFont("Helvetica-Bold", 8)
    c.drawString(300, y(linha_y), "Subtotal")
    c.drawRightString(460, y(linha_y), brl(subtotal_geral))
    c.drawRightString(510, y(linha_y), brl(desconto_geral))
    c.drawRightString(570, y(linha_y), brl(total_geral))
    linha_y += 13
    c.drawString(300, y(linha_y), "Frete")
    c.drawRightString(570, y(linha_y), brl(0.0))
    linha_y += 13
    c.drawString(300, y(linha_y), "Total")
    c.drawRightString(570, y(linha_y), brl(total_geral))

    c.save()
    return subtotal_geral, desconto_geral, total_geral


def main():
    registros = []
    numero_base = 20001

    def prox_numero():
        nonlocal numero_base
        numero_base += 1
        return str(numero_base - 1)

    def dados_comuns():
        loja = random.choice(LOJAS)
        return {
            "numero": prox_numero(),
            "loja_nome": loja[0],
            "loja_cidade": loja[2],
            "data_str": "05/09/2026",
            "previsao_str": "15/09/2026",
            "cond_pgto": "1 parcela",
            "forma_pgto": random.choice(["Boleto", "Pix", "Transferencia"]),
            "comprador_interno": random.choice(["Nadyson Ferreira", "Igor Tostes"]),
        }

    # 15 validas
    for i in range(15):
        d = dados_comuns()
        loja = next(l for l in LOJAS if l[0] == d["loja_nome"])
        fornecedor = random.choice(FORNECEDORES)
        n_itens = random.choice([2, 3, 4, 5, 6, 9, 11, 14])  # alguns 2 paginas
        itens = montar_itens(n_itens)
        caminho = os.path.join(OUT, f"OC_{d['numero']}_VALIDA.pdf")
        st, ds, tt = gerar_pdf(
            caminho, d["numero"], d["loja_nome"], d["loja_cidade"], loja[1],
            fornecedor[0], fornecedor[1], itens,
            d["data_str"], d["previsao_str"], d["cond_pgto"], d["forma_pgto"], d["comprador_interno"])
        registros.append({
            **d, "arquivo": os.path.basename(caminho), "esperado": "PROCESSADA",
            "motivo": "", "fornecedor_nome": fornecedor[0], "fornecedor_cnpj": fornecedor[1],
            "comprador_cnpj": loja[1], "n_itens": n_itens, "subtotal": st, "desconto": ds, "total": tt,
        })

    # 5 bloqueio: fornecedor sem CNPJ
    for i in range(5):
        d = dados_comuns()
        loja = next(l for l in LOJAS if l[0] == d["loja_nome"])
        fornecedor = random.choice(FORNECEDORES)
        n_itens = random.choice([2, 3, 5, 7])
        itens = montar_itens(n_itens)
        caminho = os.path.join(OUT, f"OC_{d['numero']}_BLOQ_FORNECEDOR_SEM_CNPJ.pdf")
        st, ds, tt = gerar_pdf(
            caminho, d["numero"], d["loja_nome"], d["loja_cidade"], loja[1],
            fornecedor[0], "", itens,
            d["data_str"], d["previsao_str"], d["cond_pgto"], d["forma_pgto"], d["comprador_interno"])
        registros.append({
            **d, "arquivo": os.path.basename(caminho), "esperado": "REJEITADA",
            "motivo": "fornecedor sem CNPJ", "fornecedor_nome": fornecedor[0], "fornecedor_cnpj": "",
            "comprador_cnpj": loja[1], "n_itens": n_itens, "subtotal": st, "desconto": ds, "total": tt,
        })

    # 5 bloqueio: CNPJ de faturamento errado
    for i in range(5):
        d = dados_comuns()
        fornecedor = random.choice(FORNECEDORES)
        cnpj_errado = random.choice(CNPJ_ERRADOS)
        n_itens = random.choice([1, 3, 6, 8])
        itens = montar_itens(n_itens)
        caminho = os.path.join(OUT, f"OC_{d['numero']}_BLOQ_CNPJ_FATURAMENTO_ERRADO.pdf")
        st, ds, tt = gerar_pdf(
            caminho, d["numero"], d["loja_nome"], d["loja_cidade"], cnpj_errado,
            fornecedor[0], fornecedor[1], itens,
            d["data_str"], d["previsao_str"], d["cond_pgto"], d["forma_pgto"], d["comprador_interno"])
        registros.append({
            **d, "arquivo": os.path.basename(caminho), "esperado": "REJEITADA",
            "motivo": "CNPJ de faturamento nao comeca com 03720882", "fornecedor_nome": fornecedor[0],
            "fornecedor_cnpj": fornecedor[1], "comprador_cnpj": cnpj_errado,
            "n_itens": n_itens, "subtotal": st, "desconto": ds, "total": tt,
        })

    # 5 bloqueio: os dois problemas juntos
    for i in range(5):
        d = dados_comuns()
        fornecedor = random.choice(FORNECEDORES)
        cnpj_errado = random.choice(CNPJ_ERRADOS)
        n_itens = random.choice([2, 4, 5])
        itens = montar_itens(n_itens)
        caminho = os.path.join(OUT, f"OC_{d['numero']}_BLOQ_FORNECEDOR_E_CNPJ.pdf")
        st, ds, tt = gerar_pdf(
            caminho, d["numero"], d["loja_nome"], d["loja_cidade"], cnpj_errado,
            fornecedor[0], "", itens,
            d["data_str"], d["previsao_str"], d["cond_pgto"], d["forma_pgto"], d["comprador_interno"])
        registros.append({
            **d, "arquivo": os.path.basename(caminho), "esperado": "REJEITADA",
            "motivo": "fornecedor sem CNPJ + CNPJ de faturamento errado", "fornecedor_nome": fornecedor[0],
            "fornecedor_cnpj": "", "comprador_cnpj": cnpj_errado,
            "n_itens": n_itens, "subtotal": st, "desconto": ds, "total": tt,
        })

    import json
    with open(os.path.join(OUT, "_gabarito.json"), "w", encoding="utf-8") as f:
        json.dump(registros, f, ensure_ascii=False, indent=2)
    print(f"Gerados {len(registros)} PDFs em {OUT}")


if __name__ == "__main__":
    main()
