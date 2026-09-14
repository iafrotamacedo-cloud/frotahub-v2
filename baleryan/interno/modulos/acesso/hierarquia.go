// rev 1 — vínculo hierárquico: quem responde pra quem (068_vinculo_hierarquico.sql)
//
// QUEM PODE: builder ou CEO (quemPodeGerenciarAcesso, mesma trava de
// categoria.go — ver o comentário lá sobre por que é uma trava de NÍVEL, não
// de rotina).
//
// É ÁRVORE, NÃO GRAFO
//
//	Cada perfil tem NO MÁXIMO um superior direto (`unique perfil_id` na
//	tabela). "Quem é o chefe de fulano" tem sempre UMA resposta.
//
// O DEFAULT NUNCA É GRAVADO
//
//	Antes de qualquer configuração, todo perfil responde implicitamente ao
//	único CEO do cliente — "CEO > login". Isso é CALCULADO em cadeiaDe, a
//	cada leitura, nunca uma linha na tabela. Gravar o default a cada perfil
//	criaria trabalho pra desfazer no dia (raro, mas possível) em que o CEO
//	mudar.
//
// A REGRA DE INSERÇÃO — A PARTE QUE MAIS PRECISOU DE UMA DECISÃO
//
//	"Insira X entre P e Q" só é aceito se X já responder a P ESPECIFICAMENTE
//	— não "ter algum superior em algum lugar". Sem essa trava, X apareceria
//	nesta tela reportando a P, mas a cadeia de verdade de X (vista a partir
//	do login dele) discordaria — duas fontes de verdade brigando é
//	exatamente o que CORE-06 existe para evitar.
package acesso

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/banco"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/historico"
	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/web"
)

// tetoDaCadeia é o cinto de segurança contra dado corrompido — a cadeia de
// verdade tem no máximo 4 nós (ceo→gerencial→supervisorio→operacional), 6
// sobra bastante sem abrir espaço pra um laço se perder em produção.
const tetoDaCadeia = 6

var errPerfilNaoEncontrado = fmt.Errorf("Perfil não encontrado.")

type noCadeia struct {
	ID            string `json:"id"`
	Nome          string `json:"nome"`
	Usuario       string `json:"usuario"`
	Nivel         string `json:"nivel"`
	CategoriaNome string `json:"categoria_nome"`
	// Verdadeiro só no nó de topo quando ele veio do CEO default — a tela
	// usa isto pra saber que aquele trecho da cadeia ainda não foi gravado.
	Implicito bool `json:"implicito,omitempty"`
}

// ---------------------------------------------------------------------------
// GET /perfis — listagem enxuta pro popup de "quem entra aqui"
//
// Não é a tela de Usuários (GET /usuarios continua só-builder) — de
// propósito, pra não alargar o gate de um módulo que o próprio cabeçalho de
// usuarios.go trata como fundação builder-only.
// ---------------------------------------------------------------------------

type perfilLeveListagem struct {
	ID            string `json:"id"`
	Nome          string `json:"nome"`
	Usuario       string `json:"usuario"`
	Nivel         string `json:"nivel"`
	CategoriaNome string `json:"categoria_nome"`
	Ativo         bool   `json:"ativo"`
}

func (m *Modulo) listarPerfis(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeGerenciarAcesso(w, r)
	if p == nil {
		return
	}
	var linhas []struct {
		ID         string `json:"id"`
		Nome       string `json:"nome"`
		Usuario    string `json:"usuario"`
		Ativo      bool   `json:"ativo"`
		Categorias *struct {
			Nome  string `json:"nome"`
			Nivel string `json:"nivel"`
		} `json:"categorias"`
	}
	caminho := "perfis?cliente_id=eq." + banco.Escapar(p.ClienteID) +
		"&ativo=is.true&select=id,nome,usuario,ativo,categorias(nome,nivel)&order=nome"
	if err := m.bd.Buscar(r.Context(), caminho, &linhas); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui listar os perfis.")
		return
	}
	saida := make([]perfilLeveListagem, 0, len(linhas))
	for _, l := range linhas {
		item := perfilLeveListagem{ID: l.ID, Nome: l.Nome, Usuario: l.Usuario, Ativo: l.Ativo}
		if l.Categorias != nil {
			item.CategoriaNome = l.Categorias.Nome
			item.Nivel = l.Categorias.Nivel
		}
		saida = append(saida, item)
	}
	web.Responder(w, http.StatusOK, map[string]any{"perfis": saida})
}

// ---------------------------------------------------------------------------
// as peças que leem a cadeia
// ---------------------------------------------------------------------------

func (m *Modulo) perfilLeve(ctx context.Context, clienteID, perfilID string) (*noCadeia, error) {
	var linhas []struct {
		ID         string `json:"id"`
		Nome       string `json:"nome"`
		Usuario    string `json:"usuario"`
		Categorias *struct {
			Nome  string `json:"nome"`
			Nivel string `json:"nivel"`
		} `json:"categorias"`
	}
	caminho := "perfis?id=eq." + banco.Escapar(perfilID) +
		"&cliente_id=eq." + banco.Escapar(clienteID) +
		"&select=id,nome,usuario,categorias(nome,nivel)&limit=1"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return nil, err
	}
	if len(linhas) == 0 {
		return nil, nil
	}
	l := linhas[0]
	no := &noCadeia{ID: l.ID, Nome: l.Nome, Usuario: l.Usuario}
	if l.Categorias != nil {
		no.CategoriaNome = l.Categorias.Nome
		no.Nivel = l.Categorias.Nivel
	}
	return no, nil
}

// superiorDireto lê a linha própria do perfil em vinculos_hierarquicos —
// `temVinculo` falso é o "ainda não configurado", não um erro.
func (m *Modulo) superiorDireto(ctx context.Context, perfilID string) (superiorID string, temVinculo bool, err error) {
	var linhas []struct {
		SuperiorID string `json:"superior_id"`
	}
	caminho := "vinculos_hierarquicos?perfil_id=eq." + banco.Escapar(perfilID) + "&select=superior_id&limit=1"
	if err := m.bd.Buscar(ctx, caminho, &linhas); err != nil {
		return "", false, err
	}
	if len(linhas) == 0 {
		return "", false, nil
	}
	return linhas[0].SuperiorID, true, nil
}

// ceoDoCliente é o topo IMPLÍCITO — o único login de nível ceo do cliente.
// Dois passos (acha a categoria, depois o perfil) em vez de um embed do
// PostgREST: mais simples de acertar do que o `!inner` de filtro em
// recurso embutido, e este caminho não é chamado com frequência que
// justifique economizar uma ida ao banco.
func (m *Modulo) ceoDoCliente(ctx context.Context, clienteID string) (*noCadeia, error) {
	var cats []struct {
		ID   string `json:"id"`
		Nome string `json:"nome"`
	}
	if err := m.bd.Buscar(ctx, "categorias?cliente_id=eq."+banco.Escapar(clienteID)+
		"&nivel=eq.ceo&ativo=is.true&select=id,nome&limit=1", &cats); err != nil {
		return nil, err
	}
	if len(cats) == 0 {
		return nil, nil
	}
	var linhas []struct {
		ID      string `json:"id"`
		Nome    string `json:"nome"`
		Usuario string `json:"usuario"`
	}
	if err := m.bd.Buscar(ctx, "perfis?categoria_id=eq."+banco.Escapar(cats[0].ID)+
		"&cliente_id=eq."+banco.Escapar(clienteID)+"&ativo=is.true&select=id,nome,usuario&limit=1", &linhas); err != nil {
		return nil, err
	}
	if len(linhas) == 0 {
		return nil, nil
	}
	l := linhas[0]
	return &noCadeia{ID: l.ID, Nome: l.Nome, Usuario: l.Usuario, Nivel: "ceo", CategoriaNome: cats[0].Nome}, nil
}

// cadeiaDe sobe de vínculo em vínculo até achar um ceo/builder, ou esgotar o
// que está gravado — nesse caso o degrau final é o CEO do cliente,
// implícito.
func (m *Modulo) cadeiaDe(ctx context.Context, clienteID, perfilID string) ([]noCadeia, error) {
	alvo, err := m.perfilLeve(ctx, clienteID, perfilID)
	if err != nil {
		return nil, err
	}
	if alvo == nil {
		return nil, errPerfilNaoEncontrado
	}

	cadeia := []noCadeia{*alvo}
	visto := map[string]bool{perfilID: true}

	for i := 0; i < tetoDaCadeia; i++ {
		topo := cadeia[0]
		if topo.Nivel == "ceo" || topo.Nivel == "builder" {
			break
		}
		superiorID, temVinculo, err := m.superiorDireto(ctx, topo.ID)
		if err != nil {
			return nil, err
		}
		if !temVinculo {
			ceo, err := m.ceoDoCliente(ctx, clienteID)
			if err != nil {
				return nil, err
			}
			if ceo != nil {
				ceoCom := *ceo
				ceoCom.Implicito = true
				cadeia = append([]noCadeia{ceoCom}, cadeia...)
			}
			break
		}
		if visto[superiorID] {
			break
		}
		visto[superiorID] = true
		superior, err := m.perfilLeve(ctx, clienteID, superiorID)
		if err != nil {
			return nil, err
		}
		if superior == nil {
			break
		}
		cadeia = append([]noCadeia{*superior}, cadeia...)
	}
	return cadeia, nil
}

// caminhoChegaEm sobe a partir de `inicioID` e diz se o caminho passa por
// `alvoID` — é a guarda contra ciclo: inserir X (inicioID) sob P só é seguro
// se X não for, ele mesmo, descendente de Q (alvoID).
func (m *Modulo) caminhoChegaEm(ctx context.Context, inicioID, alvoID string) (bool, error) {
	atualID := inicioID
	visto := map[string]bool{}
	for i := 0; i < tetoDaCadeia; i++ {
		if atualID == alvoID {
			return true, nil
		}
		if visto[atualID] {
			return false, nil
		}
		visto[atualID] = true
		superiorID, temVinculo, err := m.superiorDireto(ctx, atualID)
		if err != nil {
			return false, err
		}
		if !temVinculo {
			return false, nil
		}
		atualID = superiorID
	}
	return false, nil
}

// nivelEncaixaAbaixoDe diz se `nivel` pode ocupar o degrau logo abaixo de
// `acima` — gerencial só entra sob ceo, supervisorio só sob gerencial;
// operacional entra sob qualquer um dos três (a exceção que o dono pediu
// pro Operacional: vínculo pode vir de qualquer nível acima, não só do
// vizinho direto).
func nivelEncaixaAbaixoDe(nivel, acima string) bool {
	switch nivel {
	case "gerencial":
		return acima == "ceo"
	case "supervisorio":
		return acima == "gerencial"
	case "operacional":
		return acima == "ceo" || acima == "gerencial" || acima == "supervisorio"
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// GET /perfis/{id}/hierarquia
// ---------------------------------------------------------------------------

func (m *Modulo) verHierarquia(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeGerenciarAcesso(w, r)
	if p == nil {
		return
	}
	alvoID, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	cadeia, err := m.cadeiaDe(r.Context(), p.ClienteID, alvoID)
	if err != nil {
		if err == errPerfilNaoEncontrado {
			web.Falhar(w, http.StatusNotFound, err.Error())
		} else {
			web.Falhar(w, http.StatusInternalServerError, "Não consegui montar a cadeia.")
		}
		return
	}
	web.Responder(w, http.StatusOK, map[string]any{"cadeia": cadeia})
}

// ---------------------------------------------------------------------------
// PUT /perfis/{id}/hierarquia — inserir um superior intermediário
// ---------------------------------------------------------------------------

type pedidoInserirVinculo struct {
	AcimaID        string `json:"acima_id"`
	NovoSuperiorID string `json:"novo_superior_id"`
}

func (m *Modulo) inserirNaHierarquia(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeGerenciarAcesso(w, r)
	if p == nil {
		return
	}
	qID, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}

	var pedido pedidoInserirVinculo
	if err := json.NewDecoder(r.Body).Decode(&pedido); err != nil {
		web.Falhar(w, http.StatusBadRequest, "Não entendi os dados enviados.")
		return
	}
	pAcimaID, ok1 := umUUID(pedido.AcimaID)
	xID, ok2 := umUUID(pedido.NovoSuperiorID)
	if !ok1 || !ok2 {
		web.Falhar(w, http.StatusBadRequest, "Escolha quem entra e em qual linha.")
		return
	}
	if qID == xID {
		web.Falhar(w, http.StatusBadRequest, "Uma pessoa não pode entrar acima de si mesma.")
		return
	}

	// A linha clicada na tela ainda precisa ser a de verdade — sem isto,
	// duas pessoas mexendo ao mesmo tempo poderiam inserir no lugar errado
	// sem avisar ninguém (P-29: a checagem de verdade é sempre no motor).
	superiorAtualDeQ, temVinculoQ, err := m.superiorDireto(r.Context(), qID)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir a cadeia atual.")
		return
	}
	acimaDeVerdade := superiorAtualDeQ
	if !temVinculoQ {
		ceo, err := m.ceoDoCliente(r.Context(), p.ClienteID)
		if err != nil {
			web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir o topo da cadeia.")
			return
		}
		if ceo != nil {
			acimaDeVerdade = ceo.ID
		}
	}
	if acimaDeVerdade != pAcimaID {
		web.Falhar(w, http.StatusConflict, "A cadeia mudou desde que esta tela abriu. Recarregue e tente de novo.")
		return
	}

	// X precisa já se reportar a P especificamente — não "ter algum
	// superior em algum lugar" (ver o cabeçalho do arquivo).
	superiorDeX, temVinculoX, err := m.superiorDireto(r.Context(), xID)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir o vínculo de quem está entrando.")
		return
	}
	if !temVinculoX || superiorDeX != pAcimaID {
		web.Falhar(w, http.StatusBadRequest,
			"Esta pessoa precisa já se reportar a quem está logo acima antes de poder ser inserida nesta cadeia.")
		return
	}

	xNivel, err := m.nivelDoPerfil(r.Context(), p.ClienteID, xID)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir o nível de quem está entrando.")
		return
	}
	acimaNivel, err := m.nivelDoPerfil(r.Context(), p.ClienteID, pAcimaID)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir o nível de quem está acima.")
		return
	}
	if !nivelEncaixaAbaixoDe(xNivel, acimaNivel) {
		web.Falhar(w, http.StatusBadRequest, "Este nível não entra logo abaixo de "+acimaNivel+".")
		return
	}

	ciclo, err := m.caminhoChegaEm(r.Context(), xID, qID)
	if err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui conferir a cadeia.")
		return
	}
	if ciclo {
		web.Falhar(w, http.StatusBadRequest, "Isso criaria um ciclo na hierarquia.")
		return
	}

	if err := m.bd.Upsert(r.Context(), "vinculos_hierarquicos?on_conflict=perfil_id", []map[string]any{{
		"cliente_id":   p.ClienteID,
		"perfil_id":    qID,
		"superior_id":  xID,
		"definido_por": p.UserID,
	}}, nil); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui salvar o vínculo.")
		return
	}

	_ = m.hist.Registrar(semCancelar(r), p, moduloHistorico, qID, "inseriu_na_hierarquia", map[string]historico.Mudanca{
		"superior_id": {De: pAcimaID, Para: xID},
	})
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

func (m *Modulo) nivelDoPerfil(ctx context.Context, clienteID, perfilID string) (string, error) {
	no, err := m.perfilLeve(ctx, clienteID, perfilID)
	if err != nil {
		return "", err
	}
	if no == nil {
		return "", errPerfilNaoEncontrado
	}
	return no.Nivel, nil
}

// ---------------------------------------------------------------------------
// DELETE /perfis/{id}/hierarquia — remove o vínculo PRÓPRIO deste perfil
//
// Ele volta a herdar o CEO por padrão. Quem responde A ELE (se ele for um
// degrau intermediário) não muda — só o vínculo dele com quem está acima
// some.
// ---------------------------------------------------------------------------

func (m *Modulo) removerDaHierarquia(w http.ResponseWriter, r *http.Request) {
	p := m.quemPodeGerenciarAcesso(w, r)
	if p == nil {
		return
	}
	id, ok := umUUID(r.PathValue("id"))
	if !ok {
		web.Falhar(w, http.StatusBadRequest, "Endereço inválido.")
		return
	}
	filtro := "perfil_id=eq." + id + "&cliente_id=eq." + banco.Escapar(p.ClienteID)
	if err := m.bd.Apagar(r.Context(), "vinculos_hierarquicos", filtro); err != nil {
		web.Falhar(w, http.StatusInternalServerError, "Não consegui remover o vínculo.")
		return
	}
	_ = m.hist.Registrar(semCancelar(r), p, moduloHistorico, id, "removeu_da_hierarquia", nil)
	web.Responder(w, http.StatusOK, map[string]any{"ok": true})
}

// umUUID recusa qualquer coisa que não tenha cara de uuid ANTES de virar
// filtro — mesma função de administrativo/modulo.go e orcamentos/rotas.go;
// cada pacote guarda a sua de propósito, é curta demais pra valer um pacote
// próprio só por causa dela.
func umUUID(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return "", false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return "", false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return "", false
		}
	}
	return s, true
}
