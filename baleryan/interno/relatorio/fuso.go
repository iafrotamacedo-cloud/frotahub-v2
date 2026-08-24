package relatorio

import (
	"sync"
	"time"
)

// FusoDaCasa é Fortaleza. Relatório é lido por gente que está lá, e um horário
// em UTC num documento impresso não tem como ser corrigido depois.
var FusoDaCasa = sync.OnceValue(func() *time.Location {
	l, err := time.LoadLocation("America/Fortaleza")
	if err != nil {
		return time.FixedZone("BRT", -3*3600)
	}
	return l
})
