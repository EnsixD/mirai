package settings

import (
	"context"
	"errors"
	"net/url"
	"path"
	"strings"
)

const KeySubPublicURL = "sub_public_url"
const KeySubIDLength = "sub_id_length"

func ValidateSubscriptionURL(value string) error {
	if value == "" {
		return nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(u.Path, "?#") || (u.Path != "" && strings.TrimRight(path.Clean(u.Path), "/") != strings.TrimRight(u.Path, "/")) {
		return errors.New("invalid_subscription_url")
	}
	return nil
}
func (s *Settings) SubscriptionURL(ctx context.Context) string {
	value, _ := s.String(ctx, KeySubPublicURL)
	return strings.TrimRight(value, "/")
}
