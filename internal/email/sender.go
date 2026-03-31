package email

import (
	"bytes"
	"context"
	"fmt"

	resend "github.com/resend/resend-go/v2"
	"maragu.dev/gomponents"
)

type Sender struct {
	client *resend.Client
	from   string
}

func NewSender(apiKey, from string) *Sender {
	return &Sender{
		client: resend.NewClient(apiKey),
		from:   from,
	}
}

func (s *Sender) Send(ctx context.Context, to, subject string, body gomponents.Node) error {
	to = "madupuis90+resend@gmail.com" // TODO: remove when we get a domain

	var buf bytes.Buffer
	if err := body.Render(&buf); err != nil {
		return fmt.Errorf("render email: %w", err)
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		Html:    buf.String(),
	}

	_, err := s.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}
