# Formatação do menu de PCO — descritivo completo

Pedido do dono (12/09/2026): antes de mexer no menu de Notas Fiscais, ir capturar
e documentar exatamente como o menu de PCO é formatado hoje, para usar como
referência. Este documento foi feito lendo o código real (`Pco.tsx`,
`Painel.tsx`, `escuro.css`, `orcamentos.css`, `base.css`) e conferindo os
números batendo um espelho estático (mesmo HTML/CSS compilado) no navegador,
a 1512×900 — a mesma tela que roda em produção.

## 1. A casca (fora do controle de cada tela — é do `App.tsx`/`base.css`/`escuro.css`)

Toda tela de **menu** (não de lista) do FrotaHub roda em **tema escuro**
(`<div className="lay escura">`), decisão do dono de 25/08/2026: "Menu é lugar
de escolher; lista é lugar de ler."

```
.lay.escura            { height: 100vh; overflow: hidden; }      /* trava a altura na janela — zero scroll na casca */
.lay.escura .main      { min-height: 0; }
.lay.escura .content   { display:flex; flex-direction:column; min-height:0; overflow:hidden;
                          padding: var(--folga) var(--folga) var(--folga); }   /* --folga = clamp(3px, 0.5vh, 6px) */
```

Medido a 1512×900:

| peça | valor |
|---|---|
| barra lateral (`.side`) | 268px de largura, fixa |
| barra de topo (`.top`) | 56px de altura |
| padding do `.content` | **4,5px** nos três lados (topo/direita/baixo) — quase colado na borda, de propósito ("a casca escura respira de menos, não de mais") |
| altura disponível para os cartões | `900 − 56 − 4,5×2 ≈ 835px` |

Ou seja: **em telas de menu, os cartões vão do topo até o fim da janela**, sem
sobrar rodapé vazio nem precisar rolar — é a régua "MENU NUNCA GANHA BARRA DE
ROLAGEM" (comentário original do CSS).

## 2. `Pco.tsx` — o que a tela em si desenha

```tsx
<div className="orc-painel orc-painel--estreito orc-painel--3">
  <Painel etapas={montarEtapas(dados)} aoEscolher={abrir} />
</div>
```

- **Sem `<header className="hero">`** — PCO não repete o título "PCO" dentro da
  tela: ele já está na barra de cima (migalha + título). O painel de cartões é
  o ÚNICO filho de `.content`.
- `orc-painel--estreito` + `orc-painel--3`: as duas classes que dizem "este é
  um painel de 3 cartões, com teto de largura" (mais abaixo).
- `<Painel>` (componente compartilhado, `componentes/Painel.tsx`) recebe uma
  lista de **3 etapas** (`montarEtapas`): Pendentes de envio, Enviados,
  Excluídas/Substituídas — cada uma com `numero` (contador real, vindo do
  motor), o que ativa o modo "com contador" do componente (não o modo
  `simples` de navegação pura).

## 3. A grade de cartões (`.pn` / `.pn-col`) — `escuro.css`

```css
.pn {
  display: flex;
  gap: 14px;
  min-height: 0;
  flex: 1;          /* estica para preencher toda a altura que sobrou do .content */
  overflow: hidden;  /* nunca cria scroll — quem encolhe é a prévia de dentro do cartão */
}
```

O teto de largura (`orc-painel--estreito.orc-painel--N`) é o que faz os
cartões **não esticarem a linha inteira**, mesmo tendo `flex:1 1 0`:

```css
.orc-painel--estreito.orc-painel--3 .pn-col {
  flex: 1 1 0;
  max-width: calc((100% - 28px) / 4);   /* "teto de 1,5× sobre a metade da fatia igual" */
}
```

Medido de verdade (3 cartões, container de 1231px de largura disponível):

| | valor medido |
|---|---|
| largura de cada cartão | **300,75px** (bate exato com `(1231-28)/4`) |
| vão entre cartões | **14px** (`gap`) |
| espaço sobrando à direita, sem cartão nenhum | **~301px** |
| altura de cada cartão | **835px — do topo até a base da tela**, igual para os três |

**Isso é proposital, não é bug**: o teto existe porque, em Compras, o cartão
"OCs Inseridas" abre em 4 sub-cartões ao passar o mouse (`filhos`) — o teto
reserva visualmente o espaço de um "quarto cartão" mesmo quando ele não existe
(caso do PCO, que não tem nenhum cartão com `filhos`). PCO usa a MESMA classe
`orc-painel--3` só para manter a largura idêntica à de Compras — não porque
PCO precise da reserva por si.

## 4. O cartão (`.pn-col`) por dentro

```css
.pn-col {
  flex: 1 1 0; min-width: 0;
  display: flex; flex-direction: column;
  border-radius: var(--r-l);                 /* cantos bem arredondados */
  border: 1px solid var(--d-line);           /* #2F2728 — quase invisível, some no fundo escuro */
  background: linear-gradient(180deg, var(--d-card2) 0%, var(--d-card) 46%, #1A1617 100%);
  box-shadow: var(--sh-escura-2);
  overflow: hidden;
  transition: flex-grow .2s, border-color .2s, box-shadow .2s, transform .2s;
}
.pn-col::before { /* faixa colorida de 3px no topo do cartão */
  content:''; position:absolute; inset:0 0 auto 0; height:3px;
  background: var(--faixa, var(--d-line2));  /* vermelho quando "viva", cinza (--faixa custom) senão */
}
.pn-col:hover, .pn-col:focus-visible {
  border-color: var(--d-line2);
  box-shadow: var(--sh-escura-3);            /* sombra mais funda */
  transform: translateY(-2px);               /* sobe 2px */
}
.pn-col:hover::after { opacity: 1; }         /* brilho vermelho radial sutil vindo do topo */
```

`.pn-corpo` (o `<button>` que ocupa o cartão inteiro):
`padding: 22px 18px 20px`, `display:flex; flex-direction:column; flex:1` — ele
também estica para a altura toda do cartão, então o rodapé (rótulo + seta)
fica sempre colado na base, e o meio vazio (quando não há prévia) é isso mesmo:
vazio, respirando.

Dentro do corpo, de cima para baixo:

1. **`.pn-selo`** — quadrado de 44×44px, cantos de 11px, com o ícone SVG
   (21×21px) centralizado. Cinza por padrão (`--d-card3`/`--d-txt2`); quando o
   cartão está **"viva"** (contador > 0), vira vermelho-marca
   (`--red-lift` no ícone, fundo `rgba(209,73,76,.10)`, borda
   `rgba(209,73,76,.35)`).
2. **`.pn-medida`** (margem de 26px acima) — o número grande:
   `font-size:46px; font-weight:700` em cinza (`--d-mut`) quando zerado, branco
   (`--d-txt`) quando "viva"; embaixo, o rótulo em versalete pequeno
   (`ORDENS`), cinza normal / **vermelho** quando "viva".
3. **`.pn-previa`** (opcional — só quando a etapa tem prévia) — até 9 linhas
   finas, separadas por friso quase invisível, com o texto à esquerda
   (elipse se não couber) e um valor à direita em cinza claro. Cresce para
   ocupar o espaço do meio do cartão (`flex:1`); quando não há prévia
   nenhuma, o espaço fica só vazio mesmo (`.pn-sem-previa`, sem friso de
   cima).
4. **`h3`** (o título) — 15,5px, peso 650, branco.
5. **`.pn-rodape`** — friso fino em cima, texto pequeno (cinza) à esquerda e
   uma seta `→` à direita — a única pista de "isto é clicável".

## 5. Comparação com o menu "simples" (sem contador — ex.: Compras/PCO/Notas
fiscais vistos como cartões dentro de **Administrativo**, no print que o dono
mandou)

Quando NENHUMA etapa tem `numero` nem `filhos`, o `<Painel>` entra sozinho no
modo `simples` (`Painel.tsx`, variável `simples`):

```css
.pn.simples { justify-content: flex-start; }
.pn.simples .pn-col { flex: 0 1 300px; max-width: 300px; }  /* largura fixa, não estica */
.pn.simples h3 { margin-top: 22px; }
.pn-espaco { flex: 1; min-height: 0; }   /* empurra o rodapé pra baixo, sem número no lugar dele */
```

Isto é o que aparece no print do dono para "Administrativo" (Compras / PCO /
Notas fiscais): cartões de **300px fixos**, sem número grande, sem prévia —
só ícone, título e seta. É um MENU DE NAVEGAÇÃO puro (decide para onde ir),
não um painel de trabalho (não mostra contador de nada).

**A diferença entre os dois modos não é uma escolha de estilo — é o que a
etapa CARREGA**: dar `numero` a uma etapa liga o modo contador (visual do
PCO); não dar liga o modo simples (visual do print de Administrativo).
`Pco.tsx`/`Compras.tsx` sempre passam `numero` real (vindo do motor); por
isso sempre aparecem no modo contador quando abertos.

## 6. Resumo para decidir Notas Fiscais

Hoje, `NotasFiscais.tsx` já usa o MESMO modo contador do PCO (número grande,
ícone, rodapé) — a única diferença real depois do ajuste da vez passada é que
os cartões **não têm o teto de largura `orc-painel--N`** (eu tirei de
propósito, porque a classe existente só cobre 2 ou 3 cartões, e Notas Fiscais
pode mostrar 1, 3 ou 4 dependendo da rotina de quem está logado — com o teto
fixo em 3, um usuário com acesso aos 4 cartões via transbordar ~14px pra
fora da tela).

Duas perguntas concretas para fechar isso:

1. **Quer o vão à direita de volta** (cartões com teto de largura, parando
   em ~300px cada, do jeito que Compras/PCO fazem) — mesmo sabendo que, com
   4 cartões em vez de 3, ou o teto precisa de uma conta nova (`--4`) ou vai
   sobrar/faltar um pouco de espaço à direita?
2. Ou prefere manter o que está hoje — **cartões que sempre preenchem a
   linha inteira**, sem vão à direita, se ajustando sozinhos a 1/3/4
   cartões conforme a rotina de quem está logado?

Sem essas duas respostas eu não mexo em mais nada — só descrevi o que existe.
