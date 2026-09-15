#!/usr/bin/env bash
# Baixa os três arquivos que o ERA READ precisa e confere o sha256 de cada um.
#
# USO
#   bash infra/era/baixar-modelos.sh /opt/baleryan/modelos
#
# Depois, no ambiente do motor (Oracle: /etc/baleryan.env):
#   ERA_DET_ONNX=/opt/baleryan/modelos/ch_PP-OCRv4_det_infer.onnx
#   ERA_REC_ONNX=/opt/baleryan/modelos/latin_PP-OCRv3_mobile_rec_infer.onnx
#   ERA_DICT=/opt/baleryan/modelos/latin_dict.txt
#
# DE ONDE VÊM (e a licença — ver README do era-read, "SVTR: o grafo já roda")
#   · detector  ch_PP-OCRv4_det_infer.onnx  — espelho SWHL/RapidOCR (Hugging Face)
#   · reconhecedor latin_PP-OCRv3_mobile_rec_infer.onnx — espelho docato/PaddleOCR_Mobile_Models
#   · dicionário latin_dict.txt — fonte primária, PaddlePaddle/PaddleOCR no GitHub
#   Os sha256 abaixo são os mesmos dos arquivos com que o era-read foi validado
#   em 14–15/09/2026. Arquivo diferente = leitura diferente; por isso o script
#   recusa qualquer byte fora do esperado.
#
# NÃO RODE ISTO NO RENDER GRATUITO
#   Medido em 15/09/2026 no PC do dono: uma página a 1600 px usa ~1,5 GB de
#   memória e 30 s+ de CPU. O plano gratuito do Render tem 512 MB — o motor
#   inteiro reiniciaria no meio da primeira leitura. O ERA READ é para a VM
#   da Oracle (ver docs/ e infra/oracle).
set -euo pipefail

destino="${1:?informe a pasta de destino}"
mkdir -p "$destino"

baixar() {
  local url="$1" nome="$2" sha="$3"
  local alvo="$destino/$nome"
  if [[ -f "$alvo" ]] && echo "$sha  $alvo" | sha256sum -c --quiet 2>/dev/null; then
    echo "ok (já existia): $nome"
    return
  fi
  echo "baixando $nome ..."
  curl -fsSL --retry 3 -o "$alvo.parcial" "$url"
  echo "$sha  $alvo.parcial" | sha256sum -c --quiet
  mv "$alvo.parcial" "$alvo"
  echo "ok: $nome"
}

baixar "https://huggingface.co/SWHL/RapidOCR/resolve/main/PP-OCRv4/ch_PP-OCRv4_det_infer.onnx" \
  ch_PP-OCRv4_det_infer.onnx \
  d2a7720d45a54257208b1e13e36a8479894cb74155a5efe29462512d42f49da9

baixar "https://huggingface.co/docato/PaddleOCR_Mobile_Models/resolve/main/onnx/latin_PP-OCRv3_mobile_rec_infer.onnx" \
  latin_PP-OCRv3_mobile_rec_infer.onnx \
  3380c9a056655b76a36d74fb8888e7206ea0306e118960bda842c7012eda2f23

baixar "https://raw.githubusercontent.com/PaddlePaddle/PaddleOCR/main/ppocr/utils/dict/latin_dict.txt" \
  latin_dict.txt \
  8e6d4e3629788c35c31f7e530287d6147b549bb7a265bd6708bb281134429e2c

echo
echo "Pronto. Aponte no ambiente do motor:"
echo "  ERA_DET_ONNX=$destino/ch_PP-OCRv4_det_infer.onnx"
echo "  ERA_REC_ONNX=$destino/latin_PP-OCRv3_mobile_rec_infer.onnx"
echo "  ERA_DICT=$destino/latin_dict.txt"
