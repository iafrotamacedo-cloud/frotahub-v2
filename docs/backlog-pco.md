# Backlog — a coluna PCO do relatório mensal

> Aberto em 27/08/2026, na construção do **Relatório mensal** (A receber).
> Decisão do dono, na palavra dele: *"a coluna PCO do modelo sera uma variavel
> que usaremos nos proximos passos, mas hoje, por enquanto, nao eh util, coloca
> no backlog"*.

---

## O que existe hoje

A coluna **PCO** é a oitava do modelo que vai ao cliente, e ela **sai vazia** —
no lugar certo, com a largura certa, sem conteúdo.

Ela não foi omitida de propósito: tirar a coluna faria a planilha deixar de casar
com o modelo do cliente, que provavelmente é lido por PROCV do outro lado. Quando
o PCO chegar, quem recebe não precisa mudar nada na leitura dele.

## O que é o PCO

**Pedido / Ordem de Compra** emitido pelo cliente. No fluxo dele, uma célula
loja × conta do mês vira **um PCO**, e o PCO vira **uma nota fiscal** nossa.

O sistema já conhece esse conceito em dois lugares:

- a aba **A faturar** agrupa exatamente em células loja × conta — *"uma célula é
  um PCO dele, que vira uma nota"* (comentário no topo de `Faturamento.tsx`);
- a tabela `faturas` já tem onde anotar o número do PCO e o da nota, na aba
  **Faturas**.

## O que falta decidir

1. **De onde vem o número.** O cliente emite e informa (a gente digita), ou ele
   volta num arquivo/integração?
2. **Quando ele existe.** O PCO nasce *depois* de o cliente receber esta
   planilha — então na primeira extração a coluna está necessariamente vazia. A
   pergunta é se existe uma **segunda extração**, já com os números, ou se o PCO
   só aparece na fatura.
3. **Um por linha ou um por célula.** Se um PCO cobre uma loja × conta inteira,
   todas as linhas daquela célula repetem o mesmo número — e aí a coluna é
   derivada do agrupamento, não digitada linha a linha.

## Onde mexer quando chegar a hora

| Peça | O que muda |
|---|---|
| `db/migrations/…` | onde o número do PCO se liga ao orçamento — provavelmente via `faturas`, que já existe |
| `interno/modulos/orcamentos/relatorio_mensal.go` | a última coluna deixa de ser `""` |
| `orcamentos_a_cobrar` (migração 041) | passa a trazer o PCO junto |
| `TestOModeloDoClienteTemAsOitoColunasNaOrdem` | continua valendo — a coluna já está lá, só muda o conteúdo |

Nenhuma dessas mudanças mexe na ordem nem nos nomes das colunas. **O modelo do
cliente não muda quando o PCO chegar** — foi para isso que a coluna já nasceu.
