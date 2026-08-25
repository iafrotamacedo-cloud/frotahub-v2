# Contrato da API do Trílogo — lançamento de custos

Levantado em **25/08/2026** por captura de rede na tela real, ticket 130998
(LOJA 20 - RUI BARBOSA, conta Frota Instalações). Dois custos foram inseridos e
os dois foram excluídos; o ticket voltou ao estado original (1 custo, R$ 66,96).

Este documento existe porque a alternativa era escrever `lancar.go` no escuro e
descobrir o formato errando em produção do cliente.

## Bases

| host | para quê |
|---|---|
| `https://web.api.trilogo.app/api/` | dados (custos, tickets, cotações) |
| `https://upload.api.trilogo.app/` | arquivos |

Autenticação: header `Authorization` (o token vive na sessão do navegador).
Acompanham `X-Location` e `Accept-Language`.

## 1. Subir o arquivo

```
POST https://upload.api.trilogo.app/upload
Content-Type: multipart/form-data
campo: files            (o arquivo)

200 -> [{"filename":"NF_19702.pdf",
         "permalink":"https://s3.amazonaws.com/files.trilogo.app/grupos/361/<uuid>/NF_19702.pdf"}]
```

O `permalink` é público (S3, sem assinatura). O `361` é o grupo do cliente.

## 2. Criar o custo

```
POST https://web.api.trilogo.app/api/Ticket/CreateTicketCost
Content-Type: application/json

{ "TicketId": 130998,
  "Type": 1,
  "TotalValue": 950,
  "ProductCost": 950,        // Type 1
  "ServiceCost": null,       // Type 2 usa este
  "ShippingCost": 0,
  "DocumentNumber": "130998",
  "IssueDate": "2026-08-03",
  "DueDate": null,
  "Comment": "...",
  "CompanyId": 35,
  "UploadedFiles": [],
  "UploadedInvoiceFiles": [{ "filename": "...", "permalink": "..." }] }
```

### Tabelas de código

| `Type` | significado | campo de valor |
|---|---|---|
| 1 | Materiais | `ProductCost` |
| 2 | Mão de obra | `ServiceCost` |
| ? | Mão de obra e Materiais | (existe na tela; código não levantado) |

| `CompanyId` | conta |
|---|---|
| 35 | FROTA - INSTALAÇÕES |
| 72 | FROTA - CIVIL |

O id da Civil foi levantado em 25/08/2026 por dois caminhos independentes, que
concordaram: `session.sessionUser.supplier` no navegador logado como Frota Civil,
e o `company.id` de dois custos reais lidos por `GetTicketCosts`. Um caminho só
seria um palpite bem informado; dois que batem é levantamento.

`source: 1` = "custo adicional" (é o que a tela cria).

## 3. Listar os custos do ticket

```
GET https://web.api.trilogo.app/api/Ticket/GetTicketCosts/?ticketId=130998

200 -> [{ id, type, description, totalValue, serviceCost, productCost,
          shippingCost, documentNumber, issueDate, dueDate, comment,
          partName, quantity, source,
          company: { id, name },
          invoiceFiles: [...], files: [...] }]
```

É daqui que sai a soma para a conferência do teto de R$ 600 — **somando todos os
`type`**, não só o nosso.

## 4. Excluir o custo

```
DELETE https://web.api.trilogo.app/api/Ticket/DeleteTicketCost?ticketId=&ticketCostId=
200 -> {"id": 130998}
```

Modal da própria página, não `confirm()` nativo.

## 5. Leitura de nota fiscal (existe, e NÃO vamos usar)

```
POST https://upload.api.trilogo.app/extract-invoice-data
campo: file
```

Devolve a DANFE estruturada: `ChaveAcesso` (44 dígitos), `Emitente`,
`Destinatario`, `Produtos[]` (código, descrição, quantidade, valor unitário,
total), `Totais` (ValorNota, ValorMateriais, ValorMaoDeObra, ValorFrete) e
`InformacoesComplementares` — onde vêm o DAV e o número do ticket.

Lê PDF **escaneado sem camada de texto** (conferido: as duas notas de teste são
imagem JPEG 300 dpi, `pdftotext` não extrai um caractere).

**Decisão do dono (25/08/2026): não usamos. A leitura é nossa.**
Registrado aqui só para não redescobrirem depois e acharem que foi esquecimento.

## O que este levantamento provou

1. **O teto de R$ 600 é nosso, não deles.** O ticket foi a R$ 2.442,26 sem um
   aviso. Se a nossa regra falhar, ninguém falha por nós.
2. **`IssueDate` sai um dia atrás.** A leitura devolveu `2026-08-04T00:00:00Z` e
   o formulário enviou `"2026-08-03"` — conversão para fuso local comendo o dia.
   Nosso lançamento manda a data como veio. É o mesmo defeito das datas do
   Excel (P-34).
3. **O `permalink` fecha o desenho do `lancar.go`**: subir arquivo e criar custo
   são duas chamadas, nunca uma.

## O que ainda falta levantar

- O código de `Type` para "Mão de obra e Materiais".
- Se existe endpoint de edição (`UpdateTicketCost`) — a tela tem "EDITAR".
