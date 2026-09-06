package rogueworker

// Pedido é o que o chat manda a cada turno.
//
// O `pendente` volta do turno anterior e o front devolve intacto. Assim a
// confirmação ("sim, gera essas 12") não depende de sessão no servidor —
// o Render pode ter mais de uma instância, e memória local mentiria.
type Pedido struct {
	Mensagem string    `json:"mensagem"`
	Pendente *Pendente `json:"pendente,omitempty"`
}

// Pendente é o que a Worker ficou esperando. Um turno de cada vez.
type Pendente struct {
	Tipo      string   `json:"tipo"` // acao, escopo, excel, aprendizado
	Comando   string   `json:"comando,omitempty"`
	IDs       []string `json:"ids,omitempty"`
	Rotulo    string   `json:"rotulo,omitempty"`
	Excel     string   `json:"excel,omitempty"`
	Candidato string   `json:"candidato_id,omitempty"`
	Escopo    string   `json:"escopo,omitempty"`
}

// Resposta é o que o chat desenha.
type Resposta struct {
	Texto       string       `json:"texto"`
	Pendente    *Pendente    `json:"pendente,omitempty"`
	Navegar     *DestinoNav  `json:"navegar,omitempty"`
	Excel       *OfertaExcel `json:"excel,omitempty"`
	Opcoes      []string     `json:"opcoes,omitempty"`
	CandidatoID string       `json:"candidato_id,omitempty"`
}

type DestinoNav struct {
	Rotas  []string `json:"rotas"`
	Tela   string   `json:"tela"`
	Rotina string   `json:"rotina"`
}

type OfertaExcel struct {
	Caminho string `json:"caminho"`
	Baixar  bool   `json:"baixar,omitempty"`
}

const (
	cmdPendencias        = "pendencias"
	cmdStatusNota        = "status_nota"
	cmdExtrapoladas      = "extrapoladas"
	cmdBloqueadas        = "bloqueadas"
	cmdGerar             = "gerar"
	cmdGerarLote         = "gerar_lote"
	cmdLancar            = "lancar"
	cmdLancarLote        = "lancar_lote"
	cmdPagamos           = "pagamos_fornecedores"
	cmdMaterial          = "material_lancado"
	cmdFaturado          = "faturado"
	cmdFechamento        = "fechamento"
	cmdChamadosAtendidos = "chamados_atendidos"
	cmdNavegar           = "navegar"
	cmdDesconhecido      = "desconhecido"
)

type reconhecimento struct {
	comando string
	ticket  int
	id      string
	tela    string
}
