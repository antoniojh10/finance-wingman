package mail

import (
	"bytes"
	"fmt"
	"html/template"
	textTemplate "text/template"
)

type loginCopy struct {
	Subject, Greeting, Intro, Button, CodeIntro, Expiry, Ignore string
	// Used when the code is requested to connect an app (no link).
	ConnectSubject, ConnectIntro, ConnectExpiry string
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

		ConnectSubject: "Your code to connect %s to Finance Wingman",
		ConnectIntro:   "%s is requesting access to your Finance Wingman data. Enter this code to approve it:",
		ConnectExpiry:  "The code expires in %d minutes and can be used once.",
	},
	"es": {
		Subject:   "Tu enlace para entrar a Finance Wingman",
		Greeting:  "Hola",
		Intro:     "Usa el botón de abajo para entrar a Finance Wingman.",
		Button:    "Entrar",
		CodeIntro: "O escribe este código:",
		Expiry:    "El enlace y el código expiran en %d minutos y solo pueden usarse una vez.",
		Ignore:    "Si no solicitaste este correo, puedes ignorarlo.",

		ConnectSubject: "Tu código para conectar %s con Finance Wingman",
		ConnectIntro:   "%s está solicitando acceso a tus datos de Finance Wingman. Escribe este código para aprobarlo:",
		ConnectExpiry:  "El código expira en %d minutos y solo puede usarse una vez.",
	},
}

type LoginEmail struct {
	To         string
	Name       string
	Locale     string
	Link       string // omitted when connecting an app
	Code       string
	TTLMinutes int
	// ClientName is set when the code authorizes an app (e.g. "Claude").
	ClientName string
}

var loginHTML = template.Must(template.New("login").Parse(`<!doctype html>
<html><body style="margin:0;padding:24px;background:#f4f4f5;font-family:-apple-system,Segoe UI,Roboto,sans-serif;color:#18181b">
<table role="presentation" width="100%" style="max-width:480px;margin:0 auto;background:#ffffff;border-radius:12px;padding:32px">
<tr><td>
<p style="font-size:16px;margin:0 0 16px">{{.Copy.Greeting}}{{if .Name}} {{.Name}}{{end}},</p>
<p style="font-size:16px;margin:0 0 24px">{{.Intro}}</p>
{{if .Link}}<p style="margin:0 0 24px"><a href="{{.Link}}" style="display:inline-block;background:#18181b;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:8px;font-weight:600">{{.Copy.Button}}</a></p>
<p style="font-size:14px;margin:0 0 8px;color:#52525b">{{.Copy.CodeIntro}}</p>{{end}}
<p style="font-size:28px;letter-spacing:6px;font-weight:700;margin:0 0 24px;font-family:ui-monospace,monospace">{{.Code}}</p>
<p style="font-size:13px;margin:0 0 8px;color:#71717a">{{.Expiry}}</p>
<p style="font-size:13px;margin:0;color:#71717a">{{.Copy.Ignore}}</p>
</td></tr></table>
</body></html>`))

var loginText = textTemplate.Must(textTemplate.New("login").Parse(`{{.Copy.Greeting}}{{if .Name}} {{.Name}}{{end}},

{{.Intro}}
{{if .Link}}
{{.Link}}

{{.Copy.CodeIntro}} {{.Code}}
{{else}}
{{.Code}}
{{end}}
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
	subject, intro, expiry := copy.Subject, copy.Intro, copy.Expiry
	if e.ClientName != "" {
		subject = fmt.Sprintf(copy.ConnectSubject, e.ClientName)
		intro = fmt.Sprintf(copy.ConnectIntro, e.ClientName)
		expiry = copy.ConnectExpiry
	}
	data := map[string]any{
		"Copy":   copy,
		"Intro":  intro,
		"Name":   e.Name,
		"Link":   e.Link,
		"Code":   e.Code,
		"Expiry": fmt.Sprintf(expiry, e.TTLMinutes),
	}
	var html, text bytes.Buffer
	if err := loginHTML.Execute(&html, data); err != nil {
		return Message{}, err
	}
	if err := loginText.Execute(&text, data); err != nil {
		return Message{}, err
	}
	return Message{To: e.To, Subject: subject, HTML: html.String(), Text: text.String()}, nil
}
