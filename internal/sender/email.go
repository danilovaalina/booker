package sender

import (
	"context"

	"booker/internal/model"

	"github.com/cockroachdb/errors"
	"github.com/wneessen/go-mail"
)

type EmailSender struct {
	client *mail.Client
	from   string
}

// NewEmailSender принимает только примитивы.
// Он универсален для Gmail (587) и Mail.ru (465).
func NewEmailSender(host string, port int, user, pass, from string) (*EmailSender, error) {
	if host == "" {
		return nil, errors.New("email sender requires host, user and password")
	}

	if port <= 0 || port > 65535 {
		return nil, errors.Newf("invalid smtp port: %d", port)
	}

	opts := []mail.Option{
		mail.WithPort(port),
	}

	if user != "" || pass != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(user),
			mail.WithPassword(pass),
		)
	}

	if port == 465 {
		opts = append(opts, mail.WithSSL())
	} else if port == 1025 {
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	} else {
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	}

	c, err := mail.NewClient(host, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create mail client")
	}

	return &EmailSender{client: c, from: from}, nil
}

func (e *EmailSender) Send(ctx context.Context, n model.Notification) error {
	m := mail.NewMsg()

	if err := m.From(e.from); err != nil {
		return errors.Wrapf(err, "failed to set sender: %s", e.from)
	}

	if err := m.To(n.Recipient); err != nil {
		return errors.Wrapf(err, "failed to set recipient: %s", n.Recipient)
	}

	m.Subject("Системное уведомление")

	contentType := mail.TypeTextPlain
	if IsHTML(n.Message) {
		contentType = mail.TypeTextHTML
	}
	m.SetBodyString(contentType, n.Message)

	if err := e.client.DialAndSendWithContext(ctx, m); err != nil {
		return errors.Wrap(err, "failed to dial and send email")
	}

	return nil
}
