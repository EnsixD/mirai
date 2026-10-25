package server

import "testing"

func TestRootToken(t *testing.T) {
	for seg, want := range map[string]bool{
		"oaa5kdh1nnz7omns": true, "Zt4mkG5Kbgkw": true, "abcdefghi": true,
		"abcdefgh": false, "favicon.png": false, "assets": false, "a-b-c-d-e-f": false,
		"0123456789abcdef0123456789abcdef0": false,
	} {
		if got := rootToken(seg); got != want {
			t.Errorf("rootToken(%q) = %v, want %v", seg, got, want)
		}
	}
}
