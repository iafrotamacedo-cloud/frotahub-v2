# FrotaHub — Diário de Bordo

> Registro do projeto, organizado em **Fases** e, dentro delas, em **Steps**.
> Cada step lista o que foi definido, feito e criado — plataforma por plataforma.
> Quem ler só este arquivo sabe onde estamos, o que existe e qual é o próximo passo.
>
> Última atualização: **23/08/2026** · Rodada 25 · **Fases 0, 1 e 2 concluídas · Fase 3 em andamento**

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

# FASE 2 — O motor, o modelo de acesso e os logins

**O que é.** O FrotaHub ganha servidor próprio e o modelo que vai reger o acesso de todas
as rotinas daqui para a frente: quem é cada login, a que grupo pertence, e o que esse grupo
alcança.

**Escopo fechado pelo usuário:** publicar o motor `baleryan`; construir o modelo de acesso
(categorias e matriz de permissões); e as telas de usuários e logins. A matriz **nasce
fechada** — por enquanto só o builder alcança alguma coisa, e o resto o usuário define pelo
próprio sistema, sem passar por código.

**Como foi partida.** Em duas metades, para nada subir de uma vez:
**2a** — banco, motor e as rotinas de servidor. **2b** — as telas.

**Situação: CONSTRUÍDA** em 23/08/2026. Falta só a configuração de sessão no painel do Supabase, que é do dono.

---

## Step 1 — O modelo de acesso no banco

**A pergunta que este step responde:** *quem pode o quê?*

A resposta tem uma regra e duas exceções, e só isso decide acesso no FrotaHub inteiro:

> **A regra.** Cada login pertence a uma **categoria**. Cada categoria tem uma lista de
> **rotinas** liberadas. Abrir uma rotina significa que a sua categoria a tem marcada.
>
> **Exceção 1.** O builder passa sempre — é a trava contra o dono se trancar para fora.
> **Exceção 2.** Só o builder cria ou edita login — para ninguém se promover.

### 1.1 O nível é da categoria, não da pessoa

Esta é a mudança estrutural da fase, e vale explicar porque ela desfaz um problema do
sistema antigo. Antes, o nível ficava gravado na pessoa **e** no grupo dela. Dois lugares
guardando a mesma verdade é uma divergência esperando acontecer: basta alguém editar um e
esquecer o outro, e o sistema passa a ter duas respostas para a mesma pergunta.

Agora o nível mora só na categoria. Quem entra na categoria herda. Discordar virou
impossível por construção, não por cuidado (**CORE-06**).

Consequência prática: a coluna `nivel` **saiu** da tabela de perfis.

### 1.2 Supabase — o que foi criado

| Tabela | Para quê | Como nasceu |
|---|---|---|
| `clientes` | o titular de cada dado | 1 linha: Frota Macedo Engenharia |
| `categorias` | o grupo de um login; **é aqui que mora o nível** | 1 linha: `builder`, protegida |
| `rotinas` | o catálogo do que existe no sistema | **vazio** |
| `categoria_permissoes` | a matriz categoria × rotina | **vazia** |
| `historico` | o rastro de tudo que os módulos declaram registrar | vazio |

**O catálogo nasce vazio, e isso é decisão, não pendência.** Nenhuma rotina do sistema
antigo foi copiada. Cada rotina se cadastra na migração do módulo que a constrói — assim
catálogo e realidade nunca divergem, e não existe permissão para algo que não existe.

**As fechaduras.** Todas as tabelas nascem com leitura fechada (**CORE-07**) e são abertas
uma de cada vez. Hoje um login enxerga: o próprio perfil, o próprio cliente, as categorias
do seu cliente, as permissões da **própria** categoria, e o catálogo de rotinas. Não
enxerga o que as outras categorias alcançam, e não enxerga histórico nenhum.

**Escrever, ninguém escreve pelo navegador.** Toda gravação nessas tabelas passa pelo
motor. As políticas do banco só abrem leitura.

### 1.3 Um defeito encontrado antes de chegar ao usuário

A primeira versão criava, na migração 003, uma função que lia uma coluna que só nasceria na
004. Aplicada em ordem, quebraria. Foi encontrada rodando as migrações num Postgres de
verdade, do zero, antes de qualquer entrega — e corrigida separando: **003 cria tabelas,
004 acrescenta colunas, depois funções, depois políticas.**

---

## Step 2 — O motor no ar

### 2.1 O que é o `baleryan`

O servidor do FrotaHub, escrito inteiramente em Go, **sem uma única dependência externa**.
Ele existe por uma razão que não tem volta: criar login exige a chave de serviço do
Supabase, e essa chave não pode existir no navegador (**CORE-09**). Tudo que precisa dela
é, por definição, do servidor.

Por dentro, cada peça tem um assunto só:

| Peça | Assunto |
|---|---|
| `cmd/baleryan` | liga o motor, monta os módulos, desliga com jeito |
| `interno/config` | **toda** variável de ambiente do sistema, declarada num lugar só |
| `interno/banco` | o único cliente de banco; ninguém mais fala com o Postgres |
| `interno/seguranca` | quem está chamando |
| `interno/permissao` | o que essa pessoa pode |
| `interno/web` | resposta, erro, CORS, registro de chamada |
| `interno/modulos/…` | um módulo por assunto de negócio |

### 2.2 Duas escolhas que valem registro

**O motor se recusa a ligar pela metade.** Ele lê toda a configuração antes de abrir a
porta e, se faltar alguma coisa, imprime a lista **completa** do que falta e desliga.
Não é rigidez: é a diferença entre descobrir o problema no arranque, com o nome dele
escrito, e descobri-lo três dias depois numa tela que quebrou sem explicar por quê.

**O CORS é a camada mais externa.** Quando um erro escapa sem esses cabeçalhos, o navegador
não mostra o erro de verdade — mostra "Failed to fetch", que não diz nada. Envolvendo tudo,
até a falha chega ao front como falha legível.

### 2.3 Render

- Serviço `baleryan`, Docker, região **Ohio**, plano gratuito, publicação automática a cada
  push na `main`.
- **Ohio e não São Paulo** de propósito: o Supabase deste projeto está na Virgínia. Ohio ↔
  Virgínia é ida-e-volta de ~12 ms; São Paulo ↔ Virgínia, ~120 ms. Uma tela que faz cinco
  consultas sente meio segundo de diferença.
- Confirmado no ar em 23/08/2026 respondendo em `https://baleryan.onrender.com/saude`.

### 2.4 Um tropeço que custou um deploy

O Render falhou com *"Root directory 'baleryan' does not exist"*. A causa não estava no
Render: estava numa linha do `.gitignore` escrita por mim, na Fase 0, com o nome do binário
solto — `baleryan`. Um nome solto no `.gitignore` casa também com **pasta**. O Git excluiu
a pasta inteira do motor do envio, **em silêncio**, sem aviso e sem erro.

Corrigido para o caminho completo (`/baleryan/baleryan`), com o motivo escrito no próprio
arquivo para não se repetir. *Lição registrada em P-23.*

---

## Step 3 — Usuários e logins pelo motor

### 3.1 As rotas

| Rota | O que faz |
|---|---|
| `GET /usuarios` | lista, paginada e filtrada **no banco** (**CORE-10**) |
| `POST /usuarios` | cria login |
| `PATCH /usuarios/{id}` | altera nome, categoria ou situação |
| `POST /usuarios/{id}/senha` | troca a senha |
| `GET /usuarios/{id}/historico` | o rastro daquele login |

Todas exigem builder. Nenhuma existe no catálogo de rotinas, porque builder não passa pela
matriz.

### 3.2 Criar login acontece em dois lugares — e sabe desfazer

Um login vive em dois sítios: a `auth.users` do Supabase, que guarda a senha, e a `perfis`,
que diz quem a pessoa é aqui dentro. Se o segundo passo falhar, **o primeiro é desfeito**.
Meio login é pior que nenhum: o nome fica ocupado, ninguém entende por quê, e a pessoa não
entra.

### 3.3 Histórico — a tenet MOD-USUARIOS-01

Declarada pelo usuário nesta rodada: mexer em login deixa rastro. O texto completo está no
Anexo A; o que importa aqui é como foi construído.

**Uma tabela para o sistema inteiro, não uma por módulo.** Histórico é sempre a mesma
pergunta — quem fez, quando, no quê, e o que mudou. Uma tabela por módulo repetiria a
estrutura, o código de gravação e a tela de consulta a cada módulo novo. Com uma só, o
próximo módulo declara a tenet dele e já grava.

*Cuidado para não confundir:* a tabela existir **não** obriga ninguém a gravar. Quem obriga
é a tenet do módulo.

*E a tabela ser compartilhada não quer dizer que o código já seja.* A função que grava mora
hoje **dentro** do módulo de usuários, porque é o único que grava. Ela sobe para uma peça
própria (`interno/historico/`) quando existir o segundo — desenhar a peça compartilhada
antes de ter dois usuários é adivinhar como o segundo vai ser.

**Só o que mudou, no formato `{"campo": {"de": …, "para": …}}`.** Copiar a linha inteira a
cada edição incharia a tabela e esconderia a informação útil no meio do resto (**CORE-02**).

**O nome de quem fez fica congelado.** Se a pessoa for renomeada em 2028, o evento de 2026
continua dizendo o nome de 2026. Histórico que se atualiza sozinho mente sem avisar.

**Duas linhas quando duas coisas mudam.** Renomear e desativar na mesma tela geram
`alterou` e `desativou` separados, para cada linha ter um significado só.

**Campo reenviado igual não é alteração.** Não gera gravação nem linha. Rastro cheio de
"mudou de X para X" é rastro que ninguém lê.

**Senha nunca entra.** Nem cifrada, nem em pedaço, nem o tamanho. `trocou_senha` registra o
fato e nada mais.

**A tranca é do banco.** Alterar, apagar e esvaziar a tabela de histórico são recusados por
gatilho — nem o motor, que usa a chave de serviço e passa por cima das políticas, consegue
furar. Regra que depende de lembrança um dia é esquecida.

### 3.4 Um segundo defeito, encontrado no teste da própria tranca

A tabela nasceu com o autor apontando para a tabela de perfis com "apagar → põe nulo".
Parecia certo. No teste, apagar um login fez o banco tentar **alterar** as linhas de
histórico dele — e a tranca de imutabilidade recusou. Resultado: quem já tinha feito
alguma coisa no sistema não podia mais ser removido, e a mensagem de erro não explicava
por quê.

A correção foi menos um remendo e mais uma conclusão: **histórico é retrato do passado, não
relação viva.** Retrato não se atualiza quando o retratado muda. A tabela ficou sem chaves
estrangeiras para autor e para registro; guarda o número e o rótulo da época.

### 3.5 O que o motor faz quando a alteração dá certo e o histórico não

São duas gravações separadas, e existe uma janela mínima em que a primeira funciona e a
segunda falha. Nesse caso o motor **não** responde erro — a operação aconteceu, e dizer
"deu errado" faria o usuário tentar de novo, criando um segundo login ou desfazendo o que
acabou de fazer. Ele responde sucesso **com um aviso explícito na tela**, e escreve a falha
no console do servidor como segunda trilha.

*A alternativa — as duas gravações dentro de uma função única do banco, tudo ou nada — fica
para o primeiro módulo de volume. Aqui, mexer em login é coisa de builder algumas vezes por
ano, e a falha é barulhenta, não silenciosa.*

---

## Step 4 — Categorias, matriz e Minha conta (2b, entrega 1)

**O que é.** As peças de servidor que faltavam para os logins fecharem. Nada aparece
na tela ainda: esta entrega é só o motor e o banco.

**Decisões do usuário que moldaram este step:**

| Pergunta | Resposta |
|---|---|
| Como as categorias nascem? | Por tela, desde o começo — nenhuma presa em migração |
| Usuário comum troca a própria senha? | Sim, pela tela Minha conta, exigindo a senha atual |
| Sessão expira? | Sim: **3 horas parado**, **24 horas no total** |
| Mexer na matriz deixa rastro? | Sim — nasce a `MOD-ACESSO-01` |

### 4.1 Banco — migração `006`

Uma coluna só: `ativo` nas categorias. Entrou agora porque a tela de categorias ia
nascer nesta fase e alguém ia querer tirar uma de circulação — e, pela **CORE-05**,
tirar de circulação é **marcar**, não apagar. Categoria apagada deixaria login
antigo e histórico apontando para o nada.

Acrescentar a coluna hoje, com uma categoria no banco, é trivial. Depois, com
quinze categorias em uso e telas escritas em cima delas, não é.

### 4.2 O rastro virou peça própria

Na revisão anterior, gravar histórico era uma função privada dentro do módulo de
usuários — e o diário registrava por quê: *desenhar a peça compartilhada com um
usuário só é adivinhar como o segundo vai ser.*

O segundo chegou. Com dois módulos gravando, o formato deixou de ser suposição, e
a função subiu para `interno/historico/` (**CORE-06**). Os dois módulos usam a
mesma, e o próximo usa também.

*Isso não muda quem grava.* A peça existir continua não obrigando ninguém: quem
obriga é a tenet do módulo.

### 4.3 Motor — o módulo `acesso`

| Rota | O que faz |
|---|---|
| `GET /categorias` | lista; só as em circulação, salvo pedido explícito |
| `POST /categorias` | cria |
| `PATCH /categorias/{id}` | renomeia, muda o nível, tira ou devolve à circulação |
| `GET /categorias/{id}/permissoes` | o catálogo inteiro **mais** o que esta categoria alcança |
| `PUT /categorias/{id}/permissoes` | grava a matriz |
| `GET /categorias/{id}/historico` | o rastro daquela categoria |

**Quatro travas, e o motivo de cada uma:**

**`builder` não é opção de formulário.** Nem como código, nem como nível. A
categoria builder é única, nasce protegida pela migração e é a trava anti-tranca do
sistema inteiro. Se virasse item de lista suspensa, um clique distraído criaria um
segundo dono — e o segundo dono pode desativar o primeiro.

**A categoria protegida não se edita nem entra na matriz.** Marcar rotina para ela
não teria efeito algum, já que o builder passa por construção. A tela recebe um
aviso dizendo isso, para o quadro desabilitado não parecer defeito.

**Categoria com gente dentro não sai de circulação.** A resposta diz **quantos**
logins ativos ainda estão nela. Sem isso, essas pessoas ficariam num grupo que a
tela não mostra mais — um problema invisível.

**Rotina inventada é recusada com o nome dela.** A chave estrangeira do banco também
barraria, mas o erro dela não explica nada para quem está na tela.

### 4.4 A matriz grava o estado inteiro, não o clique

A tela manda a lista **completa** do que deve ficar marcado, e o motor calcula a
diferença. É mais simples de acertar do que mandar "marque isto, desmarque aquilo":
se dois builders salvarem quase junto, o último grava um estado coerente inteiro, em
vez de metade de dois estados.

Retirar uma rotina grava `pode = false` — **não apaga a linha** (**CORE-05**). A
linha que fica é o registro de que aquela rotina já esteve liberada.

E o histórico é **uma linha por salvamento**, listando só o que mudou. Marcar trinta
rotinas de uma vez gera uma linha com trinta campos, não trinta linhas.

### 4.5 Trocar a própria senha

`POST /minha-conta/senha` é a única rota de login que **não** é exclusiva do builder.

Ela exige a senha atual, e o motivo é concreto: o computador da obra deixado aberto.
Sem essa confirmação, quem passar pela cadeira troca a senha de quem esqueceu de
sair, e o dono do login perde a própria conta.

**Quem confere a senha atual é o Supabase, não o motor.** O motor não guarda senha
nenhuma (**CORE-09**): ele pergunta ao Supabase se aquele par usuário+senha entra, e
descarta a sessão que a pergunta cria. Só o sim ou o não interessa.

E grava histórico igual. A **MOD-USUARIOS-01** não abre exceção para "foi a própria
pessoa" — o valor do rastro está justamente em ele ser completo.

---

## Step 5 — Usuários e Logins (2b, entrega 2)

**A primeira rotina de verdade do FrotaHub novo.** Configurações deixou de ser "em
breve" e passou a ter conteúdo.

### 5.1 O que a tela faz

| Ação | Onde |
|---|---|
| Listar, com busca e paginação | a própria tela |
| Criar login | janela |
| Editar nome e categoria | janela |
| Trocar senha | janela separada, com confirmação |
| Ativar / desativar | direto na linha |
| Ver o histórico | janela |

**Tudo passa pelo motor, nada vai direto ao banco.** Criar login e trocar senha
exigem a chave de serviço, que não pode existir no navegador (**CORE-09**). O
único atalho direto ao banco continua sendo a leitura do próprio perfil, que é o
que faz a tela abrir sem esperar o motor acordar.

### 5.2 Cinco decisões de tela, e o que cada uma evita

**O usuário não se edita.** Ele é a identidade da pessoa no login e no histórico.
Trocá-lo faria o rastro antigo apontar para um nome que não existe mais. Quem
precisar mudar cria outro login e desativa o antigo: dois registros honestos em
vez de um registro reescrito.

**A senha não está no formulário de edição.** Quem abre aquela janela quase sempre
quer corrigir um nome. Com a senha no meio, um salvamento distraído tranca outra
pessoa para fora. É ação separada, com a senha digitada duas vezes e um aviso de
que a antiga para de funcionar na hora.

**A categoria do dono do sistema não aparece na lista de escolha.** Colocar alguém
nela dá acesso total — e permite desativar quem está criando. Um segundo dono deve
ser decisão deliberada, não um item de lista suspensa.

**O próprio login não se desativa**, e o botão fica desabilitado explicando por quê.
A mesma trava existe no motor; a da tela é só para a pessoa não descobrir pelo erro.

**A busca espera a pessoa parar de digitar** (um terço de segundo). Cada tecla
viraria uma ida ao banco, e banco também é recurso (**CORE-01**).

### 5.3 A espera do motor é explicada, não escondida

O Render gratuito **adormece** o motor depois de um tempo sem uso, e a primeira
chamada do dia pode levar quase um minuto só para acordar o serviço.

Uma tela em branco nesse intervalo parece defeito — e quem acha que é defeito
recarrega a página, o que reinicia a espera. Por isso a mensagem muda depois de
quatro segundos: sai o "carregando" genérico e entra *"acordando o servidor — a
primeira vez do dia demora um pouco"* (**P-25**).

### 5.4 O menu se ajusta ao login

A árvore de menus deixou de ser fixa. Cada item pode ser marcado como exclusivo do
dono do sistema, e um bloco cujos filhos todos sumiram some junto — menu com pasta
vazia é pior que menu sem a pasta, porque parece defeito.

Hoje isso não muda nada na prática, já que só existe o builder. Mas é a peça que a
matriz de permissões vai usar quando existirem outras categorias, e ela precisa
existir antes.

### 5.5 O endereço do motor não está escrito no código

Ele entra na compilação, vindo de uma variável do repositório (`VITE_MOTOR_URL`).
Assim o mesmo código serve o endereço de teste e o definitivo sem ninguém editar
arquivo.

Se a variável faltar, a tela **diz exatamente isso** em vez de mostrar "falha de
rede" — que mandaria qualquer um procurar o problema no lugar errado.

### 5.6 Como foi conferido

O front foi compilado e aberto num navegador de verdade, sem ninguém olhando, contra
um Supabase e um motor de mentira montados só para o teste. Foram percorridos: entrar,
abrir a rotina, buscar, abrir as três janelas, e a mesma tela em tamanho de celular.

Dois defeitos apareceram aí e foram corrigidos antes da entrega:

1. **Bolinhas onde não devia.** A linha do tempo marcava cada evento com um ponto
   vermelho — e o ponto estava vazando para a lista de campos alterados dentro de
   cada evento, porque a regra de estilo alcançava os itens aninhados.
2. **Cabeçalho de tabela órfão.** Quando a lista falhava, a mensagem de erro aparecia
   e, logo abaixo, uma tabela vazia com o cabeçalho. Agora, se falhou, não há tabela.

---

## Step 6 — Categorias, matriz, Minha conta e sessão (2b, entrega 3)

**O que fecha aqui.** Com esta entrega o ciclo de logins fica completo: dá para criar
grupos, dizer o que cada grupo alcança, criar pessoas dentro deles, e cada um cuida da
própria senha.

### 6.1 O login do dono mudou de nome

A pedido do dono, o login `builder` passou a ser **`igor`**. A troca foi feita direto no
banco, em três lugares que precisam concordar: `perfis.usuario`, o e-mail interno em
`auth.users`, e a identidade do provedor de e-mail. Se um dos três ficasse para trás, o
login pararia de funcionar sem explicação.

**A categoria continua se chamando `builder`** — ela é o cargo, não a pessoa, e é ela que
carrega a trava anti-tranca.

*Nota de coerência:* a tela de usuários não permite editar o nome curto de um login, e
isso continua valendo. Esta troca foi feita **fora do sistema**, no banco, por quem
responde por ele — que é exatamente a forma como uma exceção deve acontecer: deliberada,
e não a um clique de distância. O histórico não perdeu nada, porque ainda não havia
nenhuma linha gravada.

### 6.2 Tela de Categorias

Lista, cria, edita, arquiva e devolve à circulação. Mostra também as arquivadas — quem
administra precisa enxergá-las para poder trazê-las de volta; quem só vai escolher uma
categoria ao criar um login recebe apenas as ativas, e esse filtro é do motor.

**O código não se edita depois de criado**, pela mesma razão que o nome curto de um login
não se edita: ele é a identidade do registro no histórico. O nome, esse sim, muda à
vontade — é ele que aparece nas telas.

**A categoria protegida tem Editar e Arquivar desabilitados**, com a explicação no próprio
botão. As mesmas travas existem no motor; as da tela são só para a pessoa não descobrir
pelo erro.

### 6.3 A matriz

Quadro de rotinas por módulo, com caixas de marcar. Salva o **estado inteiro**, não o
clique — se dois builders salvarem quase junto, o último grava um estado coerente em vez
de metade de dois.

Três estados, e cada um diz o que está acontecendo:

| Situação | O que a tela mostra |
|---|---|
| Categoria comum, catálogo com rotinas | o quadro, marcável |
| Categoria comum, **catálogo vazio** | explica que cada rotina se cadastra quando o módulo dela é construído |
| Categoria protegida | explica que ela alcança tudo por construção, e desabilita o quadro |

Hoje o segundo caso é o normal: **o catálogo está vazio**, e vai ficar até o primeiro
módulo de negócio existir. Isso é o desenho funcionando, não tela quebrada — e a mensagem
diz isso com todas as letras para ninguém procurar defeito onde não tem.

No histórico, mexer em permissão não aparece como "de inativo para ativo", que não é como
se fala de uma rotina. Aparece como **"Liberou: A, B"** e **"Retirou: C"**.

### 6.4 Minha conta

Abre clicando no próprio nome, na barra lateral — é onde a pessoa procura a própria conta,
e evita mais um item de menu para uma tela que se usa duas vezes por ano.

Mostra usuário, categoria e nível, e troca a senha exigindo a atual. Grava histórico
igual: a **MOD-USUARIOS-01** não abre exceção para "foi a própria pessoa".

### 6.5 A sessão acaba por tempo

**3 horas parado, 24 horas no total** — números escolhidos pelo dono, pensando no
computador compartilhado da obra.

**Leia isto antes de confiar na peça do navegador.** Cronômetro de navegador é
conveniência, não tranca: ele fecha a tela, mas a credencial guardada continuaria valendo
se alguém a pegasse por fora. A trava de verdade é a **configuração de sessão do Supabase**,
no painel, e é o dono quem mexe lá. O navegador entra para levar a pessoa ao login de forma
limpa, com a frase *"a sua sessão foi encerrada por tempo"*, em vez de ela descobrir que
expirou por um erro no meio de um clique.

**Os carimbos ficam guardados no navegador**, e não só na memória. Um cronômetro em memória
morre junto com a aba: quem fechasse o navegador às 18h e voltasse às 8h encontraria a
sessão aberta, porque nenhum cronômetro chegou a disparar. Com os carimbos, a conta também
é feita na hora de abrir.

### 6.6 O rastro virou peça compartilhada também na tela

Na entrega 2, a janela de histórico morava dentro da tela de usuários. Com o segundo módulo
gravando, ela subiu para `componentes/` e passou a receber o caminho e o vocabulário
(**CORE-06**) — o mesmo movimento que a função de gravar fez do lado do motor na entrega 1,
e pela mesma razão: com dois usos, o formato deixa de ser suposição.

### 6.7 Três defeitos encontrados na prova em navegador

Nenhum apareceu na compilação nem na verificação de tipos:

1. **A matriz saiu sem espaçamento e em negrito.** A regra de estilo da linha era menos
   específica que a regra geral de rótulo de formulário, que manda `display:block`. A linha
   nunca chegou a ser flex, e a caixinha ficou colada no texto.
2. **Carimbo de sessão renascendo depois da saída.** Ao expirar, o próprio rearme do
   cronômetro gravava um "usei agora" logo após a limpeza, e a marca ficava no navegador de
   quem já tinha sido deslogado. Corrigido com uma trava: encerrou, ninguém mais escreve.
3. *(da entrega 2, registrado aqui por completude)* pontos vermelhos vazando na linha do
   tempo e cabeçalho de tabela órfão no erro.

O item 2 só apareceu porque o teste **conferia o conteúdo do armazenamento depois de sair**,
e não apenas se a tela certa tinha aparecido. Olhar só o que se vê teria deixado passar.

---

## Step 7 — Ajustes depois da Fase 2

### 7.1 O botão "voltar" do navegador

**O problema.** A tela aberta vivia só na memória da página, e o endereço nunca mudava.
O navegador não sabia que alguém tinha andado dentro do sistema: apertar "voltar" saía do
FrotaHub e ia para o site anterior. É o tipo de coisa que faz perder o que se estava vendo
por um reflexo — e todo mundo tem esse reflexo.

**A correção.** O caminho passou a morar no endereço:
`novo.frotamacedo.com.br/#/configuracoes/usuarios`. Com isso, "voltar" volta uma tela,
"avançar" avança, recarregar cai no mesmo lugar, e um endereço colado numa conversa abre
onde deve.

**Por que com `#`.** O sistema é um arquivo só, servido pelo HostGator. Com endereço sem
`#`, quem recarregasse numa tela interna receberia um 404 do servidor, porque aquela pasta
não existe lá — só existe dentro do programa. Consertar isso pede uma regra de reescrita no
servidor: mais uma peça para configurar, e mais uma para quebrar em silêncio. O `#` não vai
ao servidor, funciona sem configurar nada, e o dia em que a regra existir ele some sem
mudar mais nada.

**O endereço não é derivado do título.** Cada item de menu carrega um `rota` escrito à mão.
Título é texto de tela e muda quando alguém acha uma palavra melhor; se o endereço
acompanhasse, todo favorito e todo link colado apontariam para o vazio no dia seguinte.

**De brinde, uma proteção.** A árvore de menus já vem filtrada pelo que aquele login
alcança, e é nela que o endereço é resolvido. Um endereço para uma tela que a pessoa não
pode ver simplesmente não resolve, e ela cai no início — sem precisar conferir permissão
outra vez.

**Conferido em navegador**, com dez verificações: ir e voltar entre telas, avançar,
recarregar numa tela interna, abrir por link colado (com o grupo certo já aberto na barra
lateral), endereço inventado caindo no início, e — o ponto de partida de tudo — "voltar" do
início **não** saindo mais para a página visitada antes do FrotaHub.

### 7.2 Nova prática declarada pelo dono: P-29

> **Facilidade de operação é requisito, não acabamento.**
> Quando a escolha for entre o que é simples de programar e o que é simples de operar,
> ganha operar.

Ela entra no Anexo B, na seção **Interface**, e vale daqui para a frente e para trás: o que
já está construído passa a ser lido também por esta régua.

*Observação de escala, para o dono decidir quando quiser:* pelo texto — "tudo deve ser
pensado assim" — ela tem tamanho de **Core**, não de prática. Ficou como P-29 porque foi
assim que foi pedida. Promover depois é trocar o código nas citações; o texto não muda.

### 7.3 Dois arquivos fantasmas quebraram a publicação da entrega 3

Na entrega 3, duas peças mudaram de lugar: `Carregando.tsx` e `Historico.tsx` saíram de
`telas/usuarios/` para `componentes/`. As cópias novas foram entregues; **as antigas
ficaram**, porque a ponte que escreve na máquina do dono não apaga arquivo.

O `git add -A` recolheu os dois fantasmas, e eles subiram junto. O `Historico.tsx` antigo
importava tipos que a mudança tinha removido, então a compilação passou a falhar — e, como
a publicação compila antes de enviar, **ela parou e não enviou nada**. O site continuou
mostrando a entrega 2, sem erro visível para quem estava usando.

*A publicação falhar aqui foi o sistema funcionando* (**P-21**): melhor não publicar do que
publicar pela metade. O defeito foi não perceber que mover arquivo pede uma remoção
explícita do outro lado. Virou **P-28**.

### 7.4 Configurações passou a existir para todo mundo

**O problema.** Tudo o que morava em Configurações era do builder, e a árvore de menus
esconde um bloco cujos filhos todos sumiram. Resultado: quem não é builder não via
Configurações **nenhuma** — e, portanto, não tinha por onde trocar a própria senha. A
tela existia, mas só abria clicando no próprio nome na barra lateral, o que ninguém
adivinha.

**A correção.** *Minha conta* virou um item de Configurações, **sem** marca de builder. É
ele que faz o bloco existir para qualquer login.

**Ela deixou de ser janela e virou tela.** Item de menu que abre janela sobreposta é
inconsistente com o resto do sistema, e não teria endereço próprio — não daria para voltar
nem recarregar nela.

**As duas portas continuam.** O item no menu e o clique no próprio nome levam à mesma tela,
pelo mesmo endereço. Quem pensa "minhas configurações" acha no menu; quem pensa "minha
conta" clica no nome. Nenhuma das duas obriga a lembrar da outra (**P-29**) — e é uma
implementação só (**CORE-06**).

*Detalhe de implementação que evita dor:* o clique no nome **procura** o caminho até a tela
dentro da árvore, em vez de tê-lo escrito à mão. Se o item mudar de lugar amanhã, o atalho
continua certo sozinho.

**O que a P-29 mudou nesta tela, na prática:**

- As conferências aparecem **enquanto se digita** — senha curta, senhas diferentes, senha
  nova igual à atual — em vez de só depois de clicar em salvar e esperar o servidor.
- O botão fica travado enquanto os dados não fecham, então não há clique que só serve para
  descobrir que faltava algo.
- A ficha diz **quem muda cada coisa**: nome, usuário e categoria são de quem administra; a
  senha é sua. Isso poupa a pergunta e a espera por uma resposta que não viria.

**Conferido em navegador, com os dois tipos de login.** Treze verificações: o que cada um
vê no menu, abrir pelo menu e pelo nome, o endereço em cada caso, senha atual errada
recusada com frase clara, senha certa trocada com aviso, os avisos que aparecem antes de
salvar, o botão travado, e o botão voltar continuando a funcionar.

---

## Inventário da Fase 2

| Arquivo | Hospedado | Rev |
|---|---|---|
| `db/migrations/003_acesso.sql` | repo · **Supabase** | 1 |
| `db/migrations/004_perfis_acesso.sql` | repo · **Supabase** | 1 |
| `db/migrations/005_historico.sql` | repo · **Supabase** | 1 |
| `db/migrations/006_categorias_ativo.sql` | repo · **Supabase** | 1 |
| `baleryan/go.mod` · `Dockerfile` | repo · **Render** | 1 |
| `baleryan/cmd/baleryan/baleryan.go` | repo · **Render** | 4 |
| `baleryan/interno/config/config.go` | repo · **Render** | 2 |
| `baleryan/interno/banco/cliente.go` | repo · **Render** | 2 |
| `baleryan/interno/web/web.go` | repo · **Render** | 1 |
| `baleryan/interno/seguranca/seguranca.go` | repo · **Render** | 2 |
| `baleryan/interno/permissao/permissao.go` | repo · **Render** | 2 |
| `baleryan/interno/historico/historico.go` | repo · **Render** | 1 |
| `baleryan/interno/modulos/usuarios/usuarios.go` | repo · **Render** | 3 |
| `baleryan/interno/modulos/acesso/acesso.go` | repo · **Render** | 1 |
| `baleryan/interno/modulos/*/prova_test.go` | repo (não vai para o Render) | 1 |
| `web/src/sessao/tipos.ts` · `useSessao.ts` | repo · no ar | 2 |
| `web/src/motor/cliente.ts` | repo | 1 |
| `web/src/vite-env.d.ts` | repo | 1 |
| `web/src/componentes/Janela.tsx` | repo | 1 |
| `web/src/componentes/Icone.tsx` | repo | 3 |
| `web/src/menu/arvore.ts` | repo | 5 |
| `web/src/menu/navegacao.ts` | repo | 1 |
| `web/src/App.tsx` | repo | 5 |
| `web/src/main.tsx` · `telas/Inicio.tsx` | repo | 2 |
| `web/src/estilos/telas.css` | repo | 3 |
| `web/src/telas/usuarios/` (4 arquivos) | repo | 1–2 |
| `web/src/componentes/Carregando.tsx` | repo | 2 |
| `web/src/componentes/Historico.tsx` | repo | 2 |
| `web/src/telas/categorias/` (4 arquivos) | repo | 1 |
| `web/src/telas/MinhaConta.tsx` | repo | 2 |
| `web/src/sessao/inatividade.ts` | repo | 1 |
| `web/src/sessao/useSessao.ts` | repo | 3 |
| `web/src/telas/Login.tsx` | repo | 2 |
| `.gitignore` | repo | 2 |

**Alterados por causa da fase:** `perfis` perdeu a coluna `nivel`; o front passou a ler o
nível da categoria.

---

## Decisões da Fase 2

| # | Decisão | Por quê |
|---|---|---|
| 1 | Nível é propriedade da categoria | Dois lugares guardando a mesma verdade divergem. |
| 2 | Catálogo de rotinas nasce vazio | Não existe permissão para o que não existe. |
| 3 | Motor em Ohio | ~12 ms até o banco, contra ~120 ms de São Paulo. |
| 4 | Motor não liga pela metade | Falta de configuração vira erro nomeado no arranque. |
| 5 | Uma tabela de histórico para o sistema todo | O próximo módulo declara a tenet e já grava. |
| 6 | Histórico sem chaves estrangeiras | Retrato do passado não se atualiza quando o presente muda. |
| 7 | Histórico trancado pelo banco | Regra que depende de disciplina um dia falha. |
| 8 | Falha de histórico vira aviso, não erro | A operação aconteceu; mentir sobre isso causa dano maior. |
| 9 | Categorias por tela, nenhuma semeada | Mudar um nome vira clique, não envio de código. |
| 10 | `builder` fora dos formulários | Um clique distraído criaria um segundo dono do sistema. |
| 11 | Categoria com gente dentro não sai de circulação | Evita gente presa num grupo que a tela não mostra. |
| 12 | A matriz grava o estado inteiro | Dois builders salvando junto não geram meio estado. |
| 13 | Retirar rotina grava `pode = false` | A linha que fica registra que já esteve liberada (CORE-05). |
| 14 | Senha atual conferida pelo Supabase | O motor não guarda senha nenhuma (CORE-09). |
| 15 | Sessão: 3 h parado, 24 h no total | Escolha do usuário, pensada no computador compartilhado da obra. |
| 16 | O nome curto do login não se edita | Editar faria o histórico antigo apontar para quem não existe mais. |
| 17 | Senha fora do formulário de edição | Salvamento distraído trancaria outra pessoa para fora. |
| 18 | Endereço do motor vem da compilação | O mesmo código serve teste e definitivo sem editar arquivo. |
| 19 | A espera do motor é explicada na tela | Tela em branco parece defeito, e quem acha isso recarrega. |
| 20 | Código da categoria não se edita | É a identidade dela no histórico. |
| 21 | A matriz salva o estado inteiro | Dois builders salvando junto não geram meio estado. |
| 22 | Minha conta abre pelo próprio nome na barra | É onde a pessoa procura, e não vira item de menu. |
| 23 | Carimbos de sessão guardados no navegador | Cronômetro em memória morre com a aba e não conta o tempo fechado. |
| 24 | O login do dono passou a ser `igor` | Pedido do dono; feito no banco, fora do sistema, de propósito. |
| 25 | O caminho mora no endereço, com `#` | "Voltar" volta uma tela; e não exige regra nenhuma no servidor. |
| 26 | O endereço vem de um `rota` escrito à mão | Título muda; endereço não pode mudar junto. |
| 27 | Minha conta é tela, não janela | Item de menu precisa de endereço próprio, para voltar e recarregar. |
| 28 | Duas portas para Minha conta | Menu e clique no nome; ninguém precisa lembrar da outra (P-29). |

---

## Pendências da Fase 2

| O quê | Situação |
|---|---|
| `AMBIENTE=producao` no Render | **só depois do R2**: em produção o motor exige R2 e Dropbox configurados, e hoje não estão |
| `CORS_ORIGENS` no Render | está no padrão `*`; fechar em `https://novo.frotamacedo.com.br` |
| Conferir a variável `VITE_MOTOR_URL` no GitHub | tem que valer `https://baleryan.onrender.com`; sem ela a tela avisa e não carrega |
| **Configurar a sessão no painel do Supabase** | 3 h de inatividade e 24 h no total. É a única trava real; o navegador só leva ao login de forma limpa |
| Apagar `builder_list` | tabela de teste do R2, ainda no banco |

---

## Próximo passo

**Fase 3, Step 3: o robô.** As tabelas estão de pé; falta quem as preencha.

Continua pendente da Fase 2, e é do dono: configurar a sessão no painel do Supabase
(3 h paradas, 24 h no total).

---
---

# FASE 3 — Os dados do Trílogo

**O que é.** O FrotaHub deixa de olhar o Trílogo por cima e passa a ter os dados
dentro de casa: chamado, timeline completa, custos e arquivos, das duas contas.
É a base de tudo que vem depois — métricas, orçamentos, relatórios.

**Escopo fechado pelo dono:**

- **Menu:** Manutenção › Contrato São Luiz › **Dados do Trílogo** (antes se chamava
  "Lista do Trílogo").
- **Uma carga inicial** desde 01/07/2026, feita **uma vez**, pelo Actions, sem
  aparecer no front.
- Depois, um **robô de atualização**, que aparece no front, roda no botão e
  agendado.
- Tudo em **Go**, com o máximo de agilidade. A versão antiga era boa, mas em Python
  e com navegador.
- Dados no **Supabase**, arquivos no **R2**.
- **Cuidado com os orçamentos:** nós mesmos vamos gerar e subir orçamentos no
  Trílogo. Ler de volta não pode duplicar arquivo.
- **Dedup:** na carga vem tudo; na leitura rotineira, só o que mudou.
- **Fora do escopo:** 4 lojas (Crato, Juazeiro, Lagoa Seca, Novo Juazeiro).

**Situação: Step 1 e 2 concluídos.** O robô ainda não foi escrito.

---

## Step 1 — O reconhecimento da API do Trílogo

Antes de desenhar qualquer coisa, li o robô antigo inteiro e depois mapeei a API
pelo navegador do dono, com a sessão dele já aberta. Sem isso, cada endereço
errado seria um ciclo de escrever, publicar, falhar e refazer.

### 1.1 O robô antigo não tinha API — tinha um navegador

Ele abria um Chrome, fazia login como gente e **roubava o token** que o site usa
nas próprias chamadas. Conhecia dois endereços, e só. Nada de timeline, nada de
anexos, nada de ambiente.

### 1.2 O que existe de verdade

| Chamada | O que traz |
|---|---|
| `POST /api/Login/SignIn` | **o token, direto** — e-mail e senha, sem navegador |
| `POST /api/Ticket/ListTicketsByUser` | a lista paginada, 99 campos por chamado |
| `GET /api/Ticket/GetTicketDetail?id=` | o chamado completo, 81 campos |
| `GET /api/Ticket/GetTicketCosts/?ticketId=` | os custos e os arquivos de nota |

**Um chamado inteiro cabe numa chamada só.** `GetTicketDetail` já traz
`attachments` e `activity` (a timeline) dentro.

O achado que mais pesa é o primeiro: **`Login/SignIn` mata o Playwright.** O robô
antigo gastava mais tempo abrindo o Chrome do que lendo os dados.

**Onde cada coisa dos prints está:** *Ambiente* é `department`, e já vem com o
caminho montado. *Tipo predial* é `buildingServiceType`. *Natureza*, *Tipo*,
*Tags*, *Empresa prestadora com CNPJ* e *Criado por* estão todos no detalhe.

### 1.3 Os arquivos são públicos

Fotos, vídeos e PDFs ficam no `s3.amazonaws.com`, **sem autenticação e sem
assinatura na URL**. Para o robô, ótimo: baixar é um GET simples.

Como aviso: **qualquer pessoa com o endereço abre o arquivo**, sem estar logada no
Trílogo. Não é coisa nossa e não dá para consertar de fora — mas reforça que o
nosso R2 nasça fechado (**CORE-07**), para não repetirmos o problema aqui.

### 1.4 Os números, medidos e não estimados

| | Instalações | Civil | Total |
|---|---|---|---|
| Chamados desde 01/07/2026 | 970 | 460 | **1.430** |
| Anexos por chamado | 2,8 | 2,9 | **~4.050 arquivos** |
| Deles, vídeo | 22% | 9% | **~720 vídeos** |
| Eventos de timeline | 9,1 | 9,6 | **~13.200 eventos** |
| Chamados com custo | 13% | 5% | ~150 |

Aparecem também áudios (`.ogg`, `.m4a`), `.zip` e PDFs soltos: o robô precisa
aguentar qualquer tipo, não só foto.

Os metadados cabem folgados no Supabase. **O peso está inteiro nos arquivos**, e a
estimativa passa dos 10 GB do R2 grátis. Por isso a carga foi partida em duas
passadas — ver o Step 2.

### 1.5 As 4 unidades fora do escopo, por id

| id | Loja |
|---|---|
| 94 | LOJA 07 — CRATO |
| 95 | LOJA 12 — JUAZEIRO |
| 96 | LOJA 25 — LAGOA SECA |
| 300 | LOJA 31 — MERCADÃO NOVO JUAZEIRO |

Existem **38 unidades** no histórico; sobram 34 no escopo — e três delas não são
loja: **CD**, **EMPÓRIO** e **ESCRITÓRIO**.

O robô antigo excluía procurando as palavras "juazeiro", "lagoa seca" e "crato" no
nome, o que excluiria por acidente qualquer loja futura com essas palavras. Aqui é
pelo **id**, que é exato.

### 1.6 Três defeitos herdados que não serão repetidos

**A prioridade está trocada no sistema antigo.** Ele grava `1 = Baixa, 2 = Média`.
Cruzando código com tela em seis chamados: é **`1 = Média, 2 = Baixa`**. Qualquer
relatório de prioridade tirado da base antiga está errado. Existe ainda um código
`0`, em 64 chamados, que ninguém rotulou.

**O status está certo, mas os dois arquivos do robô antigo discordam entre si.**
Um diz `5 = Executado`, o outro diz `5 = Vistoriado`. O certo é **5 = Executado,
6 = Vistoriado, 7 = Em execução, 1 = Aberto** (conferido contra a tela).

**A chave "número + conta" cria duplicatas.** A timeline tem o evento *"Nova
empresa prestadora"*: um chamado **troca de conta** ao longo da vida. Com aquela
chave, a troca cria dois registros do mesmo chamado, os dois parecendo válidos.

---

## Step 2 — As tabelas (migração 007)

### 2.1 As três decisões que moldaram o desenho

**A chave do chamado é o NÚMERO, sozinho.** A conta é atributo, não identidade —
pelo motivo do item 1.6.

**Guarda-se o código cru do Trílogo junto com o rótulo.** Depois de encontrar uma
prioridade invertida e dois arquivos discordando sobre status, guardar só o rótulo
seria apostar que acertamos de primeira. Com o número ao lado, um erro de tradução
se corrige com um `UPDATE` — sem reler 1.430 chamados.

**O arquivo e a aparição dele são coisas diferentes.** `arquivos` é o conteúdo,
identificado pelo **sha256**; `chamado_anexos` é cada vez que aquele conteúdo
aparece num chamado.

### 2.2 É isto que impede o orçamento de duplicar

O ciclo temido: o FrotaHub gera o orçamento, guarda no R2, sobe no Trílogo, e o
robô lê de volta.

Com a separação acima, o robô reconhece o conteúdo e grava só uma **aparição
nova** apontando para o arquivo que já existe. **Um arquivo, duas aparições.**

E existe um atalho antes disso: ao subir, o FrotaHub guarda **o id que o Trílogo
devolveu**. Na leitura seguinte o robô reconhece o id e **nem baixa**. O sha256
fica como rede de segurança, para quando alguém subir o mesmo arquivo pela tela do
Trílogo, onde não temos o id.

Ajuda ainda que **os orçamentos não ficam junto das fotos**: eles moram em
`invoiceFiles`, dentro do custo. São listas separadas — por isso a coleção entra na
chave da aparição.

### 2.3 A timeline tem um evento sem id

Quase todo evento traz id próprio. **Um tipo vem com `id = 0`.** Se a chave fosse
só o id, esses eventos entrariam de novo a cada leitura, para sempre. Para eles a
identidade é uma impressão digital do conteúdo (tipo + data-hora + autor + texto),
gravada com o prefixo `h:`.

### 2.4 As sete tabelas

| Tabela | Chave de dedup |
|---|---|
| `unidades` | cliente + id do Trílogo · carrega a marca **no escopo** |
| `chamados` | cliente + **número** |
| `chamado_eventos` | chamado + chave (id, ou impressão digital) |
| `chamado_custos` | chamado + id do custo |
| `arquivos` | **o sha256 do conteúdo** |
| `chamado_anexos` | chamado + coleção + id do anexo |
| `robo_execucoes` | o andamento de cada rodada |

Todas as chaves são **restrições do banco**, não disciplina de quem escreve o
código (**P-04**). Duplicar não é "evitado": é recusado.

### 2.5 A permissão chega ao dado, não só ao menu

Nasceu a função `posso('CODIGO_DA_ROTINA')`, e as políticas de leitura a usam.
Esconder o item na barra lateral seria teatro: bastaria pedir a tabela direto.

Como a rotina ainda não existe no catálogo, **hoje só o builder enxerga** — o
"nasce fechado" acontece sozinho, sem ninguém lembrar de fechar (**CORE-07**).

A tabela `arquivos` fica **sem política nenhuma**, de propósito: ela guarda o
caminho no R2 de todo arquivo do sistema. Quem precisa de um arquivo pede ao
motor, que confere o acesso e devolve um endereço temporário (**P-20**).

### 2.6 Só 4 unidades semeadas, não 38

As outras 34 o robô cria sozinho quando as encontra. Semear 38 linhas à mão seria
trabalho repetido, com risco de erro de digitação e de ficar velho no dia em que
abrir uma loja nova. As quatro fora do escopo precisam existir **antes**, porque a
regra depende delas.

### 2.7 Como foi conferido

As sete migrações aplicadas do zero, em ordem, num PostgreSQL de verdade. Depois,
cada regra exercitada contra o banco:

| Prova | Resultado |
|---|---|
| Mesmo chamado gravado como se fosse de outra conta | **recusado** |
| Chamado **trocando** de conta | aceito, e continua sendo **uma** linha |
| Reler a timeline inteira de novo | **recusada**, inclusive o evento sem id |
| Subir o orçamento e lê-lo de volta pelo id | **recusado** |
| O mesmo PDF chegando pela outra coleção | vira **2ª aparição** do **mesmo** arquivo |
| Apagar um arquivo ainda referenciado | **recusado** |
| Um login comum, sem a rotina na matriz | não enxerga nada |
| O mesmo login, depois de liberar a rotina | enxerga os chamados |
| Qualquer login pedindo a tabela `arquivos` | zero linhas |

---

## Step 3 — O robô

**Não começou.** Três modos, um código só:

- **Levantamento** — lê tudo desde 01/07/2026 e grava chamados, timeline, custos e
  a ficha de cada arquivo **com o tamanho real**, obtido do cabeçalho HTTP, **sem
  baixar nada**. No fim, o número exato de GB, por tipo, antes de gastar o primeiro
  byte de R2.
- **Cópia** — busca os bytes do que o dono escolher, transmitindo direto para o R2
  e calculando o sha256 no caminho.
- **Atualização** — a mesma leitura, com marca d'água: só os chamados cujo
  `dateOfLastChange` passou da última rodada concluída.

**Onde roda:** levantamento e cópia no Actions, no botão. A atualização mora no
motor, picotada e retomável — cada chamada processa um lote e anota onde parou,
o que protege contra o Render adormecer no meio. Dois gatilhos, uma implementação:
o botão do front e um agendamento no Actions chamando o mesmo endereço.

---

## Inventário da Fase 3

| Arquivo | Hospedado | Rev |
|---|---|---|
| `db/migrations/007_trilogo.sql` | repo · **Supabase** | 1 |

---

## Pendências da Fase 3

| O quê | Situação |
|---|---|
| Escrever o robô (3 modos) | próximo passo |
| Rodar o levantamento e decidir o que copiar para o R2 | depende do robô |
| Tela "Dados do Trílogo" e o cadastro da rotina no catálogo | depois do robô |
| Estimar o custo do R2 acima de 10 GB | ~US$ 0,015 por GB/mês, sem cobrança de saída |

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

*Onze. Declarados pelo dono do sistema. Valem para toda linha de código do FrotaHub.*

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

## Escala

**CORE-10 — Nada nasce com tamanho fixo.**
Toda lista é paginada e filtrada **no banco**, nunca no navegador. Todo limite é parâmetro,
não número solto no meio do código. Nenhuma tela é escrita supondo "são poucos". *Paginar
uma lista de três itens custa zero; despaginar uma de trinta mil custa uma reescrita.*

**CORE-11 — Todo dado de negócio tem um titular declarado.**
Nenhuma tabela de negócio existe sem dizer a quem o dado pertence, e o filtro por titular
mora na **política do banco**, não na consulta. *Filtro que depende de alguém lembrar de
escrever é filtro que um dia falta.*

---

# BLOCK — por bloco de módulos

*Nenhum ainda. O primeiro entra quando o primeiro grupo de módulos for construído.*

---

# MODULE — por módulo

## MOD-USUARIOS · Usuários e logins

**MOD-USUARIOS-01 — Mexer em login deixa rastro.**
Criar, alterar, mudar situação e trocar senha gravam histórico com data, hora, quem
executou e o que mudou. Senha e PIN **nunca** entram no conteúdo — nem cifrados, nem em
pedaço, nem o tamanho. O histórico é **só-inserção**: não se edita e não se apaga, e a
garantia é do banco, não da disciplina de quem escreve o código. *Declarada pelo dono do
sistema em 23/08/2026. Verbos deste módulo: `criou`, `alterou`, `desativou`, `reativou`,
`trocou_senha`.*

## MOD-ACESSO · Categorias e matriz de permissões

**MOD-ACESSO-01 — Mexer em permissão deixa rastro.**
Criar e alterar categoria, tirá-la ou devolvê-la à circulação, e cada salvamento da
matriz gravam histórico com data, hora, quem executou e o que mudou. A matriz gera
**uma linha por salvamento**, listando só as rotinas que mudaram de estado. Vale a
mesma tranca: só-inserção, garantida pelo banco. *Declarada pelo dono do sistema em
23/08/2026. Motivo: mexer em permissão é mais sensível que mexer em login — é ali que
alguém ganha ou perde acesso. Verbos deste módulo: `criou`, `alterou`, `desativou`,
`reativou`, `alterou_permissoes`.*

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

**P-29 — Facilidade de operação é requisito, não acabamento.**
Toda tela, mensagem e fluxo é desenhada a partir de quem vai usar: menos passos na tarefa
do dia a dia, as coisas com o nome que quem trabalha usa, o caminho certo sendo o mais
fácil de seguir, e nada que dependa de decorar. Quando a escolha for entre o que é simples
de programar e o que é simples de operar, **ganha operar**. *Sistema que funciona mas custa
esforço todo dia é sistema que as pessoas contornam — e o contorno vira a verdade, não o
sistema. Declarada pelo dono em 23/08/2026.*

## Operação

**P-20 — O motor não serve arquivo.**
Ele entrega endereço temporário e o arquivo vai direto da nuvem ao usuário.

**P-21 — Publicação vazia é pior que publicação falha.**
Antes de enviar, confere-se que a compilação gerou algo.

**P-22 — Quem executa é gente.**
Arquivo que roda código no repositório é criado por uma pessoa. *Origem: a escrita remota
em `.github/workflows/` é bloqueada de propósito — e a regra faz sentido.*

**P-23 — Regra de exclusão de arquivo aponta caminho, nunca nome solto.**
No `.gitignore`, um nome sem barra casa com **pasta** também. *Origem: a linha `baleryan`,
escrita para ignorar o binário, excluiu do envio a pasta inteira do motor — em silêncio,
sem erro, e o defeito só apareceu no servidor de publicação dizendo que a pasta não
existia.*

**P-24 — Antes de entregar, roda.**
Migração vai para um Postgres de verdade, do zero, na ordem. Motor compila e responde.
*As duas falhas de desenho desta fase — a ordem das migrações e o histórico que travava a
remoção de login — foram encontradas assim, e nenhuma chegou ao usuário.*

**P-25 — Espera longa se explica.**
Passou de alguns segundos, a tela diz o que está esperando. *Origem: o motor no plano
gratuito adormece, e a primeira chamada do dia leva quase um minuto. Tela em branco parece
defeito, e quem acha que é defeito recarrega a página — o que reinicia a espera.*

**P-26 — Tela também se abre num navegador antes de entregar.**
Não basta compilar. *Origem: os dois defeitos visuais da entrega 2 — pontos vermelhos
vazando para dentro dos itens e um cabeçalho de tabela órfão no erro — passaram por
compilação e verificação de tipos sem uma reclamação.*

**P-27 — O teste confere o estado, não só a tela.**
Depois de uma ação, verifica-se também o que ficou guardado. *Origem: ao expirar a sessão,
a tela certa aparecia — e, atrás dela, o carimbo de uso era regravado logo após a limpeza.
Um teste que olhasse só a tela teria dado tudo certo.*

**P-28 — Arquivo que muda de lugar exige apagar o antigo, à mão.**
A entrega escreve o novo; ninguém apaga o velho sozinho. *Origem: duas peças mudaram de
pasta na entrega 3, as cópias antigas ficaram na máquina, o `git add -A` recolheu as duas,
e a compilação quebrou. Toda entrega que MOVE arquivo vem acompanhada do comando que
remove o original.*

---

---

## Endereços e contas

| O quê | Onde | Existe? |
|---|---|---|
| Repositório | `github.com/iafrotamacedo-cloud/frotahub-v2` | sim, público |
| Banco | `https://hltcngamdqabqlocufrv.supabase.co` (us-east-1) | sim, 6 tabelas |
| Chave pública do banco | `sb_publishable_IyCf5yioEo-Ry8q5mB2boA__ekxvPIu` | é chave de navegador, não é segredo |
| Arquivos | Cloudflare R2, balde `frotahub`, conta `99f23938821d056868ecef92c08eed7f` | sim, vazio |
| Front | `https://novo.frotamacedo.com.br` | **no ar** |
| Hospedagem | HostGator Plano P, servidor `br1000`, IP `162.241.203.77` | sim |
| Motor | `https://baleryan.onrender.com` — Render, região Ohio | **no ar** |
| Arquivo-mestre | Dropbox | conta existente |

A `service_role` do Supabase, a senha do banco e a chave secreta do R2 **não** ficam neste
arquivo, por escolha.

---

*As fases seguintes entram abaixo, uma seção cada, no mesmo formato de Steps.*
