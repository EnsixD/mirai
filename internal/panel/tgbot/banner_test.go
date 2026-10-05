package tgbot

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mirai/internal/panel/settings"
	"mirai/internal/panel/store/db"
)

func TestBannerScreensAndInstructionRows(t *testing.T) {
	e := adminEnv(t)
	if err := e.st.Q.UpsertTgChat(e.ctx, db.UpsertTgChatParams{TgID: 555, FirstName: "Example", CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	calls := []string{}
	next := int64(100)
	failPhoto := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		calls = append(calls, method)
		if strings.Contains(r.Header.Get("Content-Type"), "multipart") {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			file, _, err := r.FormFile("photo")
			if err != nil {
				t.Error(err)
			} else {
				file.Close()
			}
		}

		if !strings.Contains(r.Header.Get("Content-Type"), "multipart") {
			var payload map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if method == "sendPhoto" || method == "editMessageMedia" {
				if v, ok := payload["reply_markup"]; ok && string(v) == "null" {
					t.Error("nil reply_markup must be omitted")
					fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"Bad Request: object expected as reply markup"}`)
					return
				}
				if failPhoto {
					fmt.Fprint(w, `{"ok":false,"error_code":400,"description":"invalid photo"}`)
					return
				}
			}
		}
		next++
		switch method {
		case "sendPhoto":
			fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d,"chat":{"id":555},"photo":[{"file_id":"cached-banner"}]}}`, next)
		case "sendMessage":
			fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d,"chat":{"id":555}}}`, next)
		default:
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		}
	}))
	defer server.Close()
	e.bot.d.API = server.URL
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 120, 60))); err != nil {
		t.Fatal(err)
	}
	data := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes.Bytes())
	id, err := e.bot.UploadBanner(e.ctx, data)
	if err != nil || id != "cached-banner" {
		t.Fatal("image not cached", id, err)
	}
	if _, err := e.bot.UploadBanner(e.ctx, "data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("not an image"))); err == nil {
		t.Fatal("invalid image accepted")
	}
	if err := settings.Set(e.ctx, e.set, KeyBanner, id); err != nil {
		t.Fatal(err)
	}
	client := NewClient(server.URL, "fake", nil)
	_, keyboard := e.bot.instruction(e.ctx, Default("ru"))
	wire, err := json.Marshal(keyboard)
	if err != nil {
		t.Fatal(err)
	}
	var markup struct {
		Rows [][]Button `json:"inline_keyboard"`
	}
	_ = json.Unmarshal(wire, &markup)
	if len(markup.Rows) != 3 || len(markup.Rows[0]) != 2 || len(markup.Rows[1]) != 1 || len(markup.Rows[2]) != 1 || markup.Rows[2][0].CallbackData != "m" {
		t.Fatal("instruction must be 2,1,1", string(wire))
	}
	short, err := e.bot.sendScreen(e.ctx, client, 555, "<b>Profile</b>", keyboard)
	if err != nil || len(short.Photo) != 0 {
		t.Fatal("screen should edit the text beneath its persistent photo", err)
	}
	initialHeader, _ := e.st.Q.TelegramBannerMessage(e.ctx, 555)
	longText := strings.Repeat("Example ", 200)
	longID, err := e.bot.editScreen(e.ctx, client, &short, longText, keyboard)
	if err != nil {
		t.Fatal(err)
	}
	header, err := e.st.Q.TelegramBannerMessage(e.ctx, 555)
	if err != nil || header == 0 || longID != short.MessageID {
		t.Fatal("long text banner not tracked", header, err)
	}
	// Persisted header state survives a bot restart and is removed when combining again.
	restarted := New(e.bot.d)
	_, err = restarted.editScreen(e.ctx, client, &Message{MessageID: longID, Chat: Chat{ID: 555}}, "<b>Profile</b>", keyboard)
	if err != nil {
		t.Fatal(err)
	}
	header, _ = e.st.Q.TelegramBannerMessage(e.ctx, 555)
	if header == 0 || header != initialHeader {
		t.Fatal("persistent banner was removed")
	}
	photos := 0
	for _, method := range calls {
		if method == "sendPhoto" {
			photos++
		}
	}
	if photos != 2 {
		t.Fatal("banner was resent on navigation", photos)
	} // upload + initial header

	// Replacing a bad banner cannot prevent the existing menu from being edited.
	failPhoto = true
	if err := settings.Set(e.ctx, e.set, KeyBanner, "bad-photo"); err != nil {
		t.Fatal(err)
	}
	fallbackID, err := restarted.editScreen(e.ctx, client, &short, "Still works", keyboard)
	if err != nil || fallbackID != short.MessageID {
		t.Fatal("banner failure blocked navigation", err)
	}
	if _, err := restarted.sendScreen(e.ctx, client, 555, "Start still works", keyboard); err != nil {
		t.Fatal("banner failure blocked start", err)
	}
	if len(calls) < 5 {
		t.Fatal("banner transitions not exercised")
	}
}

func TestAdministratorChangeNotice(t *testing.T) {
	e := adminEnv(t)
	e.say(555, "/start")
	e.tg.wait(t, 0, "sendMessage")
	if err := e.st.Q.LinkTg(e.ctx, db.LinkTgParams{UserID: e.user.ID, TgID: 555, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	start := e.tg.count()
	e.bot.NotifySubscriptionChange(e.ctx, e.user, 0, "create", 30)
	calls := e.tg.wait(t, start, "sendMessage")
	call, _ := find(calls, "sendMessage")
	if !strings.Contains(text(call), "Вам выдана подписка") || !strings.Contains(text(call), e.bot.subURL(e.ctx, e.user)) {
		t.Fatal("grant notice missing details", text(call))
	}
	start = e.tg.count()
	e.bot.NotifySubscriptionChange(e.ctx, e.user, 0, "extend", 5)
	calls = e.tg.wait(t, start, "sendMessage")
	call, _ = find(calls, "sendMessage")
	if !strings.Contains(text(call), "5 дней") || !strings.Contains(text(call), "Ссылка не изменилась") {
		t.Fatal("extension notice missing details", text(call))
	}
}
