# Regras e parâmetros — leitura da OC e envio do PCO

> Documento de referência, gerado em 11/09/2026, para testar/ajustar a lógica
> à parte (Composer 2.5 ou outro ambiente) sem mexer no FrotaHub de verdade.
> Descreve o que o código faz HOJE, arquivo por arquivo. Se você mudar algo
> aqui e quiser levar para o sistema, quem decide e aplica no repositório sou
> eu — este `.md` é só leitura/estudo.

---

## 1. Visão geral do fluxo

```
PDF da OC (Obra Prima)
   │
   ▼
INSERIR  (status = inserido)
   │  botão "ler" / lote
   ▼
LER      (status = lendo → lido | falhou)
   │
   ├── lido    → aparece em "Processadas" (para sempre) E em "PCO > Pendentes de envio"
   └── falhou  → aparece em "Rejeitadas", com o motivo
   
"Pendentes de envio" → botão enviar (um ou todos) OU robô às 20h (Fortaleza)
   │
   ▼
pco_enviado_em preenchido → some de "Pendentes", aparece em "Enviados"
   (a OC continua em "Processadas" — nunca sai de lá)
```

**A ideia central (pedido explícito do dono, 11/09/2026):** nada se move de
tabela — tudo é a MESMA linha de `ordens_compra`, e cada lista é só um filtro
diferente sobre `status` e `pco_enviado_em`. "Processadas" é a planilha de
controle permanente do comprador: uma OC nunca sai de lá (mesmo cancelada,
um dia, ela ficaria lá com uma marca — funcionalidade futura, ainda não
construída).

---

## 2. O formato de entrada — PDF do Obra Prima

- Sempre PDF **digital** (não escaneado — texto real na camada do PDF).
- Lido com `pdftotext -layout -enc UTF-8 -eol unix` — **o poppler**, não o
  xpdf. São dois programas diferentes com o mesmo nome de comando; só o
  poppler (`apk add poppler-utils`, o que roda em produção) preserva as
  colunas da tabela corretamente. Testar com o pdftotext errado (o que vem
  solto no Windows, por exemplo) produz uma tabela de itens embaralhada.
- PDF de várias páginas: o Obra Prima repete o cabeçalho inteiro
  (empresa + "ORDEM DE COMPRA `<numero>`" + a linha "N. Item ... Total (R$)")
  no topo de cada página nova. A leitura remove esse bloco automaticamente
  antes de separar os itens (ver `interno/modulos/administrativo/leitura.go`,
  função `extrairItens`).

### 2.1 Os blocos do documento, na ordem em que aparecem

```
FROTA MACEDO ENGENHARIA LTDA                                    <data impressão>
...endereço/CNPJ da Frota Macedo...

ORDEM DE COMPRA <numero>
<centro de custo>

DADOS DA ORDEM DE COMPRA      <numero>

Data: <dd/mm/aaaa>                    Previsão da entrega: <dd/mm/aaaa>
Cond. pgto.: <texto>                  Forma pgto.: <texto>
Observação:
                                       Comprador: <nome de quem cotou>
RESPONSÁVEL PELA COMPRA                Email: ...
Nome: FROTA MACEDO ENGENHARIA LTDA

DADOS DO FATURAMENTO
Nome:  <nome do cliente final>        Endereço: ...
CNPJ:  <cnpj do cliente final>

DADOS DO FORNECEDOR
Nome:  <razão social do fornecedor>   Endereço: ...
CNPJ:  <cnpj do fornecedor>

OBRA/CENTRO DE CUSTO: <texto>                                    CNO:

N. Item                    Qtd.   Unit. (R$)   Subtotal (R$)   Desc. (R$)   Total (R$)
1 <descrição>               ...        ...           ...            ...        ...
2 <descrição>               ...        ...           ...            ...        ...
...
                            Subtotal              <subtotal>   <desconto>   <total>
                            Frete                                            <frete>
                            Total                                          <total>
```

---

## 3. Campos extraídos (`leitura.go`, struct `Extraida`)

| Campo Go | Regra de extração | Observação |
|---|---|---|
| `Numero` | `^DADOS DA ORDEM DE COMPRA\s+(\d+)` | Obrigatório — sem ele, a leitura inteira é recusada ("este PDF parece não ser uma OC do Obra Prima") |
| `Data` | `Data:\s*(\d{2}/\d{2}/\d{4})` | Convertida para ISO (`aaaa-mm-dd`) |
| `PrevisaoEntrega` | `Previsão da entrega:\s*(\d{2}/\d{2}/\d{4})` | idem |
| `CondPgto` | `Cond\.\s*pgto\.:\s*(...)\s{2,}` | Precisa de 2+ espaços depois do valor (a coluna "Forma pgto." vem em seguida, na mesma linha) |
| `FormaPgto` | `Forma pgto\.:\s*(...)\s*$` | Até o fim da linha |
| `CompradorInterno` | `Comprador:\s*(...)\s*$` | Quem cotou (funcionário) — texto livre, só registro |
| `ObraCentroCusto` | `^OBRA/CENTRO DE CUSTO:\s*(...)\s{2,}` | Usado para agrupar o zip do PCO por pasta |
| `CompradorNome` | `Nome:` dentro do bloco **DADOS DO FATURAMENTO** | Nome do cliente final (a loja) |
| `CompradorCNPJ` | `CNPJ:` dentro do mesmo bloco | Só dígitos — é o CNPJ que decide o filtro 2 |
| `FornecedorNome` | `Nome:` dentro do bloco **DADOS DO FORNECEDOR** | |
| `FornecedorCNPJ` | `CNPJ:` dentro do mesmo bloco | Só dígitos — decide o filtro 1 |
| `Subtotal`/`Desconto`/`Total` | linha `Subtotal <v1> <v2> <v3>` no rodapé | `v1`=subtotal, `v2`=desconto, `v3`=total (repetido) |
| `Frete` | linha `Frete <v>` no rodapé | |
| `Itens[]` | tabela entre `N. Item` e `Subtotal` | ver §3.1 |

**Por que "Nome:"/"CNPJ:" são lidos POR BLOCO, e não do texto inteiro:** os
dois rótulos aparecem **três vezes** no documento (Responsável pela compra,
Faturamento, Fornecedor). Recortar o texto entre "DADOS DO FATURAMENTO" e
"DADOS DO FORNECEDOR" (e depois até "OBRA/CENTRO DE CUSTO") garante que cada
"Nome:"/"CNPJ:" lido é do bloco certo.

### 3.1 Itens (`ItemExtraido`)

Cada linha de item precisa estar **inteira numa única linha de texto**, no
formato:

```
<numero sequencial> <descrição> <qtd>,<dd> <unitário>,<dd> <subtotal>,<dd> <desconto>,<dd> <total>,<dd>
```

- O número tem que ser **sequencial** (1, 2, 3, ...) — se o próximo número
  não bater, a linha vira continuação da descrição do item anterior, em vez
  de item novo (proteção contra confundir "13" (item de verdade) com "1.3"
  em alguma descrição).
- Uma linha que não bate esse padrão (ex.: descrição que quebrou em duas
  linhas no PDF) é **anexada à descrição do item anterior** — nunca derruba
  a leitura inteira.
- A unidade de medida ("UN", "Unidades", "KG"...) pode aparecer sozinha numa
  linha de continuação, ou grudada no fim de uma linha de descrição — lista
  fechada de unidades reconhecidas (ver `padraoDeUnidades` no código).
- Números em formato brasileiro: `.` milhar, `,` decimal (ex.: `1.234,56`).

---

## 4. Os dois filtros de negócio (`MotivosDeRejeicao`)

**Os dois são checados sempre — uma OC pode falhar nos dois ao mesmo
tempo, e os dois motivos aparecem juntos.**

1. **Fornecedor precisa ter nome E CNPJ.**
   Falha se `FornecedorNome` ou `FornecedorCNPJ` vierem vazios.
   Motivo: `"não achei o nome e o CNPJ do fornecedor nesta OC"`

2. **CNPJ de faturamento precisa começar com `03720882`.**
   (constante `CNPJRaizPermitida` — é a raiz do CNPJ do grupo Mercadinhos
   São Luiz / Distribuidora de Alimentos Fartura.)
   - Se `CompradorCNPJ` vier vazio: `"não achei o CNPJ de faturamento
     (\"DADOS DO FATURAMENTO\") nesta OC"`
   - Se vier preenchido mas não começar com `03720882`: `"o CNPJ de
     faturamento (<valor>) não começa com 03720882 — esta OC não é para a
     Frota Macedo faturar"`

**Resultado:**
- **Zero motivos** → `status = lido` (Processada).
- **Um ou mais motivos** → `status = falhou`, `erro_leitura` = os motivos
  juntados com `"; "` (Rejeitada).

---

## 5. Máquina de status da OC (`ordens_compra.status`)

```
inserido ──(clique "ler")──► lendo ──► lido    (passou nos 2 filtros)
                                   └──► falhou  (falhou em 1+ filtro, ou erro de leitura)

falhou ──(clique "ler de novo")──► lendo ──► lido | falhou (repete)
```

- **Trava otimista**: só quem consegue trocar o status de `inserido`/`falhou`
  para `lendo` (via `UPDATE ... WHERE status IN ('inserido','falhou')`)
  ganha o direito de ler — evita duas leituras simultâneas da mesma OC.
- Se o `pdftotext` não estiver instalado no servidor (falha de
  infraestrutura, não do PDF), a OC **volta** para `inserido` — nunca vira
  `falhou` por culpa do servidor.
- Cada leitura grava rastro na tabela `historico` (ação `ler_ordem_compra`
  ou `rejeitar_ordem_compra`).

---

## 6. As cinco vistas (filtro SQL exato, `filtroDasOrdens` em `ordens.go`)

| Vista | Filtro | Onde aparece |
|---|---|---|
| `fila` | `status IN ('inserido','lendo')` | Compras › Inserir OC |
| `processadas` | `status = 'lido'` | Compras › OCs Inseridas › Processadas |
| `rejeitadas` | `status = 'falhou'` | Compras › OCs Inseridas › Rejeitadas |
| `pco-pendentes` | `status = 'lido' AND pco_enviado_em IS NULL` | PCO › Pendentes de envio |
| `pco-enviados` | `status = 'lido' AND pco_enviado_em IS NOT NULL` | PCO › Enviados |

Note que **"processadas" não depende de `pco_enviado_em`** — é por isso que
uma OC enviada continua aparecendo em "Processadas" para sempre, mas some de
"Pendentes de envio" assim que é enviada.

---

## 7. O envio do PCO (`pco_enviar.go`)

### 7.1 Quem pode disparar

Duas portas, mesma rota (`POST /administrativo/compras/pco/enviar` — geral,
ou `.../ordens/{id}/enviar` — uma só):

- **Robô** (GitHub Actions, cron `0 23 * * *` UTC = 20h Fortaleza, todo dia),
  autenticado por um segredo compartilhado (`X-Robot-Key`).
- **Pessoa**, com a rotina de permissão `COMPRAS_PCO_ENVIAR` (separada de
  `COMPRAS_PCO_DESTINATARIOS`, que só permite editar a lista de e-mails —
  pedido explícito: as duas ações têm permissões diferentes).

Botão manual: **um por OC** (na linha da tabela) e **um geral** ("Enviar
tudo (N)", no topo da lista de Pendentes) — os dois chamam a mesma rota.

### 7.2 Nunca envia vazio

Se não houver nenhuma OC pendente (ou se, entre a lista e a trava de baixo,
outra chamada já levou todas), a rota responde `{"enviado": false, "motivo":
"..."}` e **não chama o Brevo**. Isso vale tanto para o robô quanto para o
botão.

### 7.3 A trava (evita e-mail duplicado)

Antes de montar o e-mail, a rota tenta um `UPDATE` condicional:

```sql
UPDATE ordens_compra
   SET pco_enviado_em = now()
 WHERE id IN (<ids escolhidos>)
   AND pco_enviado_em IS NULL
```

Só as linhas que ESTAVAM `NULL` são marcadas e devolvidas — é essa lista
(não a lista original) que vira o e-mail. Se o Brevo falhar depois disso, a
marca é desfeita (`pco_enviado_em` volta para `NULL`) e a OC continua
pendente para a próxima tentativa.

### 7.4 O e-mail

- **Assunto:** `Ordens de Compra (PCO) - <dd-mm-aaaa> - Frota Macedo
  Engenharia`
- **Corpo:** HTML com uma tabela (Centro de custo | OC | Fornecedor | CNPJ |
  Valor), uma linha por OC, ordenada por centro de custo, com o total no
  rodapé.
- **Anexo:** sempre um `.zip` (`Pedidos_PCO_<data>.zip`), com uma pasta por
  centro de custo dentro, e cada PDF original renomeado `<numero>.pdf`.
- **Destinatários:** todo e-mail **ativo** na tabela `pco_destinatarios`
  entra no "Para" (não existe Cc separado — pedido explícito). Editável em
  **Configurações › PCO — Destinatários**, atrás da rotina
  `COMPRAS_PCO_DESTINATARIOS`. Nunca é apagado de vez — só
  ativado/desativado.
- **Assinatura:** por ora, fixo `"Frota Macedo Engenharia"` — vai virar
  configurável numa rodada futura.

---

## 8. As permissões novas (catálogo `rotinas`)

| Código | Para quê | Quem tem, por padrão |
|---|---|---|
| `COMPRAS_ORDENS_GERENCIAR` | Ver/inserir/ler OCs (migração 059) | CEO |
| `COMPRAS_PCO_DESTINATARIOS` | Editar a lista de e-mails do PCO | CEO |
| `COMPRAS_PCO_ENVIAR` | Apertar o botão de enviar (ou deixar o robô enviar em nome do login que a rotina alcançar) | CEO |

Quem for operar isso no dia a dia (ex.: o comprador) precisa ganhar essas
rotinas pela tela de Categorias/Acesso — elas nascem só para CEO.

---

## 9. Onde cada coisa mora no código (para referência)

| Assunto | Arquivo |
|---|---|
| Extração do PDF, os dois filtros | `baleryan/interno/modulos/administrativo/leitura.go` |
| Testes da extração (com um PDF real) | `baleryan/interno/modulos/administrativo/leitura_test.go` |
| Rotas de ler uma OC / lote | `baleryan/interno/modulos/administrativo/ler.go` |
| As 5 vistas, o painel de contadores | `baleryan/interno/modulos/administrativo/ordens.go` |
| Envio do PCO (Brevo, zip, trava) | `baleryan/interno/modulos/administrativo/pco_enviar.go` |
| Destinatários (CRUD) | `baleryan/interno/modulos/administrativo/destinatarios.go` |
| Cliente HTTP do Brevo | `baleryan/interno/brevo/brevo.go` |
| Tabelas/rotinas no banco | `db/migrations/059_*.sql`, `060_*.sql`, `061_*.sql` |
| Telas (Inserir OC, OCs Inseridas, PCO, Destinatários) | `web/src/telas/administrativo/*.tsx` |

---

## 10. Os 30 PDFs de teste desta pasta

Gerados por `_gerar_ocs.py` (reportlab), com o gabarito do que cada um
DEVERIA dar em `_gabarito.json`. Rodei a extração de verdade
(`administrativo.Ler`) contra os 30 antes de entregar — **30/30 bateram**
com o gabarito (status, número, fornecedor, CNPJ, quantidade de itens e
total).

**4 lojas usadas** (da lista que você mandou):
`DUNAS` (03.720.882/0002-39) · `CAMBEBA` (03.720.882/0007-43) ·
`RUI B.` (03.720.882/0024-44) · `VILLAS` (03.720.882/0039-20)

| Arquivo (padrão) | Quantidade | O que testa |
|---|---|---|
| `OC_*_VALIDA.pdf` | 15 | Passa nos dois filtros — vira Processada |
| `OC_*_BLOQ_FORNECEDOR_SEM_CNPJ.pdf` | 5 | Fornecedor sem CNPJ (comprador OK) |
| `OC_*_BLOQ_CNPJ_FATURAMENTO_ERRADO.pdf` | 5 | Comprador com CNPJ fora da raiz 03720882 (fornecedor OK) |
| `OC_*_BLOQ_FORNECEDOR_E_CNPJ.pdf` | 5 | Os dois problemas juntos — dois motivos na mesma rejeição |

Volume de itens variado por OC (de 1 a 14) — algumas geram PDF de 2 páginas,
para exercitar a limpeza do cabeçalho repetido (§2).

**Para testar sozinho:** `_gabarito.json` tem, por arquivo, o resultado
esperado (`PROCESSADA`/`REJEITADA`), o motivo, e todos os campos que a
leitura deveria extrair — dá para comparar direto contra o que o seu
ambiente novo (Composer 2.5) produzir, sem precisar abrir o FrotaHub.
