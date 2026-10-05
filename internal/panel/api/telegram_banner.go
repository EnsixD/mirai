package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"mirai/internal/panel/tgbot"
)

type telegramBannerOutput struct {
	ContentType  string `header:"Content-Type"`
	CacheControl string `header:"Cache-Control"`
	Body         []byte
}

func (h *handlers) telegramBanner(ctx context.Context, _ *struct{}) (*telegramBannerOutput, error) {
	data, err := h.d.Settings.String(ctx, tgbot.KeyBannerPreview)
	if err != nil {
		return nil, err
	}
	prefix, encoded, ok := strings.Cut(data, ",")
	if !ok {
		return nil, huma.Error404NotFound("banner_not_set")
	}
	mime := "image/jpeg"
	if prefix == "data:image/png;base64" {
		mime = "image/png"
	}
	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, huma.Error404NotFound("banner_not_set")
	}
	return &telegramBannerOutput{ContentType: mime, CacheControl: "private, max-age=3600", Body: body}, nil
}

func (h *handlers) bannerPreviewURL(ctx context.Context) string {
	file, _ := h.d.Settings.String(ctx, tgbot.KeyBanner)
	if file == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(file))
	return fmt.Sprintf("api/v1/telegram/banner?v=%x", digest[:8])
}

func (h *handlers) registerTelegramBanner() {
	huma.Register(h.api, huma.Operation{OperationID: "telegram-banner-preview", Metadata: sessionOnly, Extensions: sessionOnlyExt, Method: http.MethodGet, Path: "/api/v1/telegram/banner", Summary: "Private Telegram banner preview", Tags: []string{"telegram"}, Responses: map[string]*huma.Response{"200": {Description: "Banner image", Content: map[string]*huma.MediaType{"image/png": {}, "image/jpeg": {}}}}}, h.telegramBanner)
}
