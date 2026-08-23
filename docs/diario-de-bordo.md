# FrotaHub — Diário de Bordo

> Registro do projeto, organizado em **Fases** e, dentro delas, em **Steps**.
> Cada step lista o que foi definido, feito e criado — plataforma por plataforma.
> Quem ler só este arquivo sabe onde estamos, o que existe e qual é o próximo passo.
>
> Última atualização: **23/08/2026** · Rodada 17 · **Fases 0 e 1 concluídas**

---

## O que é o FrotaHub

O sistema interno da **Frota Macedo Engenharia**: um só lugar, com um só login, para as
rotinas dos quatro departamentos.

- **Administrativo** — envio de PCO, ciclo das notas fiscais, locações e compras.
- **Manutenção** — lista de chamados do Trílogo, orçamentos de material (leitura da nota,
  geração, lançamento), manutenção preventiva e financeiro do contrato.
- **Engenharia** — prontuário elétrico NR-10 e projetos.
- **SESMT** — formulários de APR e Permissão de Trabalho.

Mais **Auditoria** (o que foi comprado, por quem, e se era permitido) e **Configuração**
(logins, permissões, PIN, agendamentos).

Roda 100% na nuvem, em serviços de plano gratuito, com acesso por computador e celular.

---

## As regras de trabalho

1. **Explico antes, você autoriza, aí eu subo.** Nada entra no repositório sem o seu "pode".
2. **Nada de uma vez.** Sobe pouco a pouco, módulo a módulo, na ordem que você mandar.
3. **Amostra primeiro.** Antes de uma leva grande, os arquivos-chave vão no chat para você
   ver o estilo.
4. **Segredo não passa pelo chat.** Senha de banco, chave de serviço e chave secreta do R2
   vão direto no painel de cada serviço.
5. **Este diário é atualizado a cada rodada**, em Fases e Steps.
6. **Todo arquivo tem endereço e revisão.** "Está no chat", "está no repositório" e "está
   rodando" são três coisas diferentes — o inventário diz qual é qual.
7. **Assinatura do projeto:** Igor Tostes — NorthCore.

> Estas sete são as regras da **nossa colaboração** — como trabalhamos. As regras do
> **sistema** ficam no **Anexo A — Tenets** (declarados pelo dono, em cinco níveis) e no
> **Anexo B — Práticas de construção** (o padrão de quem escreve o código).

---
---

# FASE 0 — Fundação e alinhamento

**O que é.** Antes de escrever o sistema, montar e provar o terreno onde ele vai viver:
escolher cada peça da stack com um motivo declarado, criar e configurar todas as contas,
desenhar a arquitetura, e provar que o caminho entre o código e o ar funciona de ponta a
ponta. Nenhuma rotina de negócio é construída nesta fase.

**Por que existe.** Encanamento errado falha em silêncio e no pior momento. Provando o
caminho com uma página vazia, qualquer erro daqui pra frente só pode ser do sistema.

**Como sei que terminou.** Todas as contas criadas e testadas de verdade (não no papel), e
uma página publicando sozinha a cada push, sem ninguém copiar arquivo.

**Situação: CONCLUÍDA** em 23/08/2026.

---

## Step 1 — Definição da stack

Cada peça, para que serve, e por que foi escolhida.

**1. GitHub — onde o código mora**
Guarda todo o código-fonte e o histórico de quem mudou o quê. É também quem **compila e
publica** o front automaticamente, pelo GitHub Actions. Escolhido por já ser usado nos
robôs atuais e por ser gratuito.

**2. Go — a linguagem do motor**
O motor se chama **baleryan**. Go compila para um único arquivo executável que sobe em
milissegundos — importante porque o plano gratuito do Render hiberna o serviço, e o
primeiro acesso do dia acorda ele. Também acusa erro na compilação, antes de ir ao ar,
em vez de em produção.

**3. React + Vite + TypeScript — o front**
São 79 telas repetindo os mesmos padrões (tabela com filtro, formulário, modal, upload).
É o problema que componente resolve. TypeScript no front com Go no motor deixa os dois
lados tipados: campo trocado quebra a compilação, não a tela do usuário.

**4. Supabase — banco de dados e login**
Postgres gerenciado com sistema de login embutido. Um só login para o sistema inteiro,
sem autocadastro. Plano gratuito.

**5. Cloudflare R2 — os arquivos do dia a dia**
Guarda notas, orçamentos e ordens de compra. Escolhido por **não cobrar saída de dados** —
o PDF vai direto da Cloudflare para o navegador, por endereço temporário, sem passar pelo
motor. 10 GB grátis, sem prazo.

**6. Dropbox — o arquivo-mestre**
Continua guardando a cópia de segurança de tudo que já foi gerado, com lixeira e histórico
nativos. É a rede de proteção.

**7. Render — onde o motor roda**
Roda o `baleryan`. Único da lista que executa programa de verdade. Plano gratuito, região
**Ohio** — escolhida por ficar a ~12 ms do banco.

**8. HostGator — onde o front é servido**
Serve o site no domínio próprio da empresa. Substitui o Netlify: um endereço a mais na
casa e uma conta a menos para cuidar. **Não roda o motor** — hospedagem compartilhada
derruba programa que fica ligado sozinho.

**9. Cowork / Claude — quem escreve**
Escreve o código na pasta do computador; o push é do usuário.

---

## Step 2 — Configurações da stack

O que foi criado e configurado em cada plataforma.

### 2.1 GitHub

- **Repositório `iafrotamacedo-cloud/frotahub-v2`** criado vazio.
- **Visibilidade:** nasceu público → mudado para privado → **voltou a público**. Motivo da
  volta na seção de decisões (cota de Actions).
- **Identidade dos commits:** `Igor Tostes - NorthCore <ia.frotamacedo@gmail.com>`,
  configurada no Git do computador.
- **Commits até aqui:** diário de bordo · página de prova e configuração de compilação ·
  criação do workflow.
- **Workflow `.github/workflows/publicar-front.yml`** (rev 1) — compila o front e envia por
  FTPS ao HostGator a cada push que mexa em `web/`. Criado **pela interface do GitHub**,
  porque escrita remota em `.github/workflows/` é bloqueada por segurança.
- **Secrets criados** (Settings → Secrets and variables → Actions):
  `FTP_SERVIDOR` · `FTP_USUARIO` · `FTP_SENHA`.
- **Variable prevista:** `VITE_MOTOR_URL` — endereço do baleryan. Ainda vazia.
- **PAT (token de acesso):** foi criado e **descartado**. O ambiente do Claude bloqueia
  escrita em repositórios não autorizados, então o token não servia. Deve ser revogado.
- **Descoberta:** os robôs do Trílogo consomem a maior parte da cota mensal de Actions.

### 2.2 Supabase

- **Projeto `frotahub-v2`** — ref `hltcngamdqabqlocufrv`, região **us-east-1 (Virgínia)**,
  Postgres 17.6. Projeto dedicado, separado do que roda em produção.
- **Conector MCP ligado** — o Claude lê e escreve no banco direto. Ele enxerga os dois
  projetos da conta; **compromisso: não tocar no de produção** (`faalgfbugvekbuhhtatt`).
- **Estado inicial conferido:** schema `public` vazio, `auth` pronto com 0 usuários,
  0 migrations aplicadas.
- **Tabela de teste `builder_list`** criada para provar a conexão — colunas `id`, `builder`,
  `empresa`, `criado_em`, com segurança de linha ligada e uma linha:
  `Igor Tostes / NorthCore`. **Descartável**, some quando o usuário mandar.
- **Chaves:** a pública está registrada neste diário; a `service_role` e a senha do banco
  nunca passaram pela conversa — vão direto nas variáveis do Render.

### 2.3 Cloudflare R2

- **Conta** — Account ID `99f23938821d056868ecef92c08eed7f`.
- **Balde `frotahub`** criado: localização **Automatic**, que a Cloudflare resolveu para
  **Eastern North America** (ao lado do motor e do banco); **sem jurisdição**; classe
  **Standard**; **URL pública desabilitada** — todo acesso por endereço temporário assinado.
- **Regra de ciclo de vida `expurgo-lixeira`** — prefixo `_lixeira/`, apaga após **30 dias**.
  É a lixeira do sistema, executada pela Cloudflare mesmo com o motor fora do ar.
- **Regra `limpar-uploads-incompletos`** — aborta envios pela metade após 7 dias.
  **Redundante:** a Cloudflare já cria uma regra nativa igual. Pode ser apagada.
- **Token `frotahub-baleryan`** — Account API token, permissão **Object Read & Write**,
  **restrito ao balde `frotahub`**, sem expiração. Chave e segredo ficaram com o usuário.
- **CORS:** ainda não configurado. Espera o endereço definitivo do front.
- **Conector MCP ligado** — cria e lista baldes; **não** faz regra de ciclo de vida nem token.

### 2.4 HostGator

- **Plano P**, servidor `br1000`, IP `162.241.203.77`, domínio `frotamacedo.com.br`,
  vencimento 09/03/2027.
- **Subdomínio `novo.frotamacedo.com.br`** criado, apontando para
  `/home4/frotam86/novo.frotamacedo.com.br` — **fora do `public_html`**, para o sistema em
  construção não ficar acessível pelo endereço principal.
- **Conta de FTP `deploy@frotamacedo.com.br`** criada, **trancada nessa pasta**, cota
  ilimitada. Não alcança o `public_html` nem o FrotaHub que está no ar.
- **Certificado de segurança** emitido automaticamente — o endereço já responde em `https`.
- **Confirmado:** subdomínios são **ilimitados** no plano (o limite de "1 site" vale para
  domínios adicionais, não para subdomínios).
- **Endereço temporário** que o painel oferece: `http://br1000.teste.website/~frotam86`.
- **Existem** SSH, Tarefas Cron e Gerenciador de aplicações — mas **o motor não roda aqui**:
  hospedagem compartilhada derruba processo que fica ligado, e o gerenciador atende Node,
  Python e Ruby, não Go.
- **Alerta operacional:** disco em **85,7 GB de 100 GB**, com 74 contas de e-mail. Quando
  encher, e-mail para de ser recebido. Assunto separado, mas precisa de faxina.

### 2.5 Render

- **Workspace** "IA's workspace" (`tea-d9pnt4ht0dsc73cv3590`).
- **Serviço existente `motor-orcamentos`** — região **Ohio**, plano free, Docker, deploy
  automático a cada commit. É do sistema **atual**: não tocar.
- **Conector MCP ligado** — lista serviços, lê registros, publica.
- **Definido:** o `baleryan` nasce **em Ohio**, para ficar a ~12 ms do banco na Virgínia.
- Ainda **não criado**.

### 2.6 Dropbox

- Nada configurado ainda. Continua com o papel de **arquivo-mestre** na arquitetura.

### 2.7 Computador do usuário

- **Pasta `C:\Users\FROTAMACEDO\Desktop\FrotaHub-v2`** conectada ao Claude — é onde os
  arquivos são escritos antes do push.
- **Git 2.55.0.3** instalado (via `winget install --id Git.Git`).
- **Identidade global** configurada com nome e e-mail.
- **Fluxo de trabalho:** Claude escreve na pasta → usuário confere →
  `git add . && git commit -m "..." && git push`.

---

## Step 3 — Arquitetura

### Os seis princípios

**1. O banco é a única fonte da verdade.**
A fila de trabalho, o estado de cada orçamento e a localização de cada arquivo saem do
banco. Nenhuma rotina descobre o que fazer varrendo pasta: o arquivo é consequência do
registro, nunca o contrário.

**2. Cada regra existe uma vez só.**
A margem, o teto de lançamento, a duplicidade e o roteio moram numa pasta `dominio` —
código puro, sem banco, sem internet, sem nuvem. Por isso **dá para testar de verdade**.

**3. Arquivo é registro.**
Toda escrita em nuvem gera linha na tabela `arquivos`, com endereço, tamanho e impressão
digital. Achar um PDF é uma consulta, não uma busca por nome.

**4. Nada de estado na memória do servidor.**
Tarefa demorada é linha na tabela `jobs`. Se o servidor cair e voltar no meio, ela continua
de onde parou.

**5. Uma porta de entrada só.**
Um lugar recebe a credencial e diz **quem é**; outro diz **o que pode**, por rotina.

**6. Mudança de banco é migração numerada.**
Cada mudança é um arquivo com número, e o banco guarda quais já rodaram.

### As peças conversando

```
        NAVEGADOR                    NUVEM DO MOTOR              NUVENS DE APOIO
   ┌────────────────┐            ┌──────────────────┐         ┌──────────────────┐
   │  FrotaHub web  │            │    baleryan      │         │ Supabase         │
   │  (HostGator)   │───HTTPS───▶│    (Render)      │────────▶│ banco e login    │
   │                │            │                  │         └──────────────────┘
   │ React + Vite   │            │  Go, binário     │         ┌──────────────────┐
   │ TypeScript     │            │  único, Ohio     │────────▶│ Cloudflare R2    │
   │ 1 tela =       │◀──URL──────│                  │         │ arquivos vivos   │
   │ 1 arquivo      │  assinada  │                  │         └──────────────────┘
   └────────────────┘            │                  │         ┌──────────────────┐
                                 │                  │────────▶│ Dropbox          │
                                 │                  │         │ arquivo-mestre   │
                                 └────────┬─────────┘         └──────────────────┘
                                          │                   ┌──────────────────┐
                                          └─────────────────▶ │ Trílogo, Gemini, │
                                                              │ e-mail, GitHub   │
                                                              └──────────────────┘
```

**A seta da URL assinada:** o motor não serve arquivo. Ele entrega um endereço temporário e
o PDF vai direto da Cloudflare ao usuário. O servidor não vira gargalo e o R2 não cobra saída.

### O modelo de dados

| Tabela | O que guarda |
|---|---|
| `documentos` | A nota fiscal ou DAV lida: número, fornecedor, comprador, data, valor |
| `documento_itens` | Os itens dessa nota, um por linha |
| `orcamentos` | Ticket, loja, valor, se foi lançado, se é rateio, se extrapolou |
| `orcamento_itens` | O que está impresso no papel: preço da nota e preço cobrado lado a lado |
| `orcamento_documentos` | O vínculo entre os dois — é o que permite **uma nota atender vários tickets** |
| `arquivos` | Onde cada arquivo está, em que nuvem, com que tamanho e impressão digital |
| `jobs` | Tarefas demoradas e o progresso de cada uma |
| `parametros` | Margem, teto, limite de extrapolado — com histórico de quem mudou |
| `chamados` | O espelho da lista do Trílogo |
| `perfis`, `categorias`, `categoria_permissoes` | Quem entra e o que cada um pode |
| `log_atividades` | Tudo que foi feito, por quem e quando |

Duas escolhas que valem explicação. **Os itens são tabela, não bloco de texto** — é o que
permite a auditoria perguntar "quem comprou ferramenta este mês" com uma consulta simples.
E **documento e orçamento são coisas separadas**, ligadas por uma terceira tabela — o que
faz o rateio ser caso normal do modelo, não exceção.

### As regras que o banco impõe sozinho

- Nada é apagado de vez: `DELETE` bloqueado nas tabelas de registro; remover é marcar.
- Um documento é nota fiscal **ou** DAV, nunca os dois.
- A mesma nota não gera orçamento duas vezes.
- Números de sequência saem do banco — duas pessoas clicando junto não colidem.
- Cada campo de situação só aceita os valores da sua lista.

---

## Step 4 — As árvores

### 4.1 Árvore do repositório — **planejada**

> Só o que está marcado como existente foi criado. O resto é o desenho.

```
frotahub-v2/
├── README.md                 o mapa do projeto
├── .gitignore                ← EXISTE
├── docs/
│   ├── diario-de-bordo.md    ← EXISTE (este arquivo)
│   └── decisoes/             uma decisão por arquivo
│
├── .github/workflows/
│   └── publicar-front.yml    ← EXISTE (rev 1)
│
├── db/
│   ├── migrations/           001_..., 002_... numeradas, aplicadas em ordem
│   ├── seed/                 carga inicial: categorias, permissões, parâmetros
│   └── aplicar.py            confere o que já rodou antes de rodar
│
├── baleryan/                 ← o motor, em Go
│   ├── go.mod · Dockerfile
│   ├── cmd/baleryan/baleryan.go      o ponto de partida
│   └── interno/
│       ├── config/           variáveis de ambiente tipadas e validadas na subida
│       ├── seguranca/        quem é você (usuário, robô ou agendador)
│       ├── permissao/        o que você pode, por rotina, mais o PIN
│       ├── auditoria/        o log de atividades
│       ├── banco/            o cliente do Supabase
│       ├── arquivos/         R2, Dropbox e o roteador entre os dois
│       ├── dominio/          AS REGRAS: preço, teto, duplicidade, NF × DAV
│       ├── documentos/       geração de PDF e planilha
│       ├── servicos/         leitura de nota (OCR e IA), Trílogo, e-mail
│       ├── tarefas/          a fila de jobs
│       └── modulos/          uma pasta por módulo
│
├── web/                      ← React + Vite + TypeScript
│   ├── package.json          ← EXISTE
│   ├── package-lock.json     ← EXISTE
│   ├── vite.config.js        ← EXISTE (rev 1)
│   ├── index.html            ← EXISTE (rev 1, página de prova)
│   ├── public/logo.jpg       ← EXISTE
│   └── src/
│       ├── main.tsx · App.tsx        entrada e casca
│       ├── estilos/          a identidade da Frota Macedo
│       ├── api/              cliente do motor + tipos espelhando o Go
│       ├── sessao/           login, token, permissões
│       ├── componentes/      Tabela, Modal, Toast, Upload, KPI, Formulario
│       ├── menu/             a árvore + o filtro por permissão
│       └── telas/            1 PASTA POR MÓDULO, 1 arquivo por tela
│
└── robos/                    GitHub Actions: chamados, lançamento, manutenção
```

**A regra de ouro:** para mexer no cálculo do orçamento, abre
`baleryan/interno/dominio/preco.go`. Para mexer na tela de lançamento, abre
`web/src/telas/manutencao/LancarTrilogo.tsx`. Cada assunto tem um lugar, e só um.

### 4.2 Árvore de menus

**O FrotaHub novo não tem árvore de menus pré-definida.** Cada menu e cada rotina nascem
quando são construídos — nada é prometido antes de existir.

O que existe hoje está na Fase 1: dois menus, três itens, todos marcados como "em breve"
menos a tela inicial.

O menu do sistema antigo, com as suas 79 rotinas, está em `docs/referencia-sistema-antigo.md`.
Ele serve de **consulta**, não de plano: quando quisermos trazer alguma coisa de lá, a
decisão é explícita e vira rotina nova aqui.

---

## Step 5 — Prova do caminho de publicação

**O que foi feito.** Uma página mínima, sem função nenhuma, publicada pelo caminho completo
para provar o encanamento **antes** de o sistema existir.

**Arquivos criados:**

- `web/index.html` — a página de prova, com a identidade da casa.
- `web/package.json` · `web/package-lock.json` · `web/vite.config.js` — o mínimo para compilar.
- `web/public/logo.jpg` — o logo.
- `.gitignore` — impede `node_modules` e a pasta compilada de irem para o repositório.
- `.github/workflows/publicar-front.yml` — a automação.

**O que a automação faz:** acorda a cada push que mexa em `web/`; instala o Node; compila;
**confere se a compilação gerou mesmo o arquivo** (senão aborta, para nunca publicar vazio
por cima de tudo); e envia por FTPS só a pasta compilada.

**Resultado da primeira execução, medido no registro:**

```
Compilar               →  index.html (4 kB) + logo.jpg (3,8 kB)
Conferir se compilou   →  passou
Enviar por FTPS        →  primeira publicação, 2 arquivos, 7,81 kB
Tempo total            →  4,1 segundos (2,1 s só para conectar)
```

**Confirmado no ar:** `https://novo.frotamacedo.com.br` — com certificado de segurança já
emitido, redirecionando de `http` sozinho. O FrotaHub atual não foi tocado.

**Tropeços do caminho, registrados para não repetir:**

1. Escrita remota em `.github/workflows/` é **bloqueada** — o arquivo da automação precisa
   ser criado por uma pessoa, pela interface do GitHub.
2. A primeira execução falhou com *"the job was not started because recent account payments
   have failed or your spending limit needs to be increased"*. Não era o código: era a cota
   de Actions, que repositório **privado** consome e repositório **público** não. Resolvido
   voltando o repositório a público.
3. Aviso do GitHub: Node 20 está sendo descontinuado nos executores. Não quebra nada agora;
   corrigir na próxima revisão do workflow.

---

## Inventário — o que existe e em que revisão

**Como funciona a revisão.** Todo arquivo nasce em **rev 1** e sobe a cada mudança. O número
fica em dois lugares que têm que bater: um comentário na primeira linha do arquivo e esta
tabela. Se divergirem, vale o comentário dentro do arquivo.

| Valor de "Hospedado" | Significa |
|---|---|
| `repo` | Está no GitHub `frotahub-v2`. É a fonte. |
| `no ar` | Publicado e servindo em `novo.frotamacedo.com.br`. |
| `Render` | Compilado e rodando no motor. |
| `Supabase` | Migration **já aplicada** no banco — não basta estar no repo. |
| `chat` | Só entregue na conversa. **Não está hospedado.** |
| `Projeto` | No Projeto do Claude, visível em qualquer conversa. |

| Arquivo | Hospedado | Rev | O que é |
|---|---|---|---|
| `docs/diario-de-bordo.md` | repo · Projeto | 6 | Este arquivo |
| `.gitignore` | repo | 1 | O que não entra no controle de versão |
| `.github/workflows/publicar-front.yml` | repo | 1 | Compila e publica o front |
| `web/index.html` | repo · **no ar** | 1 | Página de prova (será substituída) |
| `web/package.json` | repo | 1 | Dependências do front |
| `web/package-lock.json` | repo | 1 | Versões travadas |
| `web/vite.config.js` | repo | 1 | Configuração da compilação |
| `web/public/logo.jpg` | repo · **no ar** | 1 | Logo da empresa |

**No banco:** só a tabela de teste `builder_list`. Nenhuma migration aplicada.
**No R2:** balde criado e vazio.
**No Render:** nada do FrotaHub 2 ainda.

---

## Decisões da Fase 0

| # | Decisão | Por quê |
|---|---|---|
| 1 | Repositório `frotahub-v2`, **público** | Começar limpo. Público porque Actions em repositório privado consome cota mensal, e a conta já estava perto do limite por causa dos robôs do Trílogo. Nenhuma senha mora no repositório. |
| 2 | Projeto Supabase dedicado | Isolamento total. Custo aceito: os logins são criados do zero na virada. |
| 3 | Tabelas no schema `public` | É onde o Supabase espera encontrá-las. Um passo a menos. |
| 4 | Motor em Go, tudo chamado **`baleryan`** | Binário único que sobe em milissegundos; erro aparece na compilação. Nome único da pasta ao serviço, sem tradução no meio. |
| 5 | Front em React + Vite + TypeScript | 79 telas repetindo padrões. Tipagem casando com o Go. |
| 6 | Visual próprio, sem biblioteca de componentes | Preserva a identidade já aprovada: sidebar escura, `#A11F22`, Inter, ícones SVG. |
| 7 | Arquivos vivos no R2; mestre no Dropbox | R2 não cobra saída e serve direto ao navegador; Dropbox dá lixeira e histórico como rede de segurança. |
| 8 | **Lixeira própria** no R2 | Prefixo `_lixeira/` + regra de ciclo de vida nativa (30 dias), com restauração comandada pelo banco. Fica melhor que a do Dropbox: a exclusão guarda autor e motivo. |
| 9 | Front no HostGator, motor no Render | Compartilhada não roda programa ligado. O Netlify sai de cena. |
| 10 | Render em **Ohio** | ~12 ms até o banco na Virgínia, contra ~120 ms se ficasse longe. |
| 11 | Publicação por GitHub Actions via FTPS | Compila na nuvem; o usuário não instala nada. Conta de FTP trancada na pasta do subdomínio. |
| 12 | **Margem arredondada linha a linha** | O unitário arredondado é o número impresso; o total tem que ser a soma exata do que o cliente vê. *(aguardando aval)* |
| 13 | Decimal, não float | Meio centavo sobe, sempre. |
| 14 | Parâmetros de negócio no banco | Margem, teto e limite com histórico de quem mudou. |

---

## Pendências

| O quê | Para quê | Situação |
|---|---|---|
| **O `.docx` do orçamento é necessário?** | Se o PDF basta, o sistema fica bem mais simples — Go não tem biblioteca boa para Word. | aguardando |
| **Aval do arredondamento linha a linha** | Muda centavo nos orçamentos novos. | aguardando |
| **Matriz de permissões** | Existe um rascunho de quem alcança o quê nas sete categorias. É decisão do usuário, não minha. | aguardando |
| Revogar o PAT do GitHub | Não serve para nada e ficou no histórico da conversa. | a fazer |
| CORS no R2 | Para o navegador enviar arquivo direto ao balde. | espera o endereço final do front |
| Apagar `builder_list` | Tabela de teste. | quando o usuário mandar |
| Apagar a regra redundante do R2 | `limpar-uploads-incompletos` duplica a nativa. | opcional |
| Faxina no disco do HostGator | 85,7 GB de 100 GB. Quando encher, e-mail para de entrar. | assunto separado |
| Olhar o consumo dos robôs do Trílogo | São eles que comem a cota de Actions. | assunto separado |
| Tamanho da pasta FROTAHUB no Dropbox | Para saber se cabe nos 10 GB grátis do R2. | quando puder |

---

## Próximo passo

A definir com o usuário. O sistema tem casca, login e os dois primeiros menus no ar; o
motor `baleryan` ainda não foi construído, e nenhuma rotina de negócio existe.

---
---

# FASE 1 — Site inicial, login e primeiros menus

**O que é.** O sistema deixa de ser página de prova e passa a ser um app de verdade: uma
pessoa entra com usuário e senha, e vê a casca do FrotaHub com os primeiros menus.

**Escopo fechado pelo usuário — e nada além disso:** implementar o login (tabela de auth,
um login builder genérico, autenticação configurada, **sem** tela de configuração de
logins) e as primeiras funções (front no padrão do FrotaHub atual, com **apenas dois**
menus: Manutenção — com os submenus "Contrato São Luiz" e "Serviços", ambos em breve — e
Configurações, em breve).

**Situação: CONSTRUÍDA E NO AR** em 23/08/2026.

---

## Step 1 — A marca

**O problema.** O logo disponível tinha 200×100 pixels e compressão pesada. Recortar a
figura dele produzia halos brancos e sujeira nas bordas — inaceitável num cabeçalho.

**A primeira tentativa** foi redesenhar a marca em vetor: três barras dobradas com
extrusão 3D. Ficou limpa e escalável, mas era uma reconstrução, não a marca da empresa. E
o `viewBox` apertado demais cortava a figura embaixo, no site.

**A solução.** O usuário mandou o logo em alta resolução — **já com fundo transparente**.
Aí virou recorte puro, sem perda: os degradês, o relevo e as sombras são exatamente os do
logo da empresa. Ficou em 710×640, nítida em tela densa e boa até de favicon.

**O conjunto.** Figura + "FrotaHub" ao lado, repetindo o tratamento do logo original
(primeira palavra pesada, segunda leve, como "FROTA MACEDO"), e "by NorthCore" embaixo,
com o metálico da referência.

Arquivo: `web/public/marca.png`.

---

## Step 2 — Login

### 2.1 Banco

Duas migrations, ambas testadas num PostgreSQL 16 antes de irem para o Supabase.

**`001_tipos.sql`** — cria `schema_migrations` (o registro do que já rodou, **CORE-07**),
o tipo `nivel_acesso` (`builder`, `ceo`, `gerente`, `comum`) e a função de carimbo de
`atualizado_em`.

**`002_perfis.sql`** — cria `perfis`: quem cada login é dentro do FrotaHub.

- **Não guarda senha.** Quem guarda é o Supabase, na `auth.users`, que já vem pronta no
  projeto — não fomos nós que criamos. A `perfis` só aponta para lá, e login apagado leva
  o perfil junto.
- Nasce com **leitura fechada** (**CORE-19**) e ganha **uma única política**: cada pessoa
  enxerga a própria linha. Ninguém lista os outros usuários pelo navegador.
- Escrita não tem política nenhuma — só o servidor grava.

**`seed/001_perfil_builder.sql`** — liga o usuário criado no painel ao perfil. Não cria
login nem guarda senha (**CORE-21** e a regra de que quem cria conta é gente).

**O que deliberadamente NÃO foi criado:** categorias, matriz de permissões, log de
atividades. Entram junto com a tela de configuração de logins, que o usuário pediu para
deixar de fora.

### 2.2 Supabase

- Usuário `builder@frotahub.local` criado **pelo usuário**, no painel, com *Auto Confirm
  User* ligado.
- As três consultas rodadas no SQL Editor, na ordem, conferindo o resultado de cada uma.

### 2.3 Front

- Entrada por **usuário**, não por e-mail: o front acrescenta o domínio sozinho.
- **Sem autocadastro e sem "esqueci minha senha"** — quem cria login é o administrador.
- Erro escrito para ser lido (**CORE-24**): *"Usuário ou senha inválidos."*
- Login que existe no Supabase mas não tem perfil, ou perfil desativado, é recusado com
  explicação — e a sessão é desfeita.

---

## Step 3 — A casca e os menus

**Arquivos criados** (`web/src/`):

| Arquivo | O que faz |
|---|---|
| `main.tsx` | O ponto de partida |
| `App.tsx` | A casca: barra lateral, cabeçalho com o caminho, área de trabalho |
| `estilos/tokens.css` | A identidade num lugar só (**CORE-25**) |
| `estilos/base.css` | O layout: barra lateral fixa, cartões, login, celular |
| `supabase/cliente.ts` | A conexão e a conversão usuário → e-mail |
| `sessao/tipos.ts` · `sessao/useSessao.ts` | Quem está logado (**CORE-17**) |
| `menu/arvore.ts` | A árvore dos menus |
| `componentes/Marca.tsx` | A marca |
| `componentes/Icone.tsx` | Os ícones, desenhados à mão — sem biblioteca |
| `telas/Login.tsx` · `telas/Inicio.tsx` · `telas/EmBreve.tsx` | As três telas |

**Decisões de desenho:**

- **Ícones desenhados à mão, não biblioteca.** São poucos, e assim herdam a cor e a
  espessura do resto da interface em vez de trazerem estilo próprio (**CORE-25**).
- **O que não existe aparece marcado como "breve"**, desabilitado (**CORE-23**) — dá a
  medida do que falta sem prometer o que não funciona.
- **O `App.tsx` não cresce junto com o sistema**: cada rotina futura entra como arquivo
  próprio em `telas/` (**CORE-16**).

**O motor não foi construído nesta fase, de propósito.** O login fala direto com o
Supabase e a segurança de linha protege os dados. O `baleryan` entra quando existir rotina
que precise dele.

---

## Step 4 — Publicação e teste

Publicado por push, pela automação da Fase 0. Resultado conferido em
`https://novo.frotamacedo.com.br`: entrou logado, menu funcionando, nome e nível na barra.

**Tropeço registrado:** depois de publicar, a página continuava mostrando a versão antiga.
Não era a publicação — era **cache do navegador**. Os arquivos de estilo e de programa
levam um código no nome (`index-BYrLgD1G.css`), então mudança neles obriga o navegador a
buscar; o `index.html` tem sempre o mesmo nome e fica guardado. Resolve com Ctrl+Shift+R.
*Correção definitiva pendente: um `.htaccess` dizendo ao servidor para não guardar o
`index.html` em cache.*

---

## Inventário da Fase 1

| Arquivo | Hospedado | Rev |
|---|---|---|
| `web/public/marca.png` | repo · no ar | 1 |
| `web/index.html` | repo · no ar | 3 |
| `web/package.json` · `package-lock.json` · `tsconfig.json` · `vite.config.ts` | repo | 1–2 |
| `web/src/main.tsx` · `App.tsx` | repo · no ar | 1 |
| `web/src/estilos/tokens.css` · `base.css` | repo · no ar | 1 |
| `web/src/supabase/cliente.ts` | repo · no ar | 1 |
| `web/src/sessao/tipos.ts` · `useSessao.ts` | repo · no ar | 1 |
| `web/src/menu/arvore.ts` | repo · no ar | 1 |
| `web/src/componentes/Marca.tsx` | repo · no ar | 2 |
| `web/src/componentes/Icone.tsx` | repo · no ar | 1 |
| `web/src/telas/Login.tsx` · `Inicio.tsx` · `EmBreve.tsx` | repo · no ar | 1 |
| `db/migrations/001_tipos.sql` | repo · **Supabase** | 1 |
| `db/migrations/002_perfis.sql` | repo · **Supabase** | 1 |
| `db/seed/001_perfil_builder.sql` | repo · **Supabase** | 1 |

**Removidos:** `web/vite.config.js`, `web/public/logo.jpg`, `web/public/marca.svg`.

---

## Pendências da Fase 1

| O quê | Situação |
|---|---|
| `.htaccess` para o `index.html` não ficar em cache | proposto, aguardando |
| A sessão fica guardada e reabre sem pedir senha — ruim em computador compartilhado da obra | decidir junto com a tela de logins |
| Senha do builder | o usuário escolheu; recomendado não usar `123456` |

---

---
---

# ANEXO A — Tenets

> **O que são.** As regras de ouro do FrotaHub. Cada uma tem um código permanente, e sempre
> que uma decisão se apoiar numa delas, o texto cita o código em vez de repetir o argumento
> — por exemplo: *"a nota é lida pelo parser antes de qualquer modelo (**CORE-01**)"*.
>
> **Quem declara.** Os **Core** são declarados pelo dono do sistema. Não são extraídos de
> conversa nem deduzidos de código: são escolhas conscientes de quem responde pelo sistema.
> Os demais níveis nascem com o bloco, o módulo ou a rotina que eles governam.
>
> **A lista tem que caber na cabeça.** Uma lista de quarenta princípios fundamentais não é
> um Core — ninguém decora, e na hora da pressão ninguém lembra. Se um candidato a Core não
> for universal, imutável e caro de violar, ele desce para o Anexo B.

## Os cinco níveis

| Nível | Abrangência | Sentido | Código |
|---|---|---|---|
| **Core** | O sistema como um todo | Princípios fundamentais e **imutáveis** | `CORE-01` |
| **Block** | Um grupo de módulos | Regras que valem para aquele bloco | `BLOCK-XXX-01` |
| **Module** | Um módulo específico | Regras internas daquele módulo | `MOD-XXX-01` |
| **Routine** | Uma função ou rota específica | Regras daquela ação | `ROT-XXX-01` |
| **Skill** | Um robô ou automação | Regras daquela automação | `SKILL-XXX-01` |

## As três leis dos tenets

**Herança.** Toda rotina obedece aos tenets do seu módulo, do seu bloco e do Core. Escrever
uma rotina é escrever dentro dessa pilha — o que já está garantido acima não se reescreve.

**Especificidade.** Quando dois tenets se cruzam, vale o mais específico — **exceto contra
o Core**, que nunca é sobreposto. Se um módulo precisa violar um Core, ou o desenho do
módulo está errado, ou o tenet Core estava errado. Nos dois casos, para-se e discute-se;
não se contorna.

**Permanência.** Código de tenet nunca é reaproveitado nem renumerado. Tenet revogado fica
na lista marcado como revogado, com a data e o motivo. *Motivo: referência antiga precisa
continuar significando a mesma coisa.*

---

# CORE

*Nove. Declarados pelo dono do sistema. Valem para toda linha de código do FrotaHub.*

## Recurso

> No FrotaHub, o teto do plano gratuito **é o orçamento do projeto**. Estourar não deixa
> caro: para o sistema.

**CORE-01 — IA é o último recurso, nunca o primeiro.**
Leitura, extração e classificação tentam primeiro o caminho determinístico — regra, parser,
consulta. Só o que sobrar vai para um modelo, e no menor modelo **que resolva**. Toda
chamada é contada. *A economia para onde começa o retrabalho: nota lida errado custa mais
que token.*

**CORE-02 — Guardar é decisão; nada é guardado duas vezes.**
Arquivo tem uma cópia viva e uma de segurança, e só. Coluna que dá para calcular não é
guardada. Histórico que ninguém consulta não é gravado.

**CORE-03 — Automação só acorda quando tem trabalho.**
Rotina automática dispara pelo que mudou, não por varredura de tempo em tempo. Toda
automação declara quanto consome. *Origem: os robôs do Trílogo consumiram quase toda a cota
mensal sem ninguém olhar, e derrubaram a primeira publicação do FrotaHub.*

## Verdade

**CORE-04 — O banco é a única fonte da verdade.**
Fila de trabalho, estado e localização de arquivo saem do banco. Nenhuma rotina descobre o
que fazer varrendo pasta. *Tudo o mais se apoia nisto: se cair, cai o modelo inteiro.*

**CORE-05 — Nada é apagado de vez.**
Remover é marcar. E a garantia é do **banco**, não da disciplina de quem escreve o código.
*Regra da casa desde antes deste projeto.*

**CORE-06 — Cada regra existe uma vez só.**
Preço, teto, duplicidade, roteio: uma implementação, num lugar só, testável. *Origem: no
sistema antigo o mesmo cálculo estava escrito onze vezes, de duas formas que divergiam em
centavos — e a divergência foi para o cliente.*

## Segurança

**CORE-07 — Nada é público por padrão.**
Tabela nasce com leitura fechada; balde nasce sem endereço público. Abrir é decisão
explícita, uma de cada vez.

**CORE-08 — Na dúvida, nega.**
Falha ao verificar permissão resulta em acesso negado, nunca em acesso concedido.

**CORE-09 — Segredo nunca chega ao navegador.**
Chave de serviço, senha de banco e chave secreta ficam no servidor. Sem exceção, sem
"só neste caso".

---

# BLOCK — por bloco de módulos

*Nenhum ainda. O primeiro entra quando o primeiro grupo de módulos for construído.*

---

# MODULE — por módulo

*Nenhum ainda.*

---

# ROUTINE — por rotina

*Nenhuma ainda.*

---

# SKILL — por robô ou automação

## SKILL-PUBLICAR · Publicação do front
**SKILL-PUBLICAR-01 — Confere antes de enviar.**
Se a compilação não gerou o arquivo, aborta sem publicar. *Motivo: envio "bem-sucedido" de
pasta vazia apaga o que estava no ar.*

**SKILL-PUBLICAR-02 — Envio nunca concorre com envio.**
Dois disparos ao mesmo tempo viram fila, não corrida.

---

## Como referenciar

No diário, no código e nas conversas, cite o código entre parênteses:

> A fila vem do banco, não da pasta (**CORE-04**).
> A conta de FTP está trancada no subdomínio (**P-15**).
> Antes de enviar, confere-se que a compilação gerou algo (**SKILL-PUBLICAR-01**).

---
---

# ANEXO B — Práticas de construção

> **O que são.** O padrão de trabalho de quem escreve o código. Não são tenets: são
> subordinados ao Anexo A e podem ser revistos sem cerimônia.
>
> **A diferença que importa.** Tenet é do dono do sistema; prática é de quem constrói. Se
> uma prática um dia se mostrar universal, imutável e cara de violar, ela é candidata a
> subir para o Core — e a promoção é decisão do dono, não de quem escreve.
>
> Código `P-01`, `P-02`… numeração própria e permanente.

## Estado e arquivos

**P-01 — Arquivo é registro.**
Toda escrita em nuvem gera linha na tabela de arquivos. Achar um documento é consulta, não
busca por nome.

**P-02 — O estado não mora na pasta; o arquivo não se move.**
Estado é coluna no banco, não pasta onde o arquivo está. *Mover arquivo para representar
estado gera arquivo perdido, movimentação pela metade e reconciliação — uma família inteira
de defeitos que deixa de existir.*

**P-03 — Nada de estado na memória do servidor.**
Tarefa demorada é linha no banco, com progresso. *Servidor que hiberna, reinicia ou roda em
duas cópias perde tudo que estiver só na memória.*

## Dados

**P-04 — O que é regra vira restrição.**
Se a regra pode ser expressa como restrição do banco, ela é uma restrição — não um
comentário nem uma condição no código. *Comentário não impede nada.*

**P-05 — Mudança de estrutura é migração numerada.**
Nunca comando solto. O banco guarda o que já rodou.

**P-06 — Item é linha, não bloco de texto.**
O que precisa ser consultado, somado ou auditado é tabela. *Dado enterrado em texto não
responde pergunta.*

**P-07 — Sequência vem do banco.**
*Contar o maior e somar um colide quando duas pessoas clicam junto.*

**P-08 — Parâmetro de negócio mora no banco, não no ambiente.**
Com histórico de quem mudou e quando. *Variável de ambiente não deixa rastro.*

## Código

**P-09 — Falhe na largada, não no meio do expediente.**
Configuração incompleta impede o programa de subir, e a reclamação lista **tudo** que falta
de uma vez. *Programa que sobe quebrado transforma erro de configuração em problema do
usuário, horas depois.*

**P-10 — Erro estoura; não vira aviso no console.**
Gravação que falhou não pode seguir como se tivesse gravado.

**P-11 — Dinheiro é decimal, e meio centavo sobe.**
Nunca ponto flutuante. *O arredondamento padrão de várias linguagens manda 2,675 para 2,67.*

**P-12 — Comentário explica o porquê, não o quê.**
O código já diz o que faz. O comentário guarda a decisão, a alternativa descartada e a
armadilha.

**P-13 — Um assunto, um lugar.**
Mexer no cálculo é abrir um arquivo; mexer numa tela é abrir outro.

## Segurança

**P-14 — Uma porta de entrada só.**
Um lugar diz **quem é**; outro diz **o que pode**. Nada além disso decide acesso.

**P-15 — Credencial só enxerga o que precisa.**
*A pergunta certa não é "vai vazar?", é "se vazar, o estrago é onde?"*

**P-16 — Registro não se reescreve.**
Onde houver registro de atividade, ele é acrescentado e nunca editado. E falha ao registrar
nunca desfaz a operação já feita.

## Interface

**P-17 — O menu se ajusta ao login.**
Quem não tem a permissão não vê o item. O que ainda não existe aparece desabilitado, para
dar a medida do que falta.

**P-18 — Mensagem de erro é escrita para ser lida.**
"Sessão expirada", não "401 unauthorized".

**P-19 — A identidade da casa é a mesma em todo lugar.**
Sem biblioteca de componentes com visual próprio.

## Operação

**P-20 — O motor não serve arquivo.**
Ele entrega endereço temporário e o arquivo vai direto da nuvem ao usuário.

**P-21 — Publicação vazia é pior que publicação falha.**
Antes de enviar, confere-se que a compilação gerou algo.

**P-22 — Quem executa é gente.**
Arquivo que roda código no repositório é criado por uma pessoa. *Origem: a escrita remota
em `.github/workflows/` é bloqueada de propósito — e a regra faz sentido.*

---

---

## Endereços e contas

| O quê | Onde | Existe? |
|---|---|---|
| Repositório | `github.com/iafrotamacedo-cloud/frotahub-v2` | sim, público |
| Banco | `https://hltcngamdqabqlocufrv.supabase.co` (us-east-1) | sim, vazio |
| Chave pública do banco | `sb_publishable_IyCf5yioEo-Ry8q5mB2boA__ekxvPIu` | é chave de navegador, não é segredo |
| Arquivos | Cloudflare R2, balde `frotahub`, conta `99f23938821d056868ecef92c08eed7f` | sim, vazio |
| Front | `https://novo.frotamacedo.com.br` | **no ar** |
| Hospedagem | HostGator Plano P, servidor `br1000`, IP `162.241.203.77` | sim |
| Motor | Render, workspace "IA's workspace", região Ohio | a criar |
| Arquivo-mestre | Dropbox | conta existente |

A `service_role` do Supabase, a senha do banco e a chave secreta do R2 **não** ficam neste
arquivo, por escolha.

---

*As fases seguintes entram abaixo, uma seção cada, no mesmo formato de Steps.*
