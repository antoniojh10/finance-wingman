package mail

import (
	"bytes"
	"fmt"
	"html/template"
	textTemplate "text/template"
)

type loginCopy struct {
	Subject, Greeting, Intro, Button, CodeIntro, Expiry, Ignore string
}

var loginCopies = map[string]loginCopy{
	"en": {
		Subject:   "Your Finance Wingman sign-in link",
		Greeting:  "Hi",
		Intro:     "Use the button below to sign in to Finance Wingman.",
		Button:    "Sign in",
		CodeIntro: "Or enter this code:",
		Expiry:    "The link and code expire in %d minutes and can be used once.",
		Ignore:    "If you didn't request this email, you can safely ignore it.",
	},
	"es": {
		Subject:   "Tu enlace para entrar a Finance Wingman",
		Greeting:  "Hola",
		Intro:     "Usa el botón de abajo para entrar a Finance Wingman.",
		Button:    "Entrar",
		CodeIntro: "O escribe este código:",
		Expiry:    "El enlace y el código expiran en %d minutos y solo pueden usarse una vez.",
		Ignore:    "Si no solicitaste este correo, puedes ignorarlo.",
	},
}

type LoginEmail struct {
	To         string
	Name       string
	Locale     string
	Link       string
	Code       string
	TTLMinutes int
}

var loginHTML = template.Must(template.New("login").Parse(`<!doctype html>
<html><body style="margin:0;padding:24px;background:#f4f4f5;font-family:-apple-system,Segoe UI,Roboto,sans-serif;color:#18181b">
<table role="presentation" width="100%" style="max-width:480px;margin:0 auto;background:#ffffff;border-radius:12px;padding:32px">
<tr><td>
<p style="font-size:16px;margin:0 0 16px">{{.Copy.Greeting}}{{if .Name}} {{.Name}}{{end}},</p>
<p style="font-size:16px;margin:0 0 24px">{{.Copy.Intro}}</p>
<p style="margin:0 0 24px"><a href="{{.Link}}" style="display:inline-block;background:#18181b;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:8px;font-weight:600">{{.Copy.Button}}</a></p>
<p style="font-size:14px;margin:0 0 8px;color:#52525b">{{.Copy.CodeIntro}}</p>
<p style="font-size:28px;letter-spacing:6px;font-weight:700;margin:0 0 24px;font-family:ui-monospace,monospace">{{.Code}}</p>
<p style="font-size:13px;margin:0 0 8px;color:#71717a">{{.Expiry}}</p>
<p style="font-size:13px;margin:0;color:#71717a">{{.Copy.Ignore}}</p>
</td></tr></table>
</body></html>`))

var loginText = textTemplate.Must(textTemplate.New("login").Parse(`{{.Copy.Greeting}}{{if .Name}} {{.Name}}{{end}},

{{.Copy.Intro}}

{{.Link}}

{{.Copy.CodeIntro}} {{.Code}}

{{.Expiry}}
{{.Copy.Ignore}}
`))

// RenderLogin builds the localized sign-in email. Unknown locales fall back
// to English.
func RenderLogin(e LoginEmail) (Message, error) {
	copy, ok := loginCopies[e.Locale]
	if !ok {
		copy = loginCopies["en"]
	}
	data := map[string]any{
		"Copy":   copy,
		"Name":   e.Name,
		"Link":   e.Link,
		"Code":   e.Code,
		"Expiry": fmt.Sprintf(copy.Expiry, e.TTLMinutes),
	}
	var html, text bytes.Buffer
	if err := loginHTML.Execute(&html, data); err != nil {
		return Message{}, err
	}
	if err := loginText.Execute(&text, data); err != nil {
		return Message{}, err
	}
	return Message{To: e.To, Subject: copy.Subject, HTML: html.String(), Text: text.String()}, nil
}
