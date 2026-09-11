package administrativo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoteOCsTeste redesigna as OCs de OCs_Teste. Só roda com OC_LOTE apontando
// para a pasta de jobs que o _redesenhar.py monta — o Windows desta máquina
// bloqueia o `go run`/`go test` do módulo auxiliar.
func TestLoteOCsTeste(t *testing.T) {
	pasta := os.Getenv("OC_LOTE")
	if pasta == "" {
		t.Skip("sem OC_LOTE")
	}
	bruto, err := os.ReadFile(filepath.Join(pasta, "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var jobs []struct {
		Texto, Caso, Destino, Arquivo string
	}
	if err := json.Unmarshal(bruto, &jobs); err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		txt := j.Texto
		if !filepath.IsAbs(txt) {
			txt = filepath.Join(pasta, txt)
		}
		js := j.Caso
		if !filepath.IsAbs(js) {
			js = filepath.Join(pasta, js)
		}
		if err := redesenharUm(txt, js, j.Destino); err != nil {
			t.Errorf("%s: %v", j.Arquivo, err)
			continue
		}
		fmt.Printf("OK %s\n", j.Arquivo)
	}
}

type casoOC struct {
	Numero           string `json:"numero"`
	LojaNome         string `json:"loja_nome"`
	LojaCidade       string `json:"loja_cidade"`
	DataStr          string `json:"data_str"`
	PrevisaoStr      string `json:"previsao_str"`
	CondPgto         string `json:"cond_pgto"`
	FormaPgto        string `json:"forma_pgto"`
	CompradorInterno string `json:"comprador_interno"`
	FornecedorNome   string `json:"fornecedor_nome"`
	FornecedorCNPJ   string `json:"fornecedor_cnpj"`
	CompradorCNPJ    string `json:"comprador_cnpj"`
}

func redesenharUm(layout, casoPath, destino string) error {
	bruto, err := os.ReadFile(layout)
	if err != nil {
		return err
	}
	var c casoOC
	b, err := os.ReadFile(casoPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return err
	}
	ex, err := ExtrairDoTexto(string(bruto))
	if err != nil {
		return err
	}
	vestirOCTeste(&ex, c)
	pdf, err := DesenharOC(ex)
	if err != nil {
		return err
	}
	return os.WriteFile(destino, pdf, 0o644)
}

func vestirOCTeste(ex *Extraida, c casoOC) {
	type lj struct{ titulo, obra, fatNome, fatEnd, entrega string }
	lojas := map[string]lj{
		"VILLAS": {
			"MERCADINHOS SÃO LUIZ VILLAS - MSL VILLAS - AQUIRAZ",
			"MSL VILLAS - AQUIRAZ",
			"MERCADINHOS SÃO LUIZ VILLAS",
			"ROD. CE -040, 1980, KM 19 LOJA 11 - PARQUE ARCO IRIS, Aquiraz - CE, 61700-000",
			"ROD. CE 040 KM - 20, S/N, VILLAS JARDIM - JACUNDÁ, Aquiraz, CE - 61700-000",
		},
		"DUNAS": {
			"MERCADINHOS SÃO LUIZ DUNAS - DUNAS - FORTALEZA",
			"DUNAS - FORTALEZA",
			"MERCADINHOS SÃO LUIZ DUNAS",
			"Av. da Abolição, 3200 - Meireles, Fortaleza - CE, 60165-082",
			"Av. da Abolição, 3200 - Meireles, Fortaleza - CE, 60165-082",
		},
		"CAMBEBA": {
			"MERCADINHOS SÃO LUIZ CAMBEBA - CAMBEBA - FORTALEZA",
			"CAMBEBA - FORTALEZA",
			"MERCADINHOS SÃO LUIZ CAMBEBA",
			"Av. Washington Soares, 85 - Cambeba, Fortaleza - CE, 60822-125",
			"Av. Washington Soares, 85 - Cambeba, Fortaleza - CE, 60822-125",
		},
		"RUI B.": {
			"MERCADINHOS SÃO LUIZ RUI BARBOSA - RUI B. - FORTALEZA",
			"RUI B. - FORTALEZA",
			"MERCADINHOS SÃO LUIZ RUI BARBOSA",
			"Rua Rui Barbosa, 1700 - Aldeota, Fortaleza - CE, 60115-221",
			"Rua Rui Barbosa, 1700 - Aldeota, Fortaleza - CE, 60115-221",
		},
	}
	j, ok := lojas[c.LojaNome]
	if !ok {
		j = lj{
			titulo:  strings.TrimSpace(c.LojaNome + " - " + c.LojaCidade),
			obra:    strings.TrimSpace(c.LojaNome + " - " + c.LojaCidade),
			fatNome: c.LojaNome,
			fatEnd:  c.LojaCidade,
			entrega: c.LojaCidade,
		}
	}
	ex.TituloObra = j.titulo
	ex.ObraCentroCusto = j.obra
	ex.CompradorNome = j.fatNome
	ex.FaturamentoEndereco = j.fatEnd
	ex.EnderecoEntrega = j.entrega
	ex.EnderecoCobranca = j.entrega
	if c.Numero != "" {
		ex.Numero = c.Numero
	}
	if iso := dataISOTeste(c.DataStr); iso != "" {
		ex.Data = iso
	}
	if iso := dataISOTeste(c.PrevisaoStr); iso != "" {
		ex.PrevisaoEntrega = iso
	}
	if c.CondPgto != "" {
		ex.CondPgto = c.CondPgto
	}
	if c.FormaPgto != "" {
		ex.FormaPgto = c.FormaPgto
	}
	ex.DataImpressao = "10/09/2026"
	ex.EmitenteRazao = "FROTA MACEDO ENGENHARIA LTDA"
	ex.EmitenteEndereco = "Engenheiro Heitor de Oliveira Albuquerque, 295 - Cidade dos Funcionários - Fortaleza/CE"
	ex.EmitenteContato = "85 2181 - 1386 - frotamacedoengenharia@gmail.com"
	ex.EmitenteCNPJ = "27.363.223/0001-70"
	ex.ResponsavelNome = "FROTA MACEDO ENGENHARIA LTDA"
	ex.ResponsavelEmail = "compras@frotamacedo.com.br"
	ex.FornecedorVendedor = "Contato principal"
	ex.Observacao = ""
	ex.FornecedorEndereco = "Avenida Bezerra de Menezes, 420 - Farias Brito, Fortaleza - CE, 60325-000"
	if strings.TrimSpace(ex.FornecedorEmail) == "" {
		ex.FornecedorEmail = "contato@fornecedor.com.br"
	}
	if c.FornecedorNome != "" {
		ex.FornecedorNome = c.FornecedorNome
	}
	ex.FornecedorCNPJ = soDigitos(c.FornecedorCNPJ)
	ex.CompradorCNPJ = soDigitos(c.CompradorCNPJ)
	if c.CompradorInterno != "" {
		ex.CompradorInterno = c.CompradorInterno
	}
	ex.Frete = 0
	for i := range ex.Itens {
		ex.Itens[i].Descricao = limparLixoDeRodape(ex.Itens[i].Descricao)
		if strings.TrimSpace(ex.Itens[i].Unidade) == "" {
			ex.Itens[i].Unidade = "UN"
		}
	}
}

func dataISOTeste(br string) string {
	p := strings.Split(strings.TrimSpace(br), "/")
	if len(p) != 3 || len(p[0]) != 2 || len(p[1]) != 2 || len(p[2]) != 4 {
		return ""
	}
	return p[2] + "-" + p[1] + "-" + p[0]
}
