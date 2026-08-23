# FrotaHub — Diário de Bordo

> Registro do projeto, **organizado por fase**. Cada fase ganha a sua seção quando é aberta;
> dentro dela ficam o que foi decidido, o que foi feito, onde está hospedado e em que revisão.
> Quem ler só este arquivo sabe onde estamos e qual é o próximo passo.
>
> Última atualização: **23/08/2026** · Rodada 10 · **Fase aberta: 0**

---

## O que é o FrotaHub

O sistema interno da **Frota Macedo Engenharia**: um só lugar, com um só login, para as rotinas
dos quatro departamentos.

- **Administrativo** — envio de PCO, ciclo das notas fiscais, locações e compras.
- **Manutenção** — a lista de chamados do Trílogo, os orçamentos de material (leitura da nota,
  geração, lançamento), a manutenção preventiva e o financeiro do contrato.
- **Engenharia** — prontuário elétrico NR-10 e projetos.
- **SESMT** — os formulários de APR e Permissão de Trabalho.

Mais **Auditoria** (o que foi comprado, por quem, e se era permitido) e **Configuração**
(logins, permissões, PIN, agendamentos).

Roda 100% na nuvem, em serviços de plano gratuito, com acesso pelo computador e pelo celular.

---

## Estado atual

| | |
|---|---|
| **Fase aberta** | 0 — Fundação e alinhamento |
| **Repositório** | `github.com/iafrotamacedo-cloud/frotahub-v2` — criado, **vazio** |
| **Banco** | Supabase `hltcngamdqabqlocufrv` — criado, **vazio** |
| **Motor** | ainda não publicado |
| **Front** | ainda não publicado |
| **Bloqueio** | Falta o **PAT** do GitHub para eu poder commitar |
| **Próximo passo** | Passo 1: a casca (login e tela inicial) e o motor base em Go |

---

## O que existe hoje, de concreto

Tudo que está criado neste momento — e só isso:

| O quê | Endereço | Estado |
|---|---|---|
| Repositório | `github.com/iafrotamacedo-cloud/frotahub-v2` | **vazio** — nenhum arquivo |
| Projeto Supabase | `https://hltcngamdqabqlocufrv.supabase.co` | **vazio** — nenhuma tabela |
| Chave pública do Supabase | `sb_publishable_IyCf5yioEo-Ry8q5mB2boA__ekxvPIu` | é a chave de navegador, não é segredo |
| Conexão direta do banco | `postgresql://postgres:<SENHA>@db.hltcngamdqabqlocufrv.supabase.co:5432/postgres` | a senha fica com você |

**Nada além disso existe.** Não há código, não há tabela, não há nada publicado. As árvores das
seções 0.2 e 0.3 são o **desenho do que vai ser construído** — nenhuma daquelas pastas existe ainda.

A chave `service_role` e a senha do banco não estão aqui e não precisam passar pela conversa: elas
vão direto nas variáveis de ambiente do Render, quando chegar a hora.

---

## As regras de trabalho

1. **Explico antes, você autoriza, aí eu subo.** Nada entra no repositório sem o seu "pode" —
   mesmo depois que eu tiver acesso de escrita.
2. **Nada de uma vez.** Sobe pouco a pouco, módulo a módulo, na ordem que você mandar.
3. **Amostra primeiro.** Antes de uma leva grande, entrego os arquivos-chave no chat para você
   ver o estilo.
4. **Segredo não passa pelo chat.** Senha de banco e chave `service_role` você põe direto no
   Supabase e no Render. Eu não preciso ver.
5. **Este diário é atualizado a cada rodada**, e dividido por fase conforme você as abre.
6. **Todo arquivo tem endereço e revisão.** "Está no chat", "está no repositório" e "está rodando
   em produção" são três coisas diferentes — o inventário diz qual é qual.

---

# FASE 0 — Fundação e alinhamento

**O que é:** fechar o desenho e preparar o alicerce. Nenhuma rotina de negócio, nada publicado.

**Como sei que terminou:** o repositório tem estrutura, documentação e a fundação do motor; as
regras de preço passam nos testes; a casca sobe, autentica e mostra o menu certo para cada login.

---

## 0.1 A arquitetura

### Os seis princípios

**1. O banco é a única fonte da verdade.**
A fila de trabalho, o estado de cada orçamento e a localização de cada arquivo saem do banco.
Nenhuma rotina descobre o que fazer varrendo pasta: o arquivo é consequência do registro, nunca
o contrário.

**2. Cada regra existe uma vez só.**
A margem, o teto de lançamento, a regra de duplicidade e o roteio de pastas moram numa pasta
chamada `dominio` — código puro, que não fala com banco, internet ou nuvem. Isso tem uma
consequência prática grande: **dá para testar de verdade**, e um teste que roda em milissegundos
é um teste que se roda toda hora.

**3. Arquivo é registro.**
Toda escrita em nuvem gera uma linha na tabela `arquivos`, com o endereço, o tamanho e a impressão
digital do conteúdo. Achar um PDF é uma consulta, não uma busca por nome.

**4. Nada de estado na memória do servidor.**
Tarefa demorada — gerar orçamentos, ratear, reprocessar — é linha na tabela `jobs`. Se o servidor
cair e voltar no meio, ela continua de onde parou. Isso importa porque o plano gratuito do Render
hiberna o serviço depois de alguns minutos parado.

**5. Uma porta de entrada só.**
Um lugar recebe a credencial (do usuário, do robô ou do agendador) e diz **quem é**. Outro diz
**o que pode**, por rotina. Nada além disso decide acesso.

**6. Mudança de banco é migração numerada.**
Cada mudança de estrutura é um arquivo com número, e o banco guarda quais já rodaram. Você sempre
sabe em que estado ele está, e nunca se roda a mesma coisa duas vezes.

### As peças e como conversam

```
        NAVEGADOR                    NUVEM DO MOTOR              NUVENS DE APOIO
   ┌────────────────┐            ┌──────────────────┐         ┌──────────────────┐
   │  FrotaHub web  │            │    baleryan      │         │ Supabase (banco  │
   │  (Netlify)     │───HTTPS───▶│    (Render)      │────────▶│ e login)         │
   │                │            │                  │         └──────────────────┘
   │ React + Vite   │            │  Go, binário     │         ┌──────────────────┐
   │ TypeScript     │            │  único           │────────▶│ Cloudflare R2    │
   │ 1 tela =       │◀──URL──────│                  │         │ (arquivos vivos) │
   │ 1 arquivo      │  assinada  │                  │         └──────────────────┘
   └────────────────┘            │                  │         ┌──────────────────┐
                                 │                  │────────▶│ Dropbox          │
                                 │                  │         │ (arquivo-mestre) │
                                 │                  │         └──────────────────┘
                                 └────────┬─────────┘         ┌──────────────────┐
                                          └─────────────────▶ │ Trílogo, Gemini, │
                                                              │ e-mail, GitHub   │
                                                              └──────────────────┘
```

**A seta da URL assinada** merece atenção: o motor não serve arquivo. Ele entrega ao navegador um
endereço temporário e o PDF vai **direto** da Cloudflare para o usuário. O servidor não vira
gargalo, e o R2 não cobra saída de dados.

**Por que o motor é um binário Go:** ele sobe em milissegundos. Como o plano gratuito do Render
hiberna, o primeiro acesso do dia acorda o serviço — com Go isso é imperceptível.

**Por que dois lugares para arquivo:** o R2 guarda o fluxo vivo (entrada de notas, orçamentos a
lançar, lançados, pendências). O Dropbox guarda o **arquivo-mestre**, uma cópia de tudo que já foi
gerado, com lixeira e histórico nativos. É a rede de segurança.

### O modelo de dados

| Tabela | O que guarda |
|---|---|
| `documentos` | A nota fiscal ou DAV lida: número, fornecedor, comprador, data, valor, como foi lida |
| `documento_itens` | Os itens dessa nota, um por linha |
| `orcamentos` | O orçamento gerado: ticket, loja, valor, se foi lançado, se é rateio, se extrapolou |
| `orcamento_itens` | O que está impresso no papel, com o preço da nota e o preço cobrado lado a lado |
| `orcamento_documentos` | O vínculo entre os dois — é o que permite **uma nota atender vários tickets** (rateio) |
| `arquivos` | Onde cada arquivo está, em que nuvem, com que tamanho e que impressão digital |
| `jobs` | As tarefas demoradas e o progresso de cada uma |
| `parametros` | A margem, o teto, o limite de extrapolado — com histórico de quem mudou e quando |
| `chamados` | O espelho da lista do Trílogo |
| `perfis`, `categorias`, `categoria_permissoes` | Quem entra e o que cada um pode |
| `log_atividades` | Tudo que foi feito, por quem e quando |

Duas escolhas que valem explicação. **Os itens são tabela, não bloco de texto** — é o que permite
a auditoria perguntar "quem comprou ferramenta este mês" com uma consulta simples. E **documento e
orçamento são coisas separadas, ligadas por uma terceira tabela** — é o que faz o rateio ser um
caso normal do modelo, e não uma exceção.

### As regras que o banco impõe sozinho

Coisas que não dependem de ninguém lembrar:

- Nada é apagado de vez. `DELETE` é bloqueado nas tabelas de registro; remover é marcar.
- Um documento é nota fiscal **ou** DAV, nunca os dois.
- A mesma nota não pode gerar orçamento duas vezes.
- Os números de sequência (pedidos de fatura, lotes) saem do banco, então duas pessoas clicando
  ao mesmo tempo não colidem.
- Cada campo de situação só aceita os valores da sua lista.

---

## 0.2 A árvore do repositório — **planejada**

> Nada disto existe ainda. É o desenho de onde cada coisa vai ficar quando for construída.

```
frotahub-v2/
├── README.md                 o mapa: onde fica cada coisa, como publicar
├── docs/
│   ├── diario-de-bordo.md    ESTE arquivo
│   └── decisoes/             uma decisão por arquivo: o quê, por quê, o que foi descartado
│
├── db/
│   ├── migrations/           001_..., 002_... — mudanças numeradas, aplicadas em ordem
│   ├── seed/                 carga inicial: categorias, permissões, parâmetros
│   └── aplicar.py            confere o que já rodou antes de rodar
│
├── motor/                    ← o baleryan, em Go
│   ├── go.mod
│   ├── Dockerfile
│   ├── cmd/baleryan/
│   │   └── baleryan.go       o ponto de partida do programa
│   └── interno/
│       ├── config/           variáveis de ambiente tipadas e validadas na subida
│       ├── seguranca/        quem é você (usuário, robô ou agendador)
│       ├── permissao/        o que você pode (por rotina) + PIN
│       ├── auditoria/        o log de atividades
│       ├── banco/            o cliente do Supabase
│       ├── arquivos/         R2, Dropbox e o roteador que escolhe entre os dois
│       ├── dominio/          AS REGRAS: preço, teto, duplicidade, NF × DAV
│       ├── documentos/       geração de PDF e planilha
│       ├── servicos/         leitura de nota (OCR e IA), Trílogo, e-mail
│       ├── tarefas/          a fila de jobs e quem executa cada tipo
│       └── modulos/          uma pasta por módulo: PCO, notas, orçamentos, auditoria...
│
├── web/                      ← React + Vite + TypeScript
│   ├── package.json · vite.config.ts · tsconfig.json
│   ├── index.html            só o ponto de montagem
│   ├── public/               logo, favicon, manifest
│   └── src/
│       ├── main.tsx          entrada
│       ├── App.tsx           a casca: sidebar + conteúdo + rotas
│       ├── estilos/          tokens.css e base.css — a identidade da Frota Macedo
│       ├── api/              cliente do motor + os tipos espelhando o Go
│       ├── sessao/           login, token, permissões
│       ├── componentes/      Tabela, Modal, Toast, Upload, KPI, Formulario, Skeleton
│       ├── menu/             a árvore de menus + o filtro por permissão
│       └── telas/            1 PASTA POR MÓDULO, 1 arquivo por tela (carregada sob demanda)
│
└── robos/                    GitHub Actions: chamados, lançamento, manutenção
```

**A regra de ouro da organização:** para mexer no cálculo do orçamento, abra
`motor/interno/dominio/preco.go`. Para mexer na tela de lançamento, abra
`web/src/telas/manutencao/LancarTrilogo.tsx`. Cada assunto tem um lugar, e só um.

---

## 0.3 A árvore de menus — **planejada**

> O desenho do menu completo. Nenhuma tela está construída.

Seis menus, 79 rotinas. O código em `crase` é a permissão que controla o acesso àquele item — o
menu se ajusta sozinho ao login: quem não tem a permissão não vê o item. Rotina ainda não
construída aparece desabilitada, para dar a medida do que falta.

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
      - Lançar no Trílogo  `GERAR_ORCAMENTOS`
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
treinamento, preventiva e estatísticas do contrato. O cliente entra no mesmo sistema e não enxerga
nada de dentro de casa.

### Os sete níveis de login

Builder, gerente, encarregado/engenheiro, administrativo, manutenção, almoxarife e SESMT — mais o
CEO, para auditoria. A permissão é **por rotina**, não por menu: dá para liberar "conferir nota"
sem liberar "receber nota física". Toda ação fica registrada com quem fez e quando.

---

## 0.4 Inventário — onde cada arquivo está e em que revisão

**Como funciona a revisão.** Todo arquivo nasce em **rev 1** e sobe a cada mudança. O número fica
em dois lugares que têm que bater: um comentário na primeira linha do próprio arquivo e a tabela
abaixo. Se divergirem, vale o comentário dentro do arquivo — a tabela é índice, o arquivo é a verdade.

**Onde "hospedado" pode ser:**

| Valor | Significa |
|---|---|
| `repo` | Está no GitHub `frotahub-v2`, no caminho indicado. É a fonte. |
| `Render` | Está compilado e rodando no motor. |
| `Netlify` | Está publicado e no ar para o usuário. |
| `Supabase` | Migration **já aplicada** no banco — não basta estar no repositório. |
| `chat` | Só foi entregue na conversa. **Não está hospedado em lugar nenhum.** |
| `Projeto` | Está no Projeto do Claude, visível em qualquer conversa. |

### Hoje

| Arquivo | Hospedado | Rev | O que é |
|---|---|---|---|
| `docs/diario-de-bordo.md` | Projeto | 5 | Este arquivo |

**É só isso.** O repositório está vazio e o banco também. Nenhuma linha de código foi escrita
ou hospedada até aqui — o que existe é o desenho e as duas contas que você criou.

### O que entra no Passo 1

Lista fechada, para conferir depois se chegou tudo. Todos nascem em rev 1.

| Arquivo | Vai ficar em | O que faz |
|---|---|---|
| `README.md` | repo | O mapa do projeto |
| `.gitignore` | repo | O que não entra no controle de versão |
| `docs/diario-de-bordo.md` | repo + Projeto | Este arquivo, também versionado |
| `motor/go.mod` | repo | As dependências do motor |
| `motor/Dockerfile` | repo | Como o Render compila e roda |
| `motor/cmd/baleryan/baleryan.go` | repo | O ponto de partida: sobe, escuta, monta as rotas |
| `motor/interno/config/config.go` | repo | Variáveis de ambiente tipadas; recusa subir se faltar |
| `motor/interno/seguranca/seguranca.go` | repo | Quem é você: usuário, robô ou agendador |
| `motor/interno/permissao/permissao.go` | repo | O que você pode, por rotina, mais o PIN |
| `motor/interno/auditoria/log.go` | repo | O registro de atividades |
| `motor/interno/banco/cliente.go` | repo | A conversa com o Supabase |
| `web/package.json` · `vite.config.ts` · `tsconfig.json` | repo | A montagem do front |
| `web/index.html` | repo | Só o ponto de montagem |
| `web/src/main.tsx` · `App.tsx` | repo | Entrada e casca (sidebar + conteúdo) |
| `web/src/estilos/tokens.css` · `base.css` | repo | A identidade da Frota Macedo |
| `web/src/api/cliente.ts` | repo | A conversa com o baleryan |
| `web/src/sessao/*` | repo | Login, token e permissões |
| `web/src/menu/arvore.ts` | repo | Os 79 itens com as suas permissões |
| `web/src/telas/Login.tsx` · `Inicio.tsx` | repo | As duas telas do Passo 1 |

---

## 0.5 Decisões da Fase 0

| # | Decisão | Por quê |
|---|---|---|
| 1 | **Repositório** `frotahub-v2` | Repositório próprio, começando limpo. |
| 2 | **Banco** Supabase `hltcngamdqabqlocufrv` | Projeto dedicado. Consequência aceita: os logins da equipe são criados nele do zero. |
| 3 | Tabelas no schema **`public`** | É onde o Supabase espera encontrá-las. Um passo a menos de configuração. |
| 4 | **Motor em Go**, arquivo principal `baleryan.go` | Binário único, sobe em milissegundos, e o erro aparece na compilação em vez de em produção. |
| 5 | **Front em React + Vite + TypeScript** | São 79 telas repetindo os mesmos padrões — é o que componente resolve. E TypeScript com Go deixa os dois lados tipados: campo trocado quebra o build, não a tela. |
| 6 | **Visual próprio, sem biblioteca de componentes** | Sidebar escura, `#A11F22` como acento, Inter, ícones SVG. Biblioteca pronta traria visual próprio brigando com a identidade da casa. |
| 7 | **Arquivos vivos no Cloudflare R2**; arquivo-mestre no **Dropbox** | O R2 não cobra saída e serve o PDF direto ao navegador. O Dropbox dá lixeira e histórico nativos como rede de segurança. |
| 8 | **Margem arredondada linha a linha** | O unitário arredondado é o número impresso no papel; o total tem que ser a soma exata do que o cliente vê. *(aguardando seu aval)* |
| 9 | **Decimal, não float** | Meio centavo sobe, sempre. É como se arredonda dinheiro no Brasil. |
| 10 | Parâmetros de negócio **no banco** | A margem, o teto e o limite de extrapolado com histórico de quem mudou e quando. |

---

## 0.6 O que está pendente de você

| O quê | Para quê | Situação |
|---|---|---|
| **PAT do GitHub** (fine-grained, só o `frotahub-v2`, validade curta) | Para eu commitar sem você subir arquivo por arquivo | **bloqueando** |
| **O `.docx` do orçamento é necessário?** | Se o PDF basta, o sistema fica mais simples. Se você edita à mão às vezes, eu construo. | aguardando |
| **Aval do arredondamento linha a linha** | Muda centavo no orçamento novo. | aguardando |
| Credenciais do Cloudflare R2 | Quando chegarmos nos arquivos | mais pra frente |

---

## 0.7 Registro das rodadas

**Rodada 10** — Separado no diário o que **existe** do que é **desenho**: as duas árvores estão
marcadas como planejadas, e o que está criado de fato (repositório vazio e projeto Supabase vazio)
ganhou seção própria, com os endereços.

**Rodada 9** — Diário reescrito para tratar só do sistema novo, com o inventário de arquivos e
revisões. Fechado o Passo 1: a casca (login e tela inicial) e o motor base em Go.

**Rodada 8** — Definido que cada arquivo tem endereço e revisão declarados.

**Rodada 7** — Front fechado em React + Vite + TypeScript, com o visual próprio da casa. Diário
passa a ser dividido por fase.

**Rodada 6** — Diário criado como registro corrido do projeto.

**Rodada 5** — Motor definido em Go, com o arquivo principal `baleryan.go`. Gerador de orçamento
em PDF **testado de verdade** em Go: logo, tabela, total, extenso e assinatura, com a identidade
da casa. Duas anotações do teste: é preciso embarcar uma fonte completa no binário (o `²` de
"2,5MM²" some com a fonte embutida da biblioteca), e o `.docx` não tem biblioteca boa em Go — daí
a pergunta se ele é necessário.

**Rodadas 1 a 4** — Levantamento, desenho da arquitetura, amostra de estilo de código e criação do
repositório e do banco.

---

## Endereços e contas

| O quê | Onde | Existe? |
|---|---|---|
| Repositório | `github.com/iafrotamacedo-cloud/frotahub-v2` | sim, vazio |
| Banco | `https://hltcngamdqabqlocufrv.supabase.co` | sim, vazio |
| Motor | Render | não — a criar |
| Front | Netlify | não — a criar |
| Arquivos vivos | Cloudflare R2 | não — a criar |
| Arquivo-mestre | Dropbox | conta existente |

A chave pública do Supabase está registrada acima porque ela vai no navegador de qualquer forma.
A `service_role` e a senha do banco **não** ficam neste arquivo, por escolha.

---

*As fases seguintes entram abaixo, uma seção cada, conforme forem abertas.*
