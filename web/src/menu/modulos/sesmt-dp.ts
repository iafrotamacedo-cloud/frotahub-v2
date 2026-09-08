// rev 1 — o bloco de menu do SESMT e DP
//
// Primeiro módulo de RH do FrotaHub: cadastro de funcionário da obra e o
// controle dos documentos que a lei exige (pessoais, admissionais, ASO,
// certificados de NR). Ver a migração 044 e `baleryan/interno/modulos/
// funcionarios` para o desenho completo.
//
// Fica no próprio arquivo, junto de `administrativo.ts`, pelo mesmo motivo:
// cada módulo cresce sem precisar editar `arvore.ts` além de uma linha de
// import e uma de lista.
import type { ItemMenu } from '../arvore'

export const sesmtDpMenu: ItemMenu = {
  t: 'SESMT e DP',
  rota: 'sesmt-dp',
  icone: 'pessoas',
  desc: 'Funcionários e a documentação de segurança do trabalho',
  sub: [
    {
      t: 'Funcionários',
      rota: 'funcionarios',
      icone: 'pessoa',
      desc: 'Cadastro, ASO, EPI e certificados de NR por funcionário',
      tela: 'funcionarios',
      rotina: 'CONTRATO_FUNCIONARIOS_DADOS',
    },
  ],
}
