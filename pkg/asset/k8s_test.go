package asset

import (
	"testing"
)

func TestEncodeBytes(t *testing.T) {
	token := make([]byte, 22)
	random := []byte{ // 1324076502674089023697701463353265679
		255,
		2,
		3,
		4,
		5,
		6,
		7,
		8,
		9,
		10,
		11,
		12,
		13,
		14,
		15,
	}
	encodeBytes(validBootstrapTokenChars, random, token)
	expected := "n2r8j0ug4twvqmqt53hyff"
	if string(token) != expected {
		t.Errorf("%s != %s", string(token), expected)
	}
}
