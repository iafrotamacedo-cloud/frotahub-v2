# Referência — o sistema ANTIGO

> **Atenção: isto NÃO é o plano do FrotaHub novo.**
>
> Este arquivo guarda o que foi levantado do sistema que está em produção hoje. Serve
> para consulta quando quisermos copiar alguma coisa de lá — uma regra de negócio, o
> nome de um menu, um comportamento que já funciona.
>
> **Nada aqui vale como decisão.** O FrotaHub novo é construído do zero, e cada rotina
> só existe quando é criada. Enquanto algo deste arquivo não for explicitamente trazido
> para o projeto novo, ele é só história.
>
> Levantado em agosto de 2026.

---

## Onde o sistema antigo vive

| O quê | Onde | Observação |
|---|---|---|
| Motor | `motor-orcamentos.onrender.com` (Render, Ohio) | Python/FastAPI, um arquivo de 7.158 linhas, 168 rotas |
| Front | um `index.html` de 5.898 linhas | 475 funções, um `<script>` de 5.475 linhas |
| Banco | Supabase `faalgfbugvekbuhhtatt` (São Paulo) | 22 tabelas |
| Repositórios | `motor-orcamentos`, `trilogo_robo` | **não tocar** |
| Arquivos | Dropbox, pastas numeradas de 0 a 11 | a pasta representa o estado |

---

## O menu do sistema antigo

Seis menus, 79 rotinas. O código em `crase` é a permissão que controlava o acesso.

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

## Regras de negócio observadas no sistema antigo

> Foram extraídas lendo o código. Algumas são boas e provavelmente serão adotadas; outras
> existem por acidente histórico. **Nenhuma é regra do sistema novo até ser decidida.**

### Administrativo
> PCO, Notas Fiscais, Locações, Compras.

**A O.C. tem quatro marcos, e nenhum se pula.**
PCO solicitado → nota conferida → nota entregue → nota protocolada. Uma ordem de compra
pode gerar várias notas (recebimento parcial).

**Recebimento de nota física é marco restrito.**
Só gerência alcança. *Motivo: é o marco que confirma posse do documento fiscal.*

### Manutenção
> Chamados, Orçamentos, Rateio, Preventiva, Financeiro do contrato.

**O ticket é do cliente; o `id` é nosso.**
O sistema é governado pelo identificador interno, nunca pelo número do ticket — ticket muda
quando uma nota sem-ticket é corrigida.

**Nada vai para o cliente sem conferência de valor.**
Toda escrita no sistema do cliente passa pelas travas de status e de teto.

### Engenharia
*Nada levantado.*

### SESMT
*Nada levantado.*

### Auditoria
**Quem audita não opera.**
A auditoria é restrita a builder e CEO; gerência não audita. *Motivo: separação de funções —
quem aprova a compra não pode ser quem julga se ela era permitida.*

### Configuração
**Permissão é por rotina, não por menu.**
Dá para liberar "conferir nota" sem liberar "receber nota física".

---

### Orçamentos
**A duplicidade é da NOTA, não do ticket.**
Um ticket pode ter vários orçamentos de notas diferentes; a mesma nota nunca gera dois.

**O total é a soma do que está impresso.**
Arredonda-se o valor unitário e o total é a soma das linhas do papel. *Motivo: quem confere
é gente, com calculadora, olhando o documento.*

**Um documento é nota fiscal ou DAV, nunca os dois.**

### PCO
*Nada levantado.*

### Notas Fiscais
*Nada levantado.*

---

### Lançar orçamento no Trílogo
**Só lança em ticket Executado ou Vistoriado.**

**A soma dos custos do ticket mais o novo não passa do teto.**
Se passar, aborta e o orçamento vira extrapolado, à espera de aprovação.

**Suspeito não lança sem liberação do CEO.**

---

---

## Defeitos encontrados no sistema antigo

Achados durante o levantamento. Existem em produção hoje.

- **`_exige_builder` está definido duas vezes** no motor, e a segunda definição sobrescreve
  a primeira — mudando quem pode acessar as rotas de migração sem que ninguém percebesse.
- **As colunas `faturado` e `pago` nunca são escritas** — a planilha de controle reporta
  sempre falso nesses campos.
- **O `+20%` está implementado 11 vezes**, de duas formas que não dão o mesmo resultado.
- **A rotina de gerar orçamento existe 5 vezes** quase idêntica.
- **18 endpoints varrem pasta** em vez de consultar o banco.
- **10 dicionários de estado na memória do processo** — somem quando o servidor hiberna.
- **7 mecanismos de autenticação** diferentes.

Vários dos tenets Core do projeto novo nasceram justamente daqui.
