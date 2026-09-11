// Redesenha uma OC de teste no layout da 019731 (Obra Prima).
// Uso: go run . <layout.txt> <caso.json> <saida.pdf>
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/administrativo"
)

type caso struct {
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

type loja struct {
	titulo, obra, fatNome, fatEnd, entrega string
}

var lojas = map[string]loja{
	"VILLAS": {
		titulo:  "MERCADINHOS SÃO LUIZ VILLAS - MSL VILLAS - AQUIRAZ",
		obra:    "MSL VILLAS - AQUIRAZ",
		fatNome: "MERCADINHOS SÃO LUIZ VILLAS",
		fatEnd:  "ROD. CE -040, 1980, KM 19 LOJA 11 - PARQUE ARCO IRIS, Aquiraz - CE, 61700-000",
		entrega: "ROD. CE 040 KM - 20, S/N, VILLAS JARDIM - JACUNDÁ, Aquiraz, CE - 61700-000",
	},
	"DUNAS": {
		titulo:  "MERCADINHOS SÃO LUIZ DUNAS - DUNAS - FORTALEZA",
		obra:    "DUNAS - FORTALEZA",
		fatNome: "MERCADINHOS SÃO LUIZ DUNAS",
		fatEnd:  "Av. da Abolição, 3200 - Meireles, Fortaleza - CE, 60165-082",
		entrega: "Av. da Abolição, 3200 - Meireles, Fortaleza - CE, 60165-082",
	},
	"CAMBEBA": {
		titulo:  "MERCADINHOS SÃO LUIZ CAMBEBA - CAMBEBA - FORTALEZA",
		obra:    "CAMBEBA - FORTALEZA",
		fatNome: "MERCADINHOS SÃO LUIZ CAMBEBA",
		fatEnd:  "Av. Washington Soares, 85 - Cambeba, Fortaleza - CE, 60822-125",
		entrega: "Av. Washington Soares, 85 - Cambeba, Fortaleza - CE, 60822-125",
	},
	"RUI B.": {
		titulo:  "MERCADINHOS SÃO LUIZ RUI BARBOSA - RUI B. - FORTALEZA",
		obra:    "RUI B. - FORTALEZA",
		fatNome: "MERCADINHOS SÃO LUIZ RUI BARBOSA",
		fatEnd:  "Rua Rui Barbosa, 1700 - Aldeota, Fortaleza - CE, 60115-221",
		entrega: "Rua Rui Barbosa, 1700 - Aldeota, Fortaleza - CE, 60115-221",
	},
}

type job struct {
	Texto   string `json:"texto"`
	Caso    string `json:"caso"`
	Destino string `json:"destino"`
	Arquivo string `json:"arquivo"`
}

func main() {
	switch len(os.Args) {
	case 2:
		if err := batch(os.Args[1]); err != nil {
			fatal(err)
		}
	case 4:
		if err := um(os.Args[1], os.Args[2], os.Args[3]); err != nil {
			fatal(err)
		}
	default:
		fmt.Fprintln(os.Stderr, "uso: go run . <pasta-lote>")
		fmt.Fprintln(os.Stderr, "     go run . <layout.txt> <caso.json> <saida.pdf>")
		os.Exit(2)
	}
}

func batch(pasta string) error {
	var jobs []job
	if err := json.Unmarshal(mustRead(filepath.Join(pasta, "jobs.json")), &jobs); err != nil {
		return err
	}
	falhou := 0
	for _, j := range jobs {
		txt := j.Texto
		if !filepath.IsAbs(txt) {
			txt = filepath.Join(pasta, txt)
		}
		js := j.Caso
		if !filepath.IsAbs(js) {
			js = filepath.Join(pasta, js)
		}
		if err := um(txt, js, j.Destino); err != nil {
			fmt.Fprintf(os.Stderr, "FALHOU %s: %v\n", j.Arquivo, err)
			falhou++
			continue
		}
		fmt.Printf("OK %s\n", j.Arquivo)
	}
	if falhou > 0 {
		return fmt.Errorf("%d OC(s) falharam", falhou)
	}
	return nil
}

func um(layout, casoPath, destino string) error {
	bruto, err := os.ReadFile(layout)
	if err != nil {
		return err
	}
	var c caso
	brutoCaso, err := os.ReadFile(casoPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(brutoCaso, &c); err != nil {
		return err
	}
	ex, err := administrativo.ExtrairDoTexto(string(bruto))
	if err != nil {
		return err
	}
	vestir(&ex, c)
	pdf, err := administrativo.DesenharOC(ex)
	if err != nil {
		return err
	}
	return os.WriteFile(destino, pdf, 0o644)
}

func vestir(ex *administrativo.Extraida, c caso) {
	lj, ok := lojas[c.LojaNome]
	if !ok {
		lj = loja{
			titulo:  strings.TrimSpace(c.LojaNome + " - " + c.LojaCidade),
			obra:    strings.TrimSpace(c.LojaNome + " - " + c.LojaCidade),
			fatNome: c.LojaNome,
			fatEnd:  c.LojaCidade,
			entrega: c.LojaCidade,
		}
	}
	ex.TituloObra = lj.titulo
	ex.ObraCentroCusto = lj.obra
	ex.CompradorNome = lj.fatNome
	ex.FaturamentoEndereco = lj.fatEnd
	ex.EnderecoEntrega = lj.entrega
	ex.EnderecoCobranca = lj.entrega
	if c.Numero != "" {
		ex.Numero = c.Numero
	}
	if iso := dataISO(c.DataStr); iso != "" {
		ex.Data = iso
	}
	if iso := dataISO(c.PrevisaoStr); iso != "" {
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
	ex.FornecedorCNPJ = soDigitosCNPJ(c.FornecedorCNPJ)
	ex.CompradorCNPJ = soDigitosCNPJ(c.CompradorCNPJ)
	if c.CompradorInterno != "" {
		ex.CompradorInterno = c.CompradorInterno
	}
	ex.Frete = 0
	for i := range ex.Itens {
		if strings.TrimSpace(ex.Itens[i].Unidade) == "" {
			ex.Itens[i].Unidade = "UN"
		}
	}
}

func dataISO(br string) string {
	p := strings.Split(strings.TrimSpace(br), "/")
	if len(p) != 3 || len(p[0]) != 2 || len(p[1]) != 2 || len(p[2]) != 4 {
		return ""
	}
	return p[2] + "-" + p[1] + "-" + p[0]
}

func soDigitosCNPJ(d string) string {
	var b strings.Builder
	for _, r := range d {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func mustRead(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		fatal(err)
	}
	return b
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
