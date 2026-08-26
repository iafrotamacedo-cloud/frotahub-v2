#!/usr/bin/env python3
"""Prepara a imagem de uma nota para o OCR.

  uso:  python3 otimizar_imagem.py <entrada> <saida.png> [altura]

POR QUE PYTHON NUMA CASA DE GO
  O motor continua sem dependência nenhuma. Este arquivo não roda no motor:
  roda no GitHub Actions, ao lado do Tesseract e do poppler, pela mesma razão
  que eles — lá há tempo, disco e liberdade para instalar. O leitor chama este
  programa como chama os outros dois, e lê o resultado.
"""
#
# O QUE ESTE CORPUS É — E O QUE ELE NÃO É
#   Não são fotos de papel amassado. São CAPTURAS DE TELA do SysPDV: o
#   documento é um retângulo branco, perfeitamente alinhado, dentro de uma
#   janela do Windows. Não há inclinação, não há perspectiva, não há sombra.
#   O problema é UM só: o documento ocupa uns 530x330 px de uma tela de
#   1440x900, então a letra tem 6 a 8 pixels de altura. O Tesseract precisa de
#   três a quatro vezes isso.
#
#   Por isso o tratamento é RECORTAR e AMPLIAR — e não endireitar, corrigir
#   perspectiva ou tirar sombra, que aqui só custariam tempo.
import os, subprocess, sys, tempfile
import cv2, numpy as np

# Abaixo disto o Tesseract está chutando a orientação. Ver graus_de_giro().
CONFIANCA_MINIMA_DE_GIRO = 2.0

def graus_de_giro(caminho):
    """Pergunta ao Tesseract se a página está de lado.

    PÁGINA DEITADA É LEITURA ZERO, NÃO LEITURA RUIM
      Medido: o documento 18713 é um PDF girado 90 graus. O OCR devolveu texto
      que não casou com NENHUM campo — nem valor, nem número, nem ticket. Não
      era o OCR sendo ruim: era o Tesseract lendo de lado, o que não produz
      resultado parcial, produz nada.

      A detecção de orientação (`--psm 0`) resolve isso. Só giros de 90 em 90 —
      que é o caso real de página escaneada de lado; inclinação de dois graus é
      outro problema, e neste corpus (capturas de tela) ele não existe.

    E A CONFIANÇA NÃO É DETALHE: FOI ELA QUE PEGOU UMA PIORA MEDIDA
      Sem limiar, esta função DERRUBOU o resultado. Em captura de tela de baixa
      resolução o Tesseract chuta orientação com confiança perto de zero — e
      chuta 270 graus em documento que está perfeitamente de pé. Medido: cinco
      documentos que liam bem passaram a não ler nada.

      A separação é limpa. Na página realmente deitada a confiança foi 6,74;
      nos falsos positivos foi 0,00 · 0,14 · 0,15 · 0,35. Abaixo de 2 não se
      mexe na imagem — a dúvida se resolve deixando como está.
    """
    try:
        r = subprocess.run(["tesseract", caminho, "stdout", "--psm", "0"],
                           capture_output=True, timeout=60)
        graus, confianca = 0, 0.0
        for linha in r.stdout.decode("utf-8", "replace").splitlines():
            baixa = linha.lower()
            if baixa.startswith("rotate:"):
                graus = int(linha.split(":")[1].strip()) % 360
            elif baixa.startswith("orientation confidence:"):
                confianca = float(linha.split(":")[1].strip())
        if confianca >= CONFIANCA_MINIMA_DE_GIRO:
            return graus
    except Exception:
        pass
    return 0


def girar(img, graus):
    if graus == 90:
        return cv2.rotate(img, cv2.ROTATE_90_CLOCKWISE)
    if graus == 180:
        return cv2.rotate(img, cv2.ROTATE_180)
    if graus == 270:
        return cv2.rotate(img, cv2.ROTATE_90_COUNTERCLOCKWISE)
    return img


def recortar_documento(img):
    """Acha o retângulo branco do documento dentro da captura de tela."""
    cinza = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
    # O papel é quase branco; a moldura do aplicativo é cinza.
    _, mascara = cv2.threshold(cinza, 235, 255, cv2.THRESH_BINARY)
    mascara = cv2.morphologyEx(mascara, cv2.MORPH_CLOSE, np.ones((9, 9), np.uint8))
    contornos, _ = cv2.findContours(mascara, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    if not contornos:
        return img, False
    maior = max(contornos, key=cv2.contourArea)
    x, y, w, h = cv2.boundingRect(maior)
    area = (w * h) / float(img.shape[0] * img.shape[1])
    # Menos de 8% é ruído; mais de 95% é a tela inteira e não vale recortar.
    if area < 0.08 or area > 0.95 or w < 200 or h < 150:
        return img, False
    m = 4
    y0, y1 = max(0, y - m), min(img.shape[0], y + h + m)
    x0, x1 = max(0, x - m), min(img.shape[1], x + w + m)
    return img[y0:y1, x0:x1], True

def preparar(caminho_entrada, caminho_saida, alvo_altura=3000):
    img = cv2.imread(caminho_entrada)
    if img is None:
        return None

    graus = graus_de_giro(caminho_entrada)
    if graus:
        img = girar(img, graus)

    corte, recortou = recortar_documento(img)

    # AMPLIAR ATÉ A LETRA TER TAMANHO DE LEITURA
    #   O alvo é a altura do documento, não um fator fixo: assim uma captura
    #   grande não é inflada à toa e uma pequena chega no tamanho certo.
    escala = alvo_altura / float(corte.shape[0])
    escala = max(1.0, min(escala, 6.0))
    if escala > 1.01:
        corte = cv2.resize(corte, None, fx=escala, fy=escala, interpolation=cv2.INTER_CUBIC)

    cinza = cv2.cvtColor(corte, cv2.COLOR_BGR2GRAY)
    # Realce leve: a ampliação borra a borda da letra, e o Tesseract gosta de
    # borda dura. Forte demais engorda o traço e cola dígito com dígito.
    borrado = cv2.GaussianBlur(cinza, (0, 0), 1.2)
    cinza = cv2.addWeighted(cinza, 1.6, borrado, -0.6, 0)
    _, binario = cv2.threshold(cinza, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU)

    cv2.imwrite(caminho_saida, binario, [cv2.IMWRITE_PNG_COMPRESSION, 1])
    return {"girou": graus, "recortou": recortou, "escala": round(escala, 2),
            "saida": (binario.shape[1], binario.shape[0])}

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("uso: otimizar_imagem.py <entrada> <saida.png> [altura]", file=sys.stderr)
        sys.exit(2)
    altura = int(sys.argv[3]) if len(sys.argv) > 3 else 3000
    r = preparar(sys.argv[1], sys.argv[2], alvo_altura=altura)
    if r is None:
        print("não consegui abrir a imagem", file=sys.stderr)
        sys.exit(1)
    print(r, file=sys.stderr)
