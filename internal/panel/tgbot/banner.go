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
	in := map[string]any{"chat_id": chat, "photo": photo, "caption": text, "parse_mode": "HTML"}
	if kb != nil {
		in["reply_markup"] = kb
	}
	err := c.call(ctx, "sendPhoto", in, &m)
	return m, err
}

func (c *Client) EditPhoto(ctx context.Context, chat, msg int64, photo, text string, kb *Keyboard) error {
	in := map[string]any{"chat_id": chat, "message_id": msg, "media": map[string]any{"type": "photo", "media": photo, "caption": text, "parse_mode": "HTML"}}
	if kb != nil {
		in["reply_markup"] = kb
	}
	err := c.call(ctx, "editMessageMedia", in, nil)
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

// EditCaption changes only the text and buttons, retaining the existing photo.
func (c *Client) EditCaption(ctx context.Context, chat, msg int64, text string, kb *Keyboard) error {
	in := map[string]any{"chat_id": chat, "message_id": msg, "caption": text, "parse_mode": "HTML"}
	if kb != nil {
		in["reply_markup"] = kb
	}
	err := c.call(ctx, "editMessageCaption", in, nil)
	var ae *APIError
	if errors.As(err, &ae) && strings.Contains(ae.Description, "message is not modified") {
		return nil
	}
	return err
}

func (b *Bot) sendScreen(ctx context.Context, c *Client, chat int64, text string, kb *Keyboard) (Message, error) {
	if current, err := b.d.Store.Q.GetTgChat(ctx, chat); err == nil && current.MenuMsgID != 0 {
		id, err := b.editScreen(ctx, c, &Message{MessageID: current.MenuMsgID, Chat: Chat{ID: chat}}, text, kb)
		if err == nil {
			return Message{MessageID: id, Chat: Chat{ID: chat}}, nil
		}
		var ae *APIError
		if errors.As(err, &ae) && ae.Code == 429 {
			return Message{}, err
		}
	}
	photo, _ := b.d.Settings.String(ctx, KeyBanner)
	var sent Message
	var err error
	if photo != "" {
		sent, err = c.SendPhoto(ctx, chat, photo, text, kb)
		if err == nil {
			_ = b.d.Store.Q.SetTelegramBannerMessage(ctx, chat, sent.MessageID, photo)
		}
	}
	if photo == "" || err != nil {
		sent, err = c.Send(ctx, chat, text, kb, false)
	}
	if err == nil {
		_ = b.d.Store.Q.SetTgMenu(ctx, db.SetTgMenuParams{MenuMsgID: sent.MessageID, TgID: chat})
	}
	return sent, err
}

func (b *Bot) editScreen(ctx context.Context, c *Client, m *Message, text string, kb *Keyboard) (int64, error) {
	chat := m.Chat.ID
	photo, _ := b.d.Settings.String(ctx, KeyBanner)
	header, _ := b.d.Store.Q.TelegramBannerMessage(ctx, chat)
	old, _ := b.d.Store.Q.TelegramBannerFile(ctx, chat)
	if photo != "" {
		// Adopt an old standalone header once, combining its caption and keyboard.
		if header != 0 {
			var err error
			if old == photo {
				err = c.EditCaption(ctx, chat, header, text, kb)
			} else {
				err = c.EditPhoto(ctx, chat, header, photo, text, kb)
			}
			if err == nil {
				if header != m.MessageID {
					_ = c.Delete(ctx, chat, m.MessageID)
				}
				_ = b.d.Store.Q.SetTelegramBannerMessage(ctx, chat, header, photo)
				return header, nil
			}
			var ae *APIError
			if errors.As(err, &ae) && ae.Code == 429 {
				return m.MessageID, err
			}
		}
		sent, err := c.SendPhoto(ctx, chat, photo, text, kb)
		if err == nil {
			if header != 0 && header != m.MessageID {
				_ = c.Delete(ctx, chat, header)
			}
			_ = c.Delete(ctx, chat, m.MessageID)
			_ = b.d.Store.Q.SetTelegramBannerMessage(ctx, chat, sent.MessageID, photo)
			return sent.MessageID, nil
		}
		var ae *APIError
		if errors.As(err, &ae) && ae.Code == 429 {
			return m.MessageID, err
		}
	}
	// An unavailable photo or oversized caption must never block navigation.
	if header != m.MessageID && len(m.Photo) == 0 {
		if err := c.Edit(ctx, chat, m.MessageID, text, kb); err == nil {
			b.clearBannerHeader(ctx, c, chat)
			return m.MessageID, nil
		}
	}
	sent, err := c.Send(ctx, chat, text, kb, false)
	if err != nil {
		return m.MessageID, err
	}
	b.clearBannerHeader(ctx, c, chat)
	if header != m.MessageID {
		_ = c.Delete(ctx, chat, m.MessageID)
	}
	return sent.MessageID, nil
}
