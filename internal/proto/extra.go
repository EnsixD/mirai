package proto

// Shared reports whether a supported template shares one credential between users.
func Shared(typ string) bool { return rules[typ].shared }

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}

// Needs is what a client app must support to use an inbound.
type Needs struct {
	Type       string // listener type
	Transport  string // vless, vmess, trojan: tcp, xhttp, grpc or ws
	Encryption bool   // VLESS Encryption (post-quantum)
	Gecko      bool   // Hysteria2 with Gecko obfuscation
}

// NeedsOf describes a template for choosing which apps get it.
func NeedsOf(t Template) Needs {
	n := Needs{Type: t.Type()}
	switch n.Type {
	case "vless", "vmess", "trojan":
		n.Transport = transport(t)
	}
	n.Encryption = n.Type == "vless" && t.hasEncryption()
	n.Gecko = false
	return n
}
