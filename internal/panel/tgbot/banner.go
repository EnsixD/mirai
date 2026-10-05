package tgbot

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
)

const KeyBanner = "tg_banner_file"
const KeyBannerPreview = "tg_banner_preview"

// UploadBanner validates a private panel upload and caches Telegram's reusable file ID.
func (b *Bot) UploadBanner(ctx context.Context, data string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, documentTimeout)
	defer cancel()
	_, encoded, ok := strings.Cut(data, ",")
	if !ok || !(strings.HasPrefix(data, "data:image/png;base64,") || strings.HasPrefix(data, "data:image/jpeg;base64,")) {
		return "", errors.New("Use a PNG or JPEG image")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) > 512<<10 {
		return "", errors.New("Image must be at most 512 KB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width+cfg.Height > 10000 || cfg.Width > 20*cfg.Height || cfg.Height > 20*cfg.Width {
		return "", errors.New("Invalid image dimensions")
	}
	chat, _, err := settings.Get[int64](ctx, b.d.Settings, KeyAdminID)
	if err != nil || chat == 0 {
		return "", errors.New("Set the administrator Telegram ID and send /start to the bot first")
	}
	c, err := b.InfrastructureClient(ctx)
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("chat_id", strconv.FormatInt(chat, 10))
	_ = w.WriteField("disable_notification", "true")
	part, err := w.CreateFormFile("photo", "banner."+format)
	if err != nil {
		return "", err
	}
	_, _ = part.Write(raw)
	_ = w.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+c.token+"/sendPhoto", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	var m Message
	if err := c.do(ctx, req, &m); err != nil {
		return "", errors.New("Telegram could not accept the image; check the bot and administrator chat")
	}
	_ = c.Delete(ctx, chat, m.MessageID)
	if len(m.Photo) == 0 {
		return "", errors.New("Telegram did not return a photo")
	}
	return m.Photo[len(m.Photo)-1].FileID, nil
}

func (c *Client) SendPhoto(ctx context.Context, chat int64, photo, text string, kb *Keyboard) (Message, error) {
	var m Message
	err := c.call(ctx, "sendPhoto", map[string]any{"chat_id": chat, "photo": photo, "caption": text, "parse_mode": "HTML", "reply_markup": kb}, &m)
	return m, err
}

func (c *Client) EditPhoto(ctx context.Context, chat, msg int64, photo, text string, kb *Keyboard) error {
	err := c.call(ctx, "editMessageMedia", map[string]any{"chat_id": chat, "message_id": msg, "media": map[string]any{"type": "photo", "media": photo, "caption": text, "parse_mode": "HTML"}, "reply_markup": kb}, nil)
	var ae *APIError
	if errors.As(err, &ae) && strings.Contains(ae.Description, "message is not modified") {
		return nil
	}
	return err
}

func (b *Bot) clearBannerHeader(ctx context.Context, c *Client, chat int64) {
	id, _ := b.d.Store.Q.TelegramBannerMessage(ctx, chat)
	if id != 0 {
		_ = c.Delete(ctx, chat, id)
		_ = b.d.Store.Q.SetTelegramBannerMessage(ctx, chat, 0)
	}
}

func (b *Bot) ensureBanner(ctx context.Context, c *Client, chat int64, photo string) (bool, error) {
	header, _ := b.d.Store.Q.TelegramBannerMessage(ctx, chat)
	if header != 0 {
		old, _ := b.d.Store.Q.TelegramBannerFile(ctx, chat)
		if old == photo {
			return false, nil
		}
		if err := c.EditPhoto(ctx, chat, header, photo, "", nil); err == nil {
			return false, b.d.Store.Q.SetTelegramBannerMessage(ctx, chat, header, photo)
		} else {
			var ae *APIError
			if errors.As(err, &ae) && ae.Code == 429 {
				return false, err
			}
		}
	}
	sent, err := c.SendPhoto(ctx, chat, photo, "", nil)
	if err != nil {
		return false, err
	}
	if err := b.d.Store.Q.SetTelegramBannerMessage(ctx, chat, sent.MessageID, photo); err != nil {
		_ = c.Delete(ctx, chat, sent.MessageID)
		return false, err
	}
	return true, nil
}

// A persistent photo sits above the single editable screen message. This permits
// arbitrary screen text length without Telegram's photo-caption limit.
func (b *Bot) sendScreen(ctx context.Context, c *Client, chat int64, text string, kb *Keyboard) (Message, error) {
	photo, _ := b.d.Settings.String(ctx, KeyBanner)
	if photo == "" {
		b.clearBannerHeader(ctx, c, chat)
		return c.Send(ctx, chat, text, kb, false)
	}
	added, err := b.ensureBanner(ctx, c, chat, photo)
	if err != nil {
		return Message{}, err
	}
	if current, err := b.d.Store.Q.GetTgChat(ctx, chat); err == nil && current.MenuMsgID != 0 {
		if !added {
			if err := c.Edit(ctx, chat, current.MenuMsgID, text, kb); err == nil {
				return Message{MessageID: current.MenuMsgID, Chat: Chat{ID: chat}}, nil
			} else {
				var ae *APIError
				if errors.As(err, &ae) && ae.Code == 429 {
					return Message{}, err
				}
			}
		}
		sent, err := c.Send(ctx, chat, text, kb, false)
		if err == nil {
			_ = c.Delete(ctx, chat, current.MenuMsgID)
			_ = b.d.Store.Q.SetTgMenu(ctx, db.SetTgMenuParams{MenuMsgID: sent.MessageID, TgID: chat})
		}
		return sent, err
	}
	sent, err := c.Send(ctx, chat, text, kb, false)
	if err == nil {
		_ = b.d.Store.Q.SetTgMenu(ctx, db.SetTgMenuParams{MenuMsgID: sent.MessageID, TgID: chat})
	}
	return sent, err
}

func (b *Bot) editScreen(ctx context.Context, c *Client, m *Message, text string, kb *Keyboard) (int64, error) {
	chat := m.Chat.ID
	photo, _ := b.d.Settings.String(ctx, KeyBanner)
	if photo == "" {
		b.clearBannerHeader(ctx, c, chat)
	} else {
		added, err := b.ensureBanner(ctx, c, chat, photo)
		if err != nil {
			return m.MessageID, err
		}
		if added {
			// First enable on an existing menu: recreate once to put the banner above it.
			next, err := c.Send(ctx, chat, text, kb, false)
			if err != nil {
				return m.MessageID, err
			}
			_ = c.Delete(ctx, chat, m.MessageID)
			return next.MessageID, nil
		}
	}
	return m.MessageID, c.Edit(ctx, chat, m.MessageID, text, kb)
}
