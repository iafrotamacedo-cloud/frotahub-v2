# FrotaHub — Diário de Bordo

> Registro do projeto, organizado em **Fases** e, dentro delas, em **Steps**.
> Cada step lista o que foi definido, feito e criado — plataforma por plataforma.
> Quem ler só este arquivo sabe onde estamos, o que existe e qual é o próximo passo.
>
> Última atualização: **23/08/2026** · Rodada 13 · **Fase aberta: 0 (concluída)**

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
> **sistema** ficam no **Anexo A — Tenets**, organizadas em cinco níveis (Core, Block,
> Module, Routine, Skill), com códigos permanentes para referência.

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

### 4.2 Árvore de menus — **planejada**

Seis menus, 79 rotinas. O código em `crase` é a permissão que controla o acesso. O menu se
ajusta sozinho: quem não tem a permissão não vê o item; rotina ainda não construída aparece
desabilitada.

- Administrativo
  - PCO
    - Ordens de compra  `ENVIAR_PCO`
    - Enviar PCO  `ENVIAR_PCO`
    - Planilhas de Controle
      - PCOs solicitados  `PCOS_ENVIADOS`
      - O.C.s não enviadas  `OC_INVALIDA`
  - Notas Fiscais
    - Procurar nota  `PROCURAR_NOTA`
    - Conferir nota (obra)  `CONFERIR_NOTA`
    - Receber nota física  `RECEBER_NOTA`
    - Visualizar notas entregues  `PROCURAR_NOTA`
    - Protocolo  `GERAR_PROTOCOLO`
    - Panorama
      - Pendências  `PENDENCIAS_NOTA`
      - Vencimentos  `VENCIMENTOS_NOTA`
  - Locações
    - Relação de locações  `RELACAO_LOCACOES`
    - Nova locação  `NOVA_LOCACAO`
    - Nova devolução  `NOVA_DEVOLUCAO`
    - Procurar demonstrativos  `PROCURAR_DEMONSTRATIVO`
    - Panorama financeiro  `FINANCEIRO_LOCACAO`
  - Compras
    - Equalizar orçamentos  `EQUALIZAR_ORCAMENTOS`
- Manutenção
  - Contrato São Luiz
    - Lista do Trílogo  `CHAMADOS_TRILOGO`
    - Treinamento  `TREINAMENTO`
    - Expectativa × Realidade  `EXPECTATIVA_REAL`
    - Indicadores de Qualidade  `INDICADORES_QUALIDADE`
    - Orçamentos
      - Notas e DAVs  `GERAR_ORCAMENTOS`
      - Gerar orçamentos  `GERAR_ORCAMENTOS`
      - Reprocessar pendentes (builder)  `GERAR_ORCAMENTOS`
      - Lançar no Trílogo  `GERAR_ORCAMENTOS` · `LANCAR_TRILOGO`
      - Orçamentos apagados  `AUDITORIA`
      - Planilhas de Controle
        - Orçamentos gerados  `ORCAMENTOS_GERADOS`
        - Notas sem ticket  `SEM_TICKET`
        - Ticket não associado  `TICKET_NAO_ASSOCIADO`
        - Orçamentos anexados  `ORCAMENTOS_UPADOS`
    - Estatística
      - Dashboard  `DASHBOARD`
      - Indicativos de Mau Uso  `DASHBOARD`
      - Orçamentos extrapolados  `GERAR_ORCAMENTOS`
      - Planilha de perdas  `FIN_A_PAGAR`
      - Financeiro de materiais  `FINANCEIRO_MATERIAIS`
    - Manutenção Preventiva
      - Plano de Manutenção Preventiva  `MANUTENCAO_PREVENTIVA`
      - Realizar inspeção (Check Lists)  `MANUTENCAO_PREVENTIVA`
      - Calendário  `CALENDARIO_PREVENTIVA`
      - Relatórios
        - Relatório de loja  `RELATORIO_LOJA`
        - Relatório mensal  `RELATORIO_MENSAL`
        - Relatório geral  `RELATORIO_GERAL`
    - Financeiro
      - A Pagar  `FIN_A_PAGAR`
      - A Receber  `FIN_A_RECEBER`
      - Balanço  `FIN_BALANCO`
  - Outros
    - Em breve  `MANUT_OUTROS`
- Engenharia
  - Engenharia (em breve)  `ENGENHARIA_HOME`
- SESMT
  - APR — Análise Preliminar de Risco  `SESMT_APR`
  - PT — Permissão de Trabalho  `SESMT_PT`
- Auditoria
  - Materiais comprados
    - Lista de classificação  `AUDITORIA`
    - Orçamentos para auditoria  `AUDITORIA`
    - Abrir orçamento por ticket  `AUDITORIA`
- Configuração
  - Minha conta
  - Usuários e Logins  `CONFIG_USUARIOS`
  - Configurar logins  `CONFIG_CATEGORIAS`
  - Política de PIN  `CONFIG_PIN`
  - Agendamento de tarefas  `CONFIG_AGENDAMENTO`
  - Configurações gerais  `CONFIG_GERAL`
  - Ferramentas do Builder  `CONFIG_BUILDER`

Existe também uma **árvore separada para contas de cliente** (permissão `CLIENTE_MSL`), com
treinamento, preventiva e indicadores do contrato. O cliente entra no mesmo sistema e não
enxerga nada de dentro de casa.

**Sete níveis de login:** builder, gerente, encarregado/engenheiro, administrativo,
manutenção, almoxarife e SESMT — mais o CEO, para auditoria. A permissão é **por rotina**,
não por menu: dá para liberar "conferir nota" sem liberar "receber nota física".

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

**Passo 1 do sistema:** a casca (login e tela inicial em React) e o motor base em Go.
Nada de rotina de negócio ainda — só o esqueleto que sobe, autentica e mostra o menu certo
para cada login.

---
---

# ANEXO A — Tenets

> **O que são.** As regras de ouro do FrotaHub. Cada uma tem um código permanente, e sempre
> que uma decisão se apoiar numa delas, o texto cita o código em vez de repetir o argumento
> — por exemplo: *"o arquivo não se move quando o estado muda (**CORE-04**)"*.
>
> **Um tenet só entra se tiver custado alguma coisa** — um erro real, uma decisão difícil,
> um problema que voltou. Não é lista de boas intenções.

## Os cinco níveis

| Nível | Abrangência | Sentido | Código |
|---|---|---|---|
| **Core** | O sistema como um todo | Princípios fundamentais e **imutáveis** | `CORE-01` |
| **Block** | Um grupo de módulos (ex.: Manutenção) | Regras que valem para aquele bloco | `BLOCK-MNT-01` |
| **Module** | Um módulo específico (ex.: Orçamentos) | Regras internas daquele módulo | `MOD-ORC-01` |
| **Routine** | Uma função ou rota específica | Regras daquela ação | `ROT-LANCAR-01` |
| **Skill** | Um robô ou automação | Regras daquela automação | `SKILL-TRILOGO-01` |

## As três leis dos tenets

**Herança.** Toda rotina obedece aos tenets do seu módulo, do seu bloco e do Core. Escrever
uma rotina é escrever dentro dessa pilha — o que já está garantido acima não se reescreve.

**Especificidade.** Quando dois tenets se cruzam, vale o mais específico — **exceto contra
o Core**, que nunca é sobreposto. Se um módulo precisa violar um tenet Core, ou o desenho
do módulo está errado, ou o tenet Core estava errado. Nos dois casos, para-se e discute-se;
não se contorna.

**Permanência.** Código de tenet nunca é reaproveitado nem renumerado. Tenet revogado fica
na lista marcado como revogado, com a data e o motivo. *Motivo: referência antiga precisa
continuar significando a mesma coisa.*

---

# CORE — o sistema como um todo

*Imutáveis. Valem para toda linha de código do FrotaHub, hoje e depois.*

## Verdade e estado

**CORE-01 — O banco é a única fonte da verdade.**
Fila de trabalho, estado e localização de arquivo saem do banco. Nenhuma rotina descobre o
que fazer varrendo pasta.

**CORE-02 — Arquivo é registro.**
Toda escrita em nuvem gera linha na tabela de arquivos. Achar um documento é consulta, não
busca por nome.

**CORE-03 — Nada de estado na memória do servidor.**
Tarefa demorada é linha no banco, com progresso. *Motivo: servidor que hiberna, reinicia ou
roda em duas cópias perde tudo que estiver só na memória.*

**CORE-04 — O estado não mora na pasta; o arquivo não se move.**
Um orçamento lançado é uma coluna no banco, não um arquivo mudado de pasta. *Motivo: mover
arquivo para representar estado gera arquivo perdido, movimentação pela metade e
reconciliação — uma família inteira de defeitos que deixa de existir.*

## Dados

**CORE-05 — Nada é apagado de vez.**
Remover é marcar. A garantia é do **banco**, não da disciplina de quem escreve o código.

**CORE-06 — O que é regra vira restrição.**
Se a regra pode ser expressa como restrição do banco, ela é uma restrição — não um
comentário nem uma condição no código. *Motivo: comentário não impede nada.*

**CORE-07 — Mudança de estrutura é migração numerada.**
Nunca comando solto. O banco guarda o que já rodou, e ninguém fica na dúvida sobre o estado.

**CORE-08 — Item é linha, não bloco de texto.**
O que precisa ser consultado, somado ou auditado é tabela. *Motivo: dado enterrado em texto
não responde pergunta.*

**CORE-09 — Sequência vem do banco.**
Numeração de documento sai de sequência do próprio banco. *Motivo: contar o maior e somar
um colide quando duas pessoas clicam junto.*

**CORE-10 — Parâmetro de negócio mora no banco, não no ambiente.**
Margem, teto e limites ficam em tabela com histórico de quem mudou e quando. *Motivo:
variável de ambiente não deixa rastro, e valor de negócio precisa ser auditável.*

## Código

**CORE-11 — Cada regra existe uma vez só.**
Margem, teto, duplicidade e roteio moram num lugar só, em código puro, sem banco e sem
internet — e por isso testável de verdade.

**CORE-12 — Falhe na largada, não no meio do expediente.**
Configuração incompleta impede o programa de subir, e a reclamação lista **tudo** que falta
de uma vez. *Motivo: programa que sobe quebrado transforma erro de configuração em problema
do usuário, horas depois.*

**CORE-13 — Erro estoura; não vira aviso no console.**
Gravação que falhou não pode seguir como se tivesse gravado.

**CORE-14 — Dinheiro é decimal, e meio centavo sobe.**
Nunca número de ponto flutuante. *Motivo: o arredondamento padrão de várias linguagens manda
2,675 para 2,67.*

**CORE-15 — Comentário explica o porquê, não o quê.**
O código já diz o que faz. O comentário guarda a decisão, a alternativa descartada e a
armadilha.

**CORE-16 — Um assunto, um lugar.**
Mexer no cálculo é abrir um arquivo; mexer numa tela é abrir outro. Nunca rolar milhares de
linhas procurando.

## Segurança

**CORE-17 — Uma porta de entrada só.**
Um lugar diz **quem é**; outro diz **o que pode**. Nada além disso decide acesso.

**CORE-18 — Na dúvida, nega.**
Falha ao verificar permissão resulta em acesso negado, nunca em acesso concedido.

**CORE-19 — Nada é público por padrão.**
Tabela nasce com leitura fechada; balde nasce sem endereço público.

**CORE-20 — Credencial só enxerga o que precisa.**
*Motivo: a pergunta certa não é "vai vazar?", é "se vazar, o estrago é onde?"*

**CORE-21 — Segredo não passa pela conversa.**
Senha de banco, chave de serviço e chave secreta vão direto no painel do serviço.

**CORE-22 — O registro é auditável, não apagável.**
O log de atividades não se reescreve. E falha no registro nunca desfaz a operação já feita.

## Interface

**CORE-23 — O menu se ajusta ao login.**
Quem não tem a permissão não vê o item. Rotina ainda não construída aparece desabilitada,
para dar a medida do que falta.

**CORE-24 — Mensagem de erro é escrita para ser lida.**
"Sessão expirada", não "401 unauthorized". *Motivo: quem lê o erro é o usuário.*

**CORE-25 — A identidade da casa é a mesma em todo lugar.**
Sidebar escura, `#A11F22` como acento, Inter, ícones. Nada de biblioteca com visual próprio.

## Operação

**CORE-26 — O motor não serve arquivo.**
Ele entrega endereço temporário e o arquivo vai direto da nuvem ao usuário.

**CORE-27 — O que está no ar continua no ar.**
Sistema novo convive com o antigo em endereço separado até a virada, e a virada tem volta.

**CORE-28 — Perto é rápido.**
Motor, banco e arquivos ficam geograficamente próximos. *Origem: ~12 ms entre Ohio e
Virgínia contra ~120 ms até São Paulo — a mesma tela, dez vezes mais rápida, de graça.*

**CORE-29 — Publicar é apertar um botão, não seguir um roteiro.**
Nenhum passo manual entre o código aprovado e o ar.

**CORE-30 — Publicação vazia é pior que publicação falha.**
Antes de enviar, confere-se que a compilação gerou algo. *Motivo: envio "bem-sucedido" de
pasta vazia apaga o que estava no ar.*

**CORE-31 — Grátis tem conta.**
Todo plano gratuito tem um limite que um dia chega, e alguém precisa estar olhando.
*Origem: a cota de Actions estourada pelos robôs do Trílogo derrubou a primeira publicação.*

**CORE-32 — Quem executa é gente.**
Arquivo que roda código no repositório é criado por uma pessoa. *Origem: a escrita remota em
`.github/workflows/` é bloqueada de propósito — e a regra faz sentido.*

---

# BLOCK — por bloco de módulos

*Preenchido conforme cada bloco é construído. Herda todo o Core.*

## BLOCK-ADM · Administrativo
> PCO, Notas Fiscais, Locações, Compras.

**BLOCK-ADM-01 — A O.C. tem quatro marcos, e nenhum se pula.**
PCO solicitado → nota conferida → nota entregue → nota protocolada. Uma ordem de compra
pode gerar várias notas (recebimento parcial).

**BLOCK-ADM-02 — Recebimento de nota física é marco restrito.**
Só gerência alcança. *Motivo: é o marco que confirma posse do documento fiscal.*

## BLOCK-MNT · Manutenção
> Chamados, Orçamentos, Rateio, Preventiva, Financeiro do contrato.

**BLOCK-MNT-01 — O ticket é do cliente; o `id` é nosso.**
O sistema é governado pelo identificador interno, nunca pelo número do ticket — ticket muda
quando uma nota sem-ticket é corrigida.

**BLOCK-MNT-02 — Nada vai para o cliente sem conferência de valor.**
Toda escrita no sistema do cliente passa pelas travas de status e de teto.

## BLOCK-ENG · Engenharia
*A definir quando o bloco começar.*

## BLOCK-SST · SESMT
*A definir quando o bloco começar.*

## BLOCK-AUD · Auditoria
**BLOCK-AUD-01 — Quem audita não opera.**
A auditoria é restrita a builder e CEO; gerência não audita. *Motivo: separação de funções —
quem aprova a compra não pode ser quem julga se ela era permitida.*

## BLOCK-CFG · Configuração
**BLOCK-CFG-01 — Permissão é por rotina, não por menu.**
Dá para liberar "conferir nota" sem liberar "receber nota física".

---

# MODULE — por módulo

*Preenchido conforme cada módulo é construído. Herda o Core e o seu Block.*

## MOD-ORC · Orçamentos
**MOD-ORC-01 — A duplicidade é da NOTA, não do ticket.**
Um ticket pode ter vários orçamentos de notas diferentes; a mesma nota nunca gera dois.

**MOD-ORC-02 — O total é a soma do que está impresso.**
Arredonda-se o valor unitário e o total é a soma das linhas do papel. *Motivo: quem confere
é gente, com calculadora, olhando o documento.*

**MOD-ORC-03 — Um documento é nota fiscal ou DAV, nunca os dois.**

## MOD-PCO · PCO
*A definir quando o módulo começar.*

## MOD-NF · Notas Fiscais
*A definir quando o módulo começar.*

---

# ROUTINE — por rotina

*Preenchido conforme cada rotina é construída. Herda tudo acima.*

## ROT-LANCAR · Lançar orçamento no Trílogo
**ROT-LANCAR-01 — Só lança em ticket Executado ou Vistoriado.**

**ROT-LANCAR-02 — A soma dos custos do ticket mais o novo não passa do teto.**
Se passar, aborta e o orçamento vira extrapolado, à espera de aprovação.

**ROT-LANCAR-03 — Suspeito não lança sem liberação do CEO.**

---

# SKILL — por robô ou automação

*Preenchido conforme cada automação é construída.*

## SKILL-PUBLICAR · Publicação do front
**SKILL-PUBLICAR-01 — Confere antes de enviar.**
Se a compilação não gerou o arquivo, aborta sem publicar (aplicação de **CORE-30**).

**SKILL-PUBLICAR-02 — Envio nunca concorre com envio.**
Dois disparos ao mesmo tempo viram fila, não corrida.

## SKILL-TRILOGO · Robô do Trílogo
**SKILL-TRILOGO-01 — Toda operação destrutiva tem ensaio.**
Roda primeiro em modo relatório, sem aplicar, e o padrão é o modo seguro.

**SKILL-TRILOGO-02 — Robô não lança arquivo sem registro no banco.**

---

## Como referenciar

No diário, no código e nas conversas, cite o código entre parênteses:

> A fila de lançamento vem do banco, não da pasta (**CORE-01**, **CORE-04**).
> A conta de FTP está trancada no subdomínio (**CORE-20**).
> Este orçamento não pode ser lançado porque o ticket não está executado (**ROT-LANCAR-01**).

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
