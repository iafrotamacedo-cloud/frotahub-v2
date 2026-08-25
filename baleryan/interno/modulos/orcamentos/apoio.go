// rev 1 — as peças pequenas que os outros arquivos usam
package orcamentos

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

func somaDe(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func leitorDeBytes(b []byte) io.ReadSeeker { return bytes.NewReader(b) }

func regrasDecimal(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

// fusoDaCasa é Fortaleza. A data que vai para o Trílogo é a data de HOJE aqui,
// não a do servidor em UTC — que às 21h já virou amanhã.
var fusoDaCasa = sync.OnceValue(func() *time.Location {
	if l, err := time.LoadLocation("America/Fortaleza"); err == nil {
		return l
	}
	return time.FixedZone("BRT", -3*60*60)
})
