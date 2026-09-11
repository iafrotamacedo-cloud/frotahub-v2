// rev 2 — PCO: enviar o pacote por e-mail (SMTP do HostGator)
//
// O PEDIDO DO DONO (11/09/2026)
//
//	Duas portas para a MESMA ação: um robô agendado (GitHub Actions, 20h
//	Fortaleza) e um botão manual — um por OC, e um para "enviar tudo que
//	está pendente". As duas batem na mesma rota, porque é a mesma regra:
//	nunca duas implementações do que é "enviar o PCO" (CORE-06).
//
// MESMO PADRÃO DO ROBÔ DO TRÍLOGO
//
//	`trilogo.quemPode`/`trilogo.cliente` já resolvem exatamente este problema
//	— robô entra pela chave compartilhada (`X-Robot-Key`), pessoa entra pela
//	rotina do catálogo. Reaproveitado aqui, não reinventado.
//
// A TRAVA É UM CAS NO BANCO, NÃO UM MUTEX NO MOTOR
//
//	Antes de montar e-mail nenhum, a rota TENTA marcar as OCs escolhidas como
//	enviadas (`pco_enviado_em=is.null` no filtro do UPDATE). Só quem
//	consegue a marca entra no e-mail. Isso é o que impede dois cliques (ou
//	um clique em cima do robô agendado) de mandar duas mensagens com a
//	mesma OC. Se o envio falhar DEPOIS da marca, ela volta para null — a OC
//	continua pendente, não fica presa num limbo "marcada mas não enviada".
//
// NUNCA ENVIA VAZIO
//
//	Pedido explícito do dono. Zero pendentes (ou zero sobrando depois do
//	CAS, porque outra chamada levou todas) é sucesso silencioso, não erro —
//	a rota responde `"enviado": false` e não toca no correio.
package administrativo

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/correio"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/permissao"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/regras"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/seguranca"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// fusoFortaleza é UTC−3, sem horário de verão (o Brasil não usa desde 2019).
// A data do e-mail e do nome do zip são as do comprador, não as do servidor.
var fusoFortaleza = time.FixedZone("Fortaleza", -3*3600)

// ---------------------------------------------------------------------------
// quem pode — mesmo desenho de trilogo.quemPode/cliente
// ---------------------------------------------------------------------------

func (m *Modulo) quemPodeEnviarPCO(w http.ResponseWriter, r *http.Request) *seguranca.Principal {
	p, err := m.seg.DaRequisicao(r)
	if err != nil {
		web.Falhar(w, seguranca.StatusDoErro(err), err.Error())
		return nil
	}
	if p.Tipo == seguranca.TipoRobo {
		return p
	}
	if err := m.perm.Exige(r.Context(), p, RotinaPCOEnviar); err != nil {
		web.Falhar(w, permissao.StatusDoErro(err), err.Error())
		return nil
	}
	if p.ClienteID == "" {
		web.Falhar(w, http.StatusForbidden, "Este login não está ligado a nenhum cliente.")
		return nil
	}
	return p
}

// clienteDoPrincipal descobre de quem é o trabalho — pessoa traz no próprio
// perfil; robô diz pelo endereço (?cliente=slug) ou, havendo um cliente só,
// nem precisa dizer. Mesma função de `trilogo.Modulo.cliente`.
func (m *Modulo) clienteDoPrincipal(r *http.Request, p *seguranca.Principal) (string, error) {
	if p.ClienteID != "" {
		return p.ClienteID, nil
	}
	slug := strings.TrimSpace(r.URL.Query().Get("cliente"))
	var linhas []map[string]any
	caminho := "clientes?select=id&limit=2"
	if slug != "" {
		caminho = "clientes?slug=eq." + banco.Escapar(slug) + "&select=id&limit=2"
	}
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		return "", fmt.Errorf("não consegui descobrir o cliente")
	}
	switch len(linhas) {
	case 0:
		return "", fmt.Errorf("cliente não encontrado")
	case 1:
		return fmt.Sprint(linhas[0]["id"]), nil
	default:
		return "", fmt.Errorf("existe mais de um cliente: diga qual em ?cliente=")
	}
}

// ---------------------------------------------------------------------------
// o que vai no e-mail
// ---------------------------------------------------------------------------

type fornecedorEmbutido struct {
	RazaoSocial string `json:"razao_social"`
	CNPJ        string `json:"cnpj"`
}

type arquivoEmbutido struct {
	ChaveR2 string `json:"chave_r2"`
}

type ordemParaEnvio struct {
	ID              string              `json:"id"`
	NomeArquivo     string              `json:"nome_arquivo"`
	Numero          *string             `json:"numero"`
	ObraCentroCusto *string             `json:"obra_centro_custo"`
	Total           *float64            `json:"total"`
	Fornecedores    *fornecedorEmbutido `json:"fornecedores"`
	Arquivos        *arquivoEmbutido    `json:"arquivos"`
}

func (o ordemParaEnvio) centro() string {
	if o.ObraCentroCusto == nil || strings.TrimSpace(*o.ObraCentroCusto) == "" {
		return "SEM CENTRO DE CUSTO"
	}
	return strings.TrimSpace(*o.ObraCentroCusto)
}

func (o ordemParaEnvio) numero() string {
	if o.Numero != nil && strings.TrimSpace(*o.Numero) != "" {
		return strings.TrimSpace(*o.Numero)
	}
	return strings.TrimSuffix(o.NomeArquivo, ".pdf")
}

func (o ordemParaEnvio) valor() regras.Dinheiro {
	if o.Total == nil {
		return 0
	}
	return regras.DinheiroDe(*o.Total)
}

func (o ordemParaEnvio) fornecedorNome() string {
	if o.Fornecedores == nil || o.Fornecedores.RazaoSocial == "" {
		return "Não informado"
	}
	return o.Fornecedores.RazaoSocial
}

func (o ordemParaEnvio) fornecedorCNPJ() string {
	if o.Fornecedores == nil || o.Fornecedores.CNPJ == "" {
		return "Não informado"
	}
	return formatarCNPJ(o.Fornecedores.CNPJ)
}

func (o ordemParaEnvio) chaveArquivo() string {
	if o.Arquivos == nil {
		return ""
	}
	return o.Arquivos.ChaveR2
}

// buscarPendentes traz as OCs pendentes de envio, com fornecedor e arquivo
// embutidos numa consulta só (PostgREST resolve os dois FKs de uma vez —
// mesmo padrão de embed que `funcionarios`/`orcamentos` já usam).
func (m *Modulo) buscarPendentes(ctx context.Context, clienteID, filtroExtra string) ([]ordemParaEnvio, error) {
	caminho := "ordens_compra?cliente_id=eq." + banco.Escapar(clienteID) +
		"&status=eq.lido&pco_enviado_em=is.null" + filtroExtra +
		"&order=obra_centro_custo,criado_em" +
		"&select=id,nome_arquivo,numero,obra_centro_custo,total," +
		"fornecedores(razao_social,cnpj),arquivos(chave_r2)"
	var linhas []ordemParaEnvio
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	return linhas, nil
}

// ---------------------------------------------------------------------------
// POST /administrativo/compras/pco/enviar — tudo que está pendente
// ---------------------------------------------------------------------------

func (m *Modulo) enviarPCO(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeEnviarPCO(w, r)
	if p == nil {
		return
	}
	clienteID, err := m.clienteDoPrincipal(r, p)
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}
	p.ClienteID = clienteID // para o histórico gravar do cliente certo (robô não traz isto no perfil)

	pendentes, err := m.buscarPendentes(r.Context(), clienteID, "")
	if err != nil {
		m.erro(w, "não consegui listar as OCs pendentes de envio", err)
		return
	}
	m.enviarLote(w, r, p, pendentes)
}

// ---------------------------------------------------------------------------
// POST /administrativo/compras/pco/ordens/{id}/enviar — uma só
// ---------------------------------------------------------------------------

func (m *Modulo) enviarUmaPCO(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeEnviarPCO(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	clienteID, err := m.clienteDoPrincipal(r, p)
	if err != nil {
		web.Falhar(w, http.StatusBadRequest, err.Error())
		return
	}
	p.ClienteID = clienteID

	// A OC pode existir, mas não estar em condição de ser enviada — a
	// mensagem tem que dizer qual dos dois casos é (P-18: erro escrito para
	// ser lido).
	atual, err := m.contarUm(r.Context(), "ordens_compra?id=eq."+id+
		"&cliente_id=eq."+banco.Escapar(clienteID)+"&select=status,pco_enviado_em&limit=1")
	if err != nil {
		m.erro(w, "não achei esta ordem de compra", err)
		return
	}
	if fmt.Sprint(atual["status"]) != "lido" {
		web.Falhar(w, http.StatusConflict, "Esta OC ainda não passou pela leitura — só uma OC processada pode ser enviada.")
		return
	}
	if atual["pco_enviado_em"] != nil {
		web.Falhar(w, http.StatusConflict, "Esta OC já foi enviada.")
		return
	}

	pendentes, err := m.buscarPendentes(r.Context(), clienteID, "&id=eq."+id)
	if err != nil {
		m.erro(w, "não consegui carregar esta OC", err)
		return
	}
	if len(pendentes) == 0 {
		// Alguém enviou (ou desapareceu) entre a conferência acima e agora —
		// corrida rara, mas o CAS de `enviarLote` cobriria de qualquer jeito.
		web.Falhar(w, http.StatusConflict, "Esta OC não está mais pendente de envio.")
		return
	}
	m.enviarLote(w, r, p, pendentes)
}

// ---------------------------------------------------------------------------
// o núcleo — compartilhado pelas duas rotas
// ---------------------------------------------------------------------------

func (m *Modulo) enviarLote(w http.ResponseWriter, r *http.Request, p *seguranca.Principal, pendentes []ordemParaEnvio) {
	if len(pendentes) == 0 {
		web.Responder(w, http.StatusOK, map[string]any{
			"enviado": false, "motivo": "nada pendente de envio",
		})
		return
	}

	ids := make([]string, 0, len(pendentes))
	for _, o := range pendentes {
		ids = append(ids, o.ID)
	}

	// O CAS: só quem ainda está `pco_enviado_em is null` entra na marca. Se
	// duas chamadas disputarem a mesma OC, só uma a leva.
	agora := time.Now().UTC()
	var confirmadas []struct {
		ID string `json:"id"`
	}
	filtroCAS := "id=in.(" + strings.Join(ids, ",") + ")&pco_enviado_em=is.null"
	if err := m.bd.AtualizarDevolvendo(r.Context(), "ordens_compra", filtroCAS,
		map[string]any{"pco_enviado_em": agora.Format(time.RFC3339)}, &confirmadas); err != nil {
		m.erro(w, "não consegui marcar as OCs como enviadas", err)
		return
	}
	if len(confirmadas) == 0 {
		// Outra chamada (o robô, ou outro clique) já levou todas entre a
		// busca e aqui. Não é erro: é a mesma garantia de "nunca vazio",
		// só que descoberta um passo mais tarde.
		web.Responder(w, http.StatusOK, map[string]any{
			"enviado": false, "motivo": "já foram enviadas por outra chamada",
		})
		return
	}
	confirmadasID := make(map[string]bool, len(confirmadas))
	for _, c := range confirmadas {
		confirmadasID[c.ID] = true
	}
	enviar := pendentes[:0:0]
	for _, o := range pendentes {
		if confirmadasID[o.ID] {
			enviar = append(enviar, o)
		}
	}

	if err := m.enviarPorEmail(r.Context(), p.ClienteID, enviar); err != nil {
		// O envio falhou (ou não está configurado): desfaz a marca — a OC
		// volta a ser "pendente de envio", não fica presa num limbo.
		_ = m.bd.Atualizar(r.Context(), "ordens_compra",
			"id=in.("+strings.Join(idsDe(enviar), ",")+")",
			map[string]any{"pco_enviado_em": nil})
		web.Falhar(w, http.StatusServiceUnavailable, "Não consegui enviar o e-mail: "+err.Error())
		return
	}

	valorTotal := regras.Dinheiro(0)
	for _, o := range enviar {
		valorTotal += o.valor()
		_ = m.hist.Registrar(r.Context(), p, "administrativo", o.ID, "enviar_pco", map[string]historico.Mudanca{
			"pco_enviado_em": {De: nil, Para: agora.Format(time.RFC3339)},
		})
	}

	web.Responder(w, http.StatusOK, map[string]any{
		"enviado":     true,
		"quantidade":  len(enviar),
		"valor_total": valorTotal.Float(),
	})
}

func idsDe(ordens []ordemParaEnvio) []string {
	ids := make([]string, 0, len(ordens))
	for _, o := range ordens {
		ids = append(ids, o.ID)
	}
	return ids
}

// enviarPorEmail monta o zip e o HTML, busca os destinatários ativos, e
// chama o correio. Função pura quanto ao BANCO de OCs (só lê o que recebeu) —
// a única escrita daqui para fora é buscar destinatários e baixar do
// armazém, nenhuma delas em `ordens_compra`.
func (m *Modulo) enviarPorEmail(ctx context.Context, clienteID string, ordens []ordemParaEnvio) error {
	destinatarios, err := m.destinatariosAtivos(ctx, clienteID)
	if err != nil {
		return err
	}
	if len(destinatarios) == 0 {
		return fmt.Errorf("nenhum destinatário cadastrado — cadastre pelo menos um em Configurações antes de enviar")
	}

	agora := time.Now().In(fusoFortaleza)
	data := agora.Format("02-01-2006")

	zipBytes, err := m.montarZip(ctx, ordens)
	if err != nil {
		return fmt.Errorf("não consegui montar o zip: %w", err)
	}

	html := montarHTMLDoEnvio(ordens, data)

	return m.correio.Enviar(ctx, correio.Mensagem{
		Para:    destinatarios,
		Assunto: fmt.Sprintf("Ordens de Compra (PCO) - %s - Frota Macedo Engenharia", data),
		HTML:    html,
		Anexos: []correio.Anexo{
			{Nome: fmt.Sprintf("Pedidos_PCO_%s.zip", data), Conteudo: zipBytes},
		},
	})
}

// montarZip baixa cada PDF do armazém e organiza em pastas por centro de
// custo dentro do zip — mesma estrutura que a skill `pco-organizer` usava
// no Outlook, para quem abre o anexo reconhecer o padrão.
func (m *Modulo) montarZip(ctx context.Context, ordens []ordemParaEnvio) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, o := range ordens {
		chave := o.chaveArquivo()
		if chave == "" {
			continue // sem arquivo guardado — não deveria acontecer numa OC lida, mas não trava o lote inteiro
		}
		conteudo, err := m.arm.Baixar(ctx, chave)
		if err != nil {
			return nil, fmt.Errorf("baixando %s: %w", o.numero(), err)
		}
		nomeNoZip := pastaSegura(o.centro()) + "/" + o.numero() + ".pdf"
		f, err := w.Create(nomeNoZip)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(conteudo); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pastaSegura troca os caracteres que o Windows recusa em nome de pasta —
// mesma lista que a skill `pco-organizer` já usava (`safe_folder`).
func pastaSegura(nome string) string {
	trocar := func(r rune) rune {
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return r
	}
	return strings.TrimSpace(strings.Map(trocar, nome))
}

// montarHTMLDoEnvio monta o corpo do e-mail — adaptação do "Modelo 2" da
// skill `pco-organizer` (a versão HTML com tabela de verdade; o envio manda
// HTML sempre, então a versão Markdown/texto de fallback para o Outlook não
// se aplica aqui).
func montarHTMLDoEnvio(ordens []ordemParaEnvio, data string) string {
	agrupadas := map[string][]ordemParaEnvio{}
	var centros []string
	for _, o := range ordens {
		c := o.centro()
		if _, existe := agrupadas[c]; !existe {
			centros = append(centros, c)
		}
		agrupadas[c] = append(agrupadas[c], o)
	}
	sort.Strings(centros)

	var linhas strings.Builder
	var total regras.Dinheiro
	for _, c := range centros {
		for _, o := range agrupadas[c] {
			total += o.valor()
			fmt.Fprintf(&linhas, `<tr>
        <td style="border:1px solid #ccc;">%s</td>
        <td style="border:1px solid #ccc;">%s</td>
        <td style="border:1px solid #ccc;">%s</td>
        <td style="border:1px solid #ccc;">%s</td>
        <td style="border:1px solid #ccc;text-align:right;">%s</td>
      </tr>
`, escaparHTML(c), escaparHTML(o.numero()), escaparHTML(o.fornecedorNome()),
				escaparHTML(o.fornecedorCNPJ()), o.valor().Reais())
		}
	}

	return fmt.Sprintf(`<div style="font-family:Arial,Helvetica,sans-serif;font-size:14px;color:#222;">
  <p>Prezados,</p>

  <p>Encaminhamos o pacote de Ordens de Compra (PCO) do dia
  <strong>%s</strong>, organizado em pastas por obra/centro de custo.
  Segue abaixo o resumo das ordens incluídas neste envio:</p>

  <table cellspacing="0" cellpadding="6"
         style="border-collapse:collapse;width:100%%;font-size:13px;">
    <thead>
      <tr style="background:#1f3864;color:#ffffff;text-align:left;">
        <th style="border:1px solid #cccccc;">Centro de custo</th>
        <th style="border:1px solid #cccccc;">OC</th>
        <th style="border:1px solid #cccccc;">Fornecedor</th>
        <th style="border:1px solid #cccccc;">CNPJ</th>
        <th style="border:1px solid #cccccc;text-align:right;">Valor (R$)</th>
      </tr>
    </thead>
    <tbody>
      %s
    </tbody>
    <tfoot>
      <tr style="background:#f2f2f2;font-weight:bold;">
        <td style="border:1px solid #cccccc;" colspan="4">Total (%d ordens)</td>
        <td style="border:1px solid #cccccc;text-align:right;">%s</td>
      </tr>
    </tfoot>
  </table>

  <p style="margin-top:12px;">Arquivo em anexo:
  <strong>Pedidos_PCO_%s.zip</strong></p>

  <p>Os PDFs completos de cada ordem estão nas respectivas pastas do anexo.
  Ficamos à disposição para qualquer esclarecimento.</p>

  <p>Atenciosamente,<br><br>
  <strong>Frota Macedo Engenharia</strong></p>
</div>`, data, linhas.String(), len(ordens), total.Reais(), data)
}

func escaparHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// formatarCNPJ escreve "XX.XXX.XXX/XXXX-XX" — mesma função da skill
// `pco-organizer` (`fmt_cnpj`), só que em Go.
func formatarCNPJ(digitos string) string {
	d := soDigitos(digitos)
	if len(d) != 14 {
		return digitos
	}
	return d[0:2] + "." + d[2:5] + "." + d[5:8] + "/" + d[8:12] + "-" + d[12:14]
}
