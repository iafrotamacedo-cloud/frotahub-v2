# A virada dos domínios

> Os dois sistemas **trocam de endereço**:
> `frotamacedo.com.br` passa a servir o FrotaHub **novo**, e o **antigo** vai
> para `novo.frotamacedo.com.br` — onde fica de parâmetro enquanto o resto é
> construído.
>
> Escrito em 28/08/2026, refeito para a troca simples.

---

## O que NÃO muda

Vale começar por aqui, porque encurta o medo.

- **O código do front novo não tem endereço nenhum escrito dentro.** Ele conhece
  o motor por uma variável da compilação (`VITE_MOTOR_URL`, que aponta para o
  Render) e o banco pela URL do Supabase. Trocar o domínio **não pede uma linha
  de código**.
- **O e-mail não é tocado.** Nada aqui mexe em DNS nem em MX. As 74 contas
  continuam onde estão.
- **O motor não muda de lugar**, e o banco de cada sistema continua o dele.
- **O `#` do endereço poupa a reescrita.** O caminho da tela mora depois do `#` e
  nunca chega ao servidor, então recarregar em qualquer tela funciona sem regra
  nenhuma no HostGator.

---

## Plano A — trocar a pasta que cada domínio serve

**Este é o caminho bom, e ele não move arquivo nenhum.**

Hoje:

```
frotamacedo.com.br        →  /home4/frotam86/public_html                  (antigo)
novo.frotamacedo.com.br   →  /home4/frotam86/novo.frotamacedo.com.br      (novo)
```

Depois:

```
frotamacedo.com.br        →  /home4/frotam86/novo.frotamacedo.com.br      (novo)
novo.frotamacedo.com.br   →  /home4/frotam86/public_html                  (antigo)
```

Os arquivos ficam exatamente onde estão. O que muda é **qual pasta cada endereço
abre** — no cPanel, em Domínios, no campo de diretório/raiz de cada um.

**Por que este caminho é melhor, e não é só preguiça:**

- **Não há janela sem site.** A troca vale no instante em que é salva.
- **Desfazer é trocar de volta**, em dois cliques. Nada foi apagado, nada foi
  copiado, não existe versão "quase certa" de nada.
- **A conta de FTP continua trancada onde está.** Ela enxerga a pasta do `novo.`
  — que agora é servida pelo domínio principal. A publicação automática continua
  funcionando **sem mexer em nada**, e continua **sem alcançar** a pasta do
  sistema antigo. A proteção sobrevive à virada, que no outro caminho ela não
  sobreviveria.

**O que conferir antes:** nem todo cPanel deixa mudar a raiz do **domínio
principal** — em alguns planos ela é fixa em `public_html`. Abra Domínios e veja
se o campo é editável. Se não for, vá para o Plano B.

**Ordem:**

1. Apontar `novo.frotamacedo.com.br` para `public_html`.
2. Apontar `frotamacedo.com.br` para `novo.frotamacedo.com.br`.
3. Conferir os dois endereços (a tabela no fim).

Fazer nessa ordem deixa o sistema antigo acessível pelos dois endereços por
alguns segundos, em vez de por nenhum.

---

## Plano B — se a raiz do domínio principal não for editável

Aí os arquivos precisam trocar de lugar, e a ordem é o que torna isto reversível.

### 1. Guardar o antigo

Copiar o conteúdo de `public_html` para `/home4/frotam86/antigo-guardado/`.
**Copiar, não mover.** Esta pasta é o caminho de volta, e ela não deve ser
apagada nas primeiras semanas.

### 2. Esvaziar o `public_html`

A partir daqui o endereço principal fica fora do ar até o passo 4. Deixe a aba
do GitHub aberta antes de começar.

### 3. Apontar a conta de FTP `deploy` para o `public_html`

cPanel → Contas de FTP → `deploy@frotamacedo.com.br` → alterar diretório.

> **Faça depois de esvaziar, e não antes.** Com a conta já apontando para um
> `public_html` cheio, uma publicação acidental passa por cima do sistema antigo.
> Com ele vazio, não há o que perder.
>
> **E saiba o que isto custa:** hoje essa conta **não alcança** o `public_html` —
> é uma proteção real, e no Plano B ela acaba. Sobra a trava da automação, que se
> recusa a enviar se a compilação não gerou o `index.html` (SKILL-PUBLICAR-01).

### 4. Publicar o novo pelo Actions

GitHub → Actions → *Publicar front* → **Run workflow**.

Não copie a pasta do `novo.` à mão: publicando pela automação, o que está no ar é
exatamente o que está no repositório, e fica o registro da publicação.

### 5. Pôr o antigo no `novo.`

Esvaziar `novo.frotamacedo.com.br/` e copiar para lá o conteúdo de
`antigo-guardado/`.

> Apague também o `.publicacao-estado.json` que ficou nessa pasta: é o controle
> da automação sobre o que já foi enviado, e ele não tem mais nada a ver com o
> que passa a morar ali.

---

## Depois da virada, nos dois planos

### O CORS dos dois motores

**Motor novo** (Render → `baleryan` → Environment → `CORS_ORIGENS`):

```
https://frotamacedo.com.br,https://www.frotamacedo.com.br
```

Hoje está em `*`, que libera o mundo inteiro — era pendência desde a Fase 2, e
este é o momento certo, porque agora o endereço é definitivo. Ponha os dois:
`www` e sem `www` são origens diferentes para o navegador.

**Motor antigo** (Render → `motor-orcamentos`): se o CORS dele estiver fechado
em `https://frotamacedo.com.br`, ele precisa passar a aceitar
`https://novo.frotamacedo.com.br` — senão o sistema antigo abre a tela e nenhuma
consulta funciona, que é o pior dos dois mundos.

### O Supabase do sistema antigo

Projeto `faalgfbugvekbuhhtatt` → Authentication → URL Configuration. Se o
`Site URL` ou os `Redirect URLs` apontarem para `https://frotamacedo.com.br`, o
login dele quebra no endereço novo.

### Quando ele for "só seu"

O antigo vai ficar de parâmetro por um tempo e depois deixa de ser de todo mundo.
O caminho mais simples para isso, quando chegar a hora, é **Proteção de
diretório** no cPanel, na pasta dele: o navegador passa a pedir usuário e senha
antes de qualquer coisa. É uma tranca a mais na frente do login que já existe, e
não exige tocar no sistema.

---

## Como saber que deu certo

| O quê | Onde | Esperado |
|---|---|---|
| Sistema novo abre | `https://frotamacedo.com.br` | tela de login do FrotaHub novo |
| Login funciona | idem | entra, e o painel de Orçamentos carrega os números |
| O motor responde | idem | abrir Notas e DAVs; se o CORS estiver errado, a tela avisa |
| Sistema antigo abre | `https://novo.frotamacedo.com.br` | o sistema de sempre, com login |
| O antigo funciona | idem | entrar e abrir uma tela que fale com o motor dele |
| Cache do navegador | numa máquina que já usava o site | abre o NOVO sem Ctrl+Shift+R |
| Publicação continua | um push que mexa em `web/` | Actions verde, e a mudança no ar |

**A linha do cache é a que costuma assustar.** Quem já abriu `frotamacedo.com.br`
tem o `index.html` do sistema antigo guardado no navegador, e continuaria vendo o
antigo num endereço que já não o serve. O `.htaccess` que vai junto com esta
virada resolve: ele diz ao servidor para nunca guardar o `index.html`, mantendo o
cache longo só nos arquivos que têm código no nome.

**Ele precisa estar no ar antes da virada** — ou seja, o push com o `.htaccess`
vem primeiro, e só depois a troca dos domínios.

---

## Se der errado

**Plano A:** trocar as duas raízes de volta. Segundos, e nada foi perdido.

**Plano B:** copiar `antigo-guardado/` de volta para `public_html`. É por isso que
o passo 1 é uma cópia, e é por isso que aquela pasta não se apaga cedo.
