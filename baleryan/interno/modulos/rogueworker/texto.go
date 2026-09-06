package rogueworker

import (
	"fmt"
	"strings"
)

func emReais(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.Replace(s, ".", ",", 1)
	return s
}
