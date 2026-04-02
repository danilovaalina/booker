package sender

import (
	"context"
	"regexp"

	"booker/internal/model"
)

// Sender — интерфейс для отправки уведомлений
type Sender interface {
	Send(ctx context.Context, n model.Notification) error
}

type Option func(*MultiSender) error

// New фабрика для создания отправщика
func New(opts ...Option) (Sender, error) {
	m := &MultiSender{
		senders: make(map[string]Sender),
	}

	for _, opt := range opts {
		if err := opt(m); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// WithEmail — опция для добавления Email отправителя
func WithEmail(host string, port int, user, pass, from string) Option {
	return func(m *MultiSender) error {
		s, err := NewEmailSender(host, port, user, pass, from)
		if err != nil {
			return err
		}
		m.senders["email"] = s
		return nil
	}
}

// WithTelegram — опция для добавления Telegram
func WithTelegram(token string) Option {
	return func(m *MultiSender) error {
		s, err := NewTelegramSender(token)
		if err != nil {
			return err
		}
		m.senders["telegram"] = s
		return nil
	}
}

// MultiSender — отправщик, поддерживающий несколько каналов
type MultiSender struct {
	senders map[string]Sender
}

func (m *MultiSender) Send(ctx context.Context, n model.Notification) error {
	sender, ok := m.senders[string(n.Channel)]
	if !ok {
		return model.ErrUnsupportedChannel
	}
	return sender.Send(ctx, n)
}

func IsHTML(s string) bool {
	htmlTagRegex := regexp.MustCompile(`(?i)<[/a-z0-9]+[^>]*>`)

	return htmlTagRegex.MatchString(s)
}
