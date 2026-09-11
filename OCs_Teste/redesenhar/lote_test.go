package main

import (
	"os"
	"testing"
)

func TestLote(t *testing.T) {
	pasta := os.Getenv("OC_LOTE")
	if pasta == "" {
		t.Skip("sem OC_LOTE")
	}
	if err := batch(pasta); err != nil {
		t.Fatal(err)
	}
}
