package sender

import (
	"context"

	"booker/internal/model"

	"github.com/cockroachdb/errors"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// TelegramSender — отправщик через Telegram Bot API
type TelegramSender struct {
	bot *bot.Bot
}

// NewTelegramSender создаёт новый отправщик для Telegram
func NewTelegramSender(token string) (*TelegramSender, error) {
	if token == "" {
		return nil, errors.New("telegram bot token is required")
	}

	b, err := bot.New(token)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize telegram bot")
	}

	return &TelegramSender{bot: b}, nil
}

// Send отправляет уведомление в Telegram
func (t *TelegramSender) Send(ctx context.Context, n model.Notification) error {
	params := &bot.SendMessageParams{
		ChatID:    n.Recipient,
		Text:      n.Message,
		ParseMode: models.ParseModeHTML,
	}

	if IsHTML(n.Message) {
		params.ParseMode = models.ParseModeHTML
	}

	_, err := t.bot.SendMessage(ctx, params)
	if err != nil {
		return errors.Wrapf(err, "failed to send telegram message for recipient: %s", n.Recipient)
	}

	return nil
}
