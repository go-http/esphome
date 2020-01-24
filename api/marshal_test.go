package api

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"testing"
)

func TestReadMessage(t *testing.T) {
	var (
		bb = bytes.NewBuffer(testHelloResponseBytes)
		br = bufio.NewReader(bb)
	)
	m, err := ReadMessage(br)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%T: %+v", m, m)
}

var (
	testHelloResponseBytes, _ = hex.DecodeString("001f02080110031a1963616d657261302028657370686f6d652076312e31342e3329")
)
