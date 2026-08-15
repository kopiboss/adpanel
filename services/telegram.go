package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"adpanel/config"
	"adpanel/models"
)

const telegramAPIBase = "https://api.telegram.org/bot"

type TelegramBot struct {
	token  string
	chatID string
}

var Bot *TelegramBot

func InitTelegramBot() {
	token := config.App.TelegramBotToken
	chatID := config.App.TelegramChatID

	// Try DB settings if env not set
	if token == "" {
		if t, err := models.GetSetting("TELEGRAM_BOT_TOKEN"); err == nil {
			token = t
		}
	}
	if chatID == "" {
		if c, err := models.GetSetting("TELEGRAM_CHAT_ID"); err == nil {
			chatID = c
		}
	}

	Bot = &TelegramBot{token: token, chatID: chatID}
	log.Printf("Telegram bot initialized (token: %v)", token != "")
}

func (b *TelegramBot) ReloadConfig() {
	if t, err := models.GetSetting("TELEGRAM_BOT_TOKEN"); err == nil && t != "" {
		b.token = t
	}
	if c, err := models.GetSetting("TELEGRAM_CHAT_ID"); err == nil && c != "" {
		b.chatID = c
	}
}

func (b *TelegramBot) IsConfigured() bool {
	return b.token != "" && b.chatID != ""
}

type inlineKeyboard struct {
	InlineKeyboard [][]inlineButton `json:"inline_keyboard"`
}

type inlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type sendMessageReq struct {
	ChatID      string          `json:"chat_id"`
	Text        string          `json:"text"`
	ParseMode   string          `json:"parse_mode,omitempty"`
	ReplyMarkup *inlineKeyboard `json:"reply_markup,omitempty"`
}

func (b *TelegramBot) SendMessage(chatID, text string) error {
	if b.token == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	req := sendMessageReq{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "HTML",
	}

	return b.doRequest("sendMessage", req)
}

func (b *TelegramBot) NotifyNewUser(user *models.User) error {
	if !b.IsConfigured() {
		return nil
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	t := user.CreatedAt.In(loc).Format("02 Jan 2006 15:04 WIB")

	text := fmt.Sprintf(
		"👤 <b>User Baru Mendaftar</b>\n\nNama  : %s\nEmail : %s\nWaktu : %s",
		user.Name, user.Email, t,
	)

	req := sendMessageReq{
		ChatID:    b.chatID,
		Text:      text,
		ParseMode: "HTML",
		ReplyMarkup: &inlineKeyboard{
			InlineKeyboard: [][]inlineButton{
				{
					{Text: "✅ Approve", CallbackData: fmt.Sprintf("approve_user_%d", user.ID)},
					{Text: "❌ Reject", CallbackData: fmt.Sprintf("reject_user_%d", user.ID)},
				},
			},
		},
	}

	return b.doRequest("sendMessage", req)
}

func (b *TelegramBot) NotifyTokenError(user *models.User, credLabel string) {
	if user.TelegramChatID == "" {
		return
	}

	text := fmt.Sprintf(
		"⚠️ <b>Token Error</b>\n\nKredensial: %s\nStatus token tidak valid atau expired.\nSilakan update token di AdPanel.",
		credLabel,
	)

	_ = b.SendMessage(user.TelegramChatID, text)
}

func (b *TelegramBot) AnswerCallbackQuery(callbackID, text string) error {
	body := map[string]string{"callback_query_id": callbackID, "text": text}
	return b.doRequest("answerCallbackQuery", body)
}

func (b *TelegramBot) EditMessageText(chatID string, messageID int, text string) error {
	body := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
		"parse_mode": "HTML",
	}
	return b.doRequest("editMessageText", body)
}

func (b *TelegramBot) doRequest(method string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s%s/%s", telegramAPIBase, b.token, method)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	return nil
}

type Update struct {
	UpdateID      int            `json:"update_id"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
	Message       *TgMessage     `json:"message"`
}

type CallbackQuery struct {
	ID      string     `json:"id"`
	From    TgUser     `json:"from"`
	Message *TgMessage `json:"message"`
	Data    string     `json:"data"`
}

type TgMessage struct {
	MessageID int    `json:"message_id"`
	Chat      TgChat `json:"chat"`
	Text      string `json:"text"`
}

type TgUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type TgChat struct {
	ID int64 `json:"id"`
}

type getUpdatesResp struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

func (b *TelegramBot) StartPolling(handleCallback func(update Update)) {
	if !b.IsConfigured() {
		log.Println("Telegram bot not configured, polling disabled")
		return
	}

	offset := 0
	log.Println("Telegram bot polling started")

	for {
		updates, err := b.getUpdates(offset)
		if err != nil {
			log.Printf("Telegram polling error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			handleCallback(update)
		}

		time.Sleep(1 * time.Second)
	}
}

func (b *TelegramBot) getUpdates(offset int) ([]Update, error) {
	url := fmt.Sprintf("%s%s/getUpdates?timeout=30&offset=%d", telegramAPIBase, b.token, offset)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result getUpdatesResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Result, nil
}

func HandleTelegramUpdate(update Update) {
	if update.CallbackQuery == nil {
		return
	}

	cb := update.CallbackQuery
	data := cb.Data

	if strings.HasPrefix(data, "approve_user_") {
		idStr := strings.TrimPrefix(data, "approve_user_")
		userID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return
		}

		if err := models.UpdateUserStatus(userID, "active"); err != nil {
			_ = Bot.AnswerCallbackQuery(cb.ID, "❌ Gagal approve user")
			return
		}

		user, _ := models.GetUserByID(userID)
		if user != nil {
			msg := fmt.Sprintf("✅ User <b>%s</b> (%s) telah diapprove.", user.Name, user.Email)
			_ = Bot.AnswerCallbackQuery(cb.ID, "✅ User diapprove")
			if cb.Message != nil {
				_ = Bot.EditMessageText(
					strconv.FormatInt(cb.Message.Chat.ID, 10),
					cb.Message.MessageID,
					msg,
				)
			}
		}

	} else if strings.HasPrefix(data, "reject_user_") {
		idStr := strings.TrimPrefix(data, "reject_user_")
		userID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return
		}

		user, _ := models.GetUserByID(userID)
		if err := models.DeleteUser(userID); err != nil {
			_ = Bot.AnswerCallbackQuery(cb.ID, "❌ Gagal reject user")
			return
		}

		_ = Bot.AnswerCallbackQuery(cb.ID, "❌ User direject")
		if user != nil && cb.Message != nil {
			msg := fmt.Sprintf("❌ User <b>%s</b> (%s) telah direject.", user.Name, user.Email)
			_ = Bot.EditMessageText(
				strconv.FormatInt(cb.Message.Chat.ID, 10),
				cb.Message.MessageID,
				msg,
			)
		}
	}
}
