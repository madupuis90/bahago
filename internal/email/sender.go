package email

import (
	"context"
	"fmt"

	resend "github.com/resend/resend-go/v2"
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

func (s *Sender) Send(ctx context.Context, to, subject, htmlBody string) error {
	to = "madupuis90+resend@gmail.com" // TODO: remove when we get a domain
	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
	}

	_, err := s.client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}
