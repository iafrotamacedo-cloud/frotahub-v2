# Diário — Editor e gerador de PDF da OC · 11/09/2026, 14:15

> Sessão Cursor (Grok) · módulo Administrativo › Compras  
> **Commit no `main` (já no remoto):** `5890a9c` — `adm: editor e gerador de PDF nas OCs rejeitadas.`  
> Referência visual: PDF Obra Prima `OrdemDeCompra_019731` (Downloads do dono)

Este texto é o handoff para a outra seção. O PCO/leitura de ontem **não** foi mexido, salvo o reaproveitamento de `apagarArquivoSeOrfao` (saiu do arquivo `integracao` e foi para produção).

---

## 0. O pedido

O dono queria um **editor e gerador de PDF implantável**, usando a OC 019731 como modelo: editar qualquer parte, substituir o PDF antigo **sem vestígio na folha** (sem overlay, sem “editado”, sem rodapé FrotaHub).

Depois fechou o escopo:

| Decisão | Valor |
|---|---|
| Onde | Só OCs **já inseridas** em Compras |
| Aparência do PDF gerado | Clone visual do Obra Prima (019731) |
| Outros modelos | Ler pela **mesma palavra-chave**, mesmo que o layout mude |
| Processadas | **Não se edita** — já passaram nos filtros (pedido explícito depois do primeiro corte) |

---

## 1. Como funciona (não é tampão no PDF antigo)

```
PDF na fila  →  leitura por palavras-chave  →  folha editável na tela
                                                      │
                                                      ▼
                                              desenharOC (Folha A4)
                                                      │
                                                      ▼
                              sobe no R2  →  atualiza sha/campos/itens/status
                                                      │
                                                      ▼
                              apaga o PDF antigo se ninguém mais aponta para o sha
```

O arquivo novo é desenhado do zero com `relatorio.Folha` (mesmo encanamento dos orçamentos). **Não** se pinta branco em cima do PDF do Obra Prima — isso deixaria o texto velho no fluxo.

Totais **não** vêm do que a tela mandou: no save, `RecalcularTotais` fecha `qtd × unitário − desconto + frete` e é isso que vai no PDF e no banco.

---

## 2. Quem pode editar

- **Rejeitadas:** botão **EDITAR** na barra do visor. Depois que a OC vira `lido` nesse fluxo, o botão some e aparece “Salvar e voltar”.
- **Processadas:** só consulta. Sem botão. O `POST` do documento recusa `status = lido` com 409 (“já foi processada e não pode ser editada”).
- PCO pendentes/enviados: sem EDITAR.

`JanelaReparoOC.tsx` e `POST /reparo` **continuam no código** (correção rápida CNPJ/obra). A UI de Rejeitadas **não usa mais o pop-up**: o editor de documento cobre esses campos. Não apagar o reparo sem o dono pedir — a API ainda existe.

Se o PCO já tiver sido enviado, a tela avisa: o e-mail antigo não muda; só o arquivo guardado.

---

## 3. Rotas novas

Em [`modulo.go`](../baleryan/interno/modulos/administrativo/modulo.go), mesma rotina `COMPRAS_ORDENS_GERENCIAR`:

| Método | Caminho | Papel |
|---|---|---|
| `GET` | `/administrativo/compras/ordens/{id}/documento` | Baixa o PDF, parseia, devolve JSON da folha |
| `POST` | `/administrativo/compras/ordens/{id}/documento` | Recebe JSON, gera PDF, substitui arquivo |

POST: número obrigatório, ≥1 item com descrição, número único no cliente, `MotivosDeRejeicao` de novo (`lido` / `falhou`), histórico `editar_documento_oc`, `nome_arquivo` **não muda**.

CNPJ de faturamento fora de `03720882` **não vai para a coluna** (`compradorCNPJParaBanco` / CHECK da 059); vai no PDF e em `erro_leitura`.

**Não houve migração.** Campos extras (endereço, I.E., CNO, e-mail…) vivem no PDF regenerado. O banco continua com as colunas da 059/060.

---

## 4. Arquivos desta leva

### Novos

| Arquivo | O que é |
|---|---|
| `baleryan/interno/modulos/administrativo/leitura_extras.go` | Campos extras + extração por marcador (acento/variante) + `RecalcularTotais` |
| `baleryan/interno/modulos/administrativo/documento_pdf.go` | `desenharOC` — clone 019731, paginação, sem marca FrotaHub |
| `baleryan/interno/modulos/administrativo/documento.go` | GET/POST, JSON da folha, upload R2, órfão, bloqueio de `lido` |
| `baleryan/interno/modulos/administrativo/documento_pdf_test.go` | Extração real, layout variante, totais, PDF, JSON |
| `web/src/telas/administrativo/EditorDeOC.tsx` | Folha editável (um campo por rótulo) |

### Alterados

| Arquivo | O que mudou |
|---|---|
| `leitura.go` | `Extraida` ganhou os campos extras; regex mais tolerante (`^\s*`, `Previsao`/`Previsão`, `OBRA / CENTRO`); `blocoEntreVar`; cabeçalho de página genérico |
| `modulo.go` | As duas rotas `/documento` |
| `reparo.go` | `extraidaDoBanco` preenche data/totais (fallback se o PDF não ler) |
| `limpar_teste_integracao_test.go` | `apagarArquivoSeOrfao` saiu daqui (tag `integracao`) e foi para `documento.go` — **produção usa** |
| `ListaDeOrdens.tsx` | EDITAR só em rejeitadas; abre `EditorDeOC`; recarrega o PDF depois de salvar |
| `tipos.ts` | `DocumentoOC` / `EstadoDocumentoOC` |
| `administrativo.css` | Estilo da folha (`.adm-folha`, etc.) |

---

## 5. Palavras-chave da leitura (outros modelos)

A entrada **não** depende de coordenada. Marcadores aceitos (com e sem acento):

`DADOS DA ORDEM DE COMPRA` · `Data:` · `Cond. pgto.:` · `Previsão`/`Previsao da entrega:` · `Forma pgto.:` · `Observação`/`Observacao:` · `RESPONSÁVEL`/`RESPONSAVEL PELA COMPRA` · `DADOS DO FATURAMENTO` · `DADOS DO FORNECEDOR` · `Nome:` · `CNPJ:` · `I.E.:` · `Endereço`/`Endereco:` · `Telefone:` · `Vendedor:` · `E-mail`/`Email:` · `OBRA/CENTRO DE CUSTO` · `CNO:` · `ENDEREÇO ENTREGA` · `ENDEREÇO COBRANÇA` · `Recebedor:` · `N. Item` · `Subtotal` / `Frete` / `Total`

Campo que o modelo diferente não tiver fica **vazio** no editor. A **saída** é sempre o layout 019731.

`Extraida` agora tem, além do que a leitura de negócio já usava: emitente (razão, endereço, contato, CNPJ), data de impressão, título da obra, observação, responsável (nome/e-mail), I.E. e endereço de faturamento, telefone/vendedor/e-mail/endereço do fornecedor, CNO, entrega, recebedor, cobrança.

A leitura de **filtros** (fornecedor com CNPJ, raiz `03720882`) **não mudou de regra**.

---

## 6. Testes

`go test ./interno/modulos/administrativo/` — ok.

| Teste | Resultado |
|---|---|
| `TestExtrairDoTexto_OCReal` | 10 itens da 019731, filtros passam |
| `TestExtrairCamposExtras_OCReal` | emitente, e-mails, endereços |
| `TestExtrairDoTexto_ModeloDiferenteMesmasPalavras` | layout outro, mesmas chaves |
| `TestRecalcularTotais` | 2×5−1 + 3×4 + frete 10 = 31 |
| `TestDesenharOC_ContemCamposEFechaConta` | `%PDF`, número da OC, **sem** “FrotaHub”/“gerado em” |
| `TestJSONDocumento_IdaEVolta` | JSON → Extraida fecha |
| `TestDesenharOC_RoundTripLer` | **SKIP nesta máquina** — `pdftotext` não está no PATH. No Render (poppler) deve rodar |

`npx tsc --noEmit` no `web/` — limpo.

**Não** testei o fluxo no navegador (login). A outra seção, se for validar na tela: Rejeitadas → REPARAR → EDITAR → mudar um campo → salvar PDF → o visor tem que mostrar o arquivo novo.

---

## 7. O que a outra seção **não** deve fazer sem o dono pedir

- Ligar EDITAR em Processadas (o dono tirou de propósito).
- Overlay / pdf-lib em cima do arquivo do Obra Prima.
- Rodapé “FrotaHub · gerado em” na OC.
- Colunas novas no banco para endereço/e-mail (de propósito: o PDF é a fonte).
- Apagar `POST /reparo` ou `JanelaReparoOC.tsx` sem conversar — mortos na UI, vivos na API.
- Commitar `OCs_Teste/`, diários, `main-1.tsx`, `tipos-1.ts`, `deteccao_test.go`, migração `045` — estavam soltos e **ficaram de fora** do `5890a9c`.

---

## 8. Git

```
5890a9c  adm: editor e gerador de PDF nas OCs rejeitadas.
         (12 arquivos, já em origin/main)
```

Branch de trabalho: `main`, em sync com `origin/main` no momento deste diário.
