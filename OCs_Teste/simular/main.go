// Simula ExtrairDoTexto + MotivosDeRejeicao para conferência contra _gabarito.json.
// Uso: go run . <arquivo.txt>
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/iafrotamacedo-cloud/frotahub-v2/baleryan/interno/modulos/administrativo"
)

type saida struct {
	Erro            string   `json:"erro,omitempty"`
	Numero          string   `json:"numero"`
	FornecedorNome  string   `json:"fornecedor_nome"`
	FornecedorCNPJ  string   `json:"fornecedor_cnpj"`
	CompradorCNPJ   string   `json:"comprador_cnpj"`
	NItens          int      `json:"n_itens"`
	Subtotal        float64  `json:"subtotal"`
	Desconto        float64  `json:"desconto"`
	Total           float64  `json:"total"`
	Status          string   `json:"status"`
	Motivos         []string `json:"motivos"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "uso: go run . <arquivo.txt>")
		os.Exit(2)
	}
	bruto, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ex, err := administrativo.ExtrairDoTexto(string(bruto))
	if err != nil {
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(saida{Erro: err.Error(), Status: "ERRO_LEITURA"})
		return
	}
	motivos := ex.MotivosDeRejeicao()
	status := "PROCESSADA"
	if len(motivos) > 0 {
		status = "REJEITADA"
	}
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(saida{
		Numero:         ex.Numero,
		FornecedorNome: ex.FornecedorNome,
		FornecedorCNPJ: ex.FornecedorCNPJ,
		CompradorCNPJ:  ex.CompradorCNPJ,
		NItens:         len(ex.Itens),
		Subtotal:       ex.Subtotal.Float(),
		Desconto:       ex.Desconto.Float(),
		Total:          ex.Total.Float(),
		Status:         status,
		Motivos:        motivos,
	})
}
