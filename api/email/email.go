package email

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"

	"github.com/mailgun/mailgun-go/v3"
	"github.com/okteto/app/api/log"
)

const (
	logoPath = "assets/images/okteto.png"
)

var (
	invite     *template.Template
	inviteText = template.Must(template.New("inviteText").Parse(inviteTextTmpl))
	mg         *mailgun.MailgunImpl
	sender     = "hello@okteto.com"
)

type noopImpl struct {
	mailgun.MailgunImpl
}

func init() {
	apiKey := os.Getenv("MG_API_KEY")
	domain := os.Getenv("MG_DOMAIN")

	if len(apiKey) > 0 {
		mg = mailgun.NewMailgun(domain, apiKey)
	} else {
		mg = nil
	}

	invite = template.Must(template.ParseFiles("assets/emails/invite.html.tmpl"))
}

// Invite sends an invite to email
func Invite(ctx context.Context, url, email, receiver string) error {
	d := struct {
		URL  string
		User string
	}{
		URL:  url,
		User: email,
	}

	text := &bytes.Buffer{}
	if err := inviteText.Execute(text, d); err != nil {
		return err
	}

	html := &bytes.Buffer{}
	if err := invite.Execute(html, d); err != nil {
		return err
	}

	subject := fmt.Sprintf("%s has shared his Okteto space with you", email)
	if mg != nil {
		message := mg.NewMessage(sender, subject, text.String(), receiver)
		message.SetHtml(html.String())
		message.AddInline(logoPath)

		if _, _, err := mg.Send(ctx, message); err != nil {
			return err
		}
		log.Debugf("email sent to %s", receiver)
		return nil
	}

	log.Debugf("would've sent invite email if enabled")
	return nil
}
