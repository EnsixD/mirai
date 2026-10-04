package subs

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// ValidateClientRouting keeps the subscription header bounded and accepts a named
// app-native JSON profile, never a full Xray or Clash configuration.
func validateRoutingJSON(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) > 4096 {
		return errors.New("profile too large")
	}
	var profile map[string]json.RawMessage
	if json.Unmarshal([]byte(text), &profile) != nil || profile == nil {
		return errors.New("invalid profile")
	}
	var name string
	if json.Unmarshal(profile["Name"], &name) != nil || strings.TrimSpace(name) == "" {
		return errors.New("profile name required")
	}
	for _, key := range []string{"DirectSites", "DirectIp", "ProxySites", "ProxyIp", "BlockSites", "BlockIp"} {
		if raw, ok := profile[key]; ok {
			var values []string
			if string(raw) == "null" || json.Unmarshal(raw, &values) != nil {
				return errors.New("invalid rule list")
			}
		}
	}
	return nil
}

func ClientRoutingHeader(h http.Header, userAgent string, cfg Config) {
	ua := strings.ToLower(userAgent)
	var app, profile string
	switch {
	case strings.Contains(ua, "happ"):
		app, profile = "happ", cfg.HappRules
	case strings.Contains(ua, "incy"):
		app, profile = "incy", cfg.INCYRules
	default:
		return
	}
	if strings.TrimSpace(profile) == "" {
		return
	}
	link, err := ClientRoutingLink(profile, app)
	if err != nil {
		return
	}
	h.Set("routing", link)
}

// ClientRoutingLink accepts native deeplinks and old JSON settings. A profile for
// one app is never forwarded to the other app.
func ClientRoutingLink(text, app string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if len(text) > 8192 || strings.ContainsAny(text, "\r\n") && !strings.HasPrefix(text, "{") {
		return "", errors.New("invalid routing link")
	}
	if strings.HasPrefix(text, "{") {
		if err := validateRoutingJSON(text); err != nil {
			return "", err
		}
		return app + "://routing/onadd/" + base64.StdEncoding.EncodeToString([]byte(text)), nil
	}
	if text == app+"://routing/off" {
		return text, nil
	}
	for _, action := range []string{"onadd", "add"} {
		if encoded, ok := strings.CutPrefix(text, app+"://routing/"+action+"/"); ok {
			var raw []byte
			var err error
			for _, decoder := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
				raw, err = decoder.DecodeString(encoded)
				if err == nil {
					break
				}
			}
			if err != nil {
				return "", errors.New("invalid base64 profile")
			}
			if err := validateRoutingJSON(string(raw)); err != nil {
				return "", err
			}
			return text, nil
		}
	}
	return "", errors.New("invalid routing scheme")
}

func ValidateClientRouting(text string) error {
	for _, app := range []string{"happ", "incy"} {
		if strings.HasPrefix(strings.TrimSpace(text), app+"://") {
			_, err := ClientRoutingLink(text, app)
			return err
		}
	}
	return validateRoutingJSON(text)
}
