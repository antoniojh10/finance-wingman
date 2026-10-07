package oauth

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

type step string

const (
	stepEmail     step = "email"
	stepCode      step = "code"
	stepWorkspace step = "workspace"
	stepError     step = "error"
)

// Message keys used for errors on the sign-in page.
const (
	msgUnknownClient   = "unknown_client"
	msgInvalidRedirect = "invalid_redirect"
	msgExpired         = "expired"
	msgServerError     = "server_error"
	msgInvalidEmail    = "invalid_email"
	msgSendFailed      = "send_failed"
	msgInvalidCode     = "invalid_code"
	msgNoWorkspace     = "no_workspace"
	msgChooseWorkspace = "choose_workspace"
)

type pageData struct {
	Locale       string
	Step         step
	RequestID    string
	ClientName   string
	RedirectHost string
	Email        string
	Error        string
	Workspaces   []workspaceOption
}

type workspaceOption struct {
	ID       string
	Name     string
	Selected bool
}

var copies = map[string]map[string]string{
	"en": {
		"title":            "Connect to Finance Wingman",
		"heading":          "Connect %s",
		"intro":            "%s wants to access your Finance Wingman workspace. It will be able to view your accounts and transactions and record new ones.",
		"redirect":         "After approving you will return to %s.",
		"email_label":      "Email",
		"send_code":        "Email me a code",
		"code_sent":        "If %s has access, we sent a 6-digit code to it.",
		"code_label":       "Code",
		"approve":          "Approve access",
		"change_email":     "Use a different email",
		"deny":             "Cancel",
		"error_title":      "Something went wrong",
		"workspace_intro":  "Choose the workspace %s will use. To use another one later, connect it again.",
		"continue":         "Continue",
		msgUnknownClient:   "This application is not registered. Try connecting again from the app.",
		msgInvalidRedirect: "The application's redirect address is not valid.",
		msgExpired:         "This sign-in request has expired. Start again from the app.",
		msgServerError:     "An unexpected error occurred. Please try again.",
		msgInvalidEmail:    "Enter a valid email address.",
		msgSendFailed:      "We couldn't send the email. Please try again.",
		msgInvalidCode:     "That code is invalid or has expired.",
		msgNoWorkspace:     "Your account doesn't belong to any workspace yet. Create one in Finance Wingman, then connect again.",
		msgChooseWorkspace: "Choose one of your workspaces.",
	},
	"es": {
		"title":            "Conectar con Finance Wingman",
		"heading":          "Conectar %s",
		"intro":            "%s quiere acceder a tu espacio de Finance Wingman. Podrá ver tus cuentas y transacciones y registrar nuevas.",
		"redirect":         "Después de aprobar volverás a %s.",
		"email_label":      "Correo electrónico",
		"send_code":        "Enviarme un código",
		"code_sent":        "Si %s tiene acceso, le enviamos un código de 6 dígitos.",
		"code_label":       "Código",
		"approve":          "Aprobar acceso",
		"change_email":     "Usar otro correo",
		"deny":             "Cancelar",
		"error_title":      "Algo salió mal",
		"workspace_intro":  "Elige el espacio que usará %s. Para usar otro más adelante, vuelve a conectarlo.",
		"continue":         "Continuar",
		msgUnknownClient:   "Esta aplicación no está registrada. Intenta conectarla de nuevo desde la app.",
		msgInvalidRedirect: "La dirección de retorno de la aplicación no es válida.",
		msgExpired:         "Esta solicitud expiró. Empieza de nuevo desde la app.",
		msgServerError:     "Ocurrió un error inesperado. Inténtalo de nuevo.",
		msgInvalidEmail:    "Escribe un correo electrónico válido.",
		msgSendFailed:      "No pudimos enviar el correo. Inténtalo de nuevo.",
		msgInvalidCode:     "El código no es válido o ya expiró.",
		msgNoWorkspace:     "Tu cuenta todavía no pertenece a ningún espacio. Crea uno en Finance Wingman y vuelve a conectar.",
		msgChooseWorkspace: "Elige uno de tus espacios.",
	},
}

// pickLocale chooses "es" or "en" from an explicit value or Accept-Language.
func pickLocale(explicit, acceptLanguage string) string {
	for _, candidate := range strings.Fields(explicit) {
		if _, ok := copies[candidate]; ok {
			return candidate
		}
	}
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.Split(part, ";")[0]))
		lang := strings.Split(tag, "-")[0]
		if _, ok := copies[lang]; ok {
			return lang
		}
	}
	return "en"
}

var pageTemplate = template.Must(template.New("page").Funcs(template.FuncMap{
	"t": func(locale, key string, args ...any) string {
		text := copies[locale][key]
		if len(args) > 0 {
			return sprintf(text, args...)
		}
		return text
	},
}).Parse(`<!doctype html>
<html lang="{{.Locale}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>{{t .Locale "title"}}</title>
<style>
:root{color-scheme:light dark;--bg:#f4f4f5;--card:#fff;--text:#18181b;--muted:#71717a;--border:#e4e4e7;--accent:#18181b;--accent-text:#fff;--error:#b91c1c}
@media (prefers-color-scheme:dark){:root{--bg:#09090b;--card:#18181b;--text:#fafafa;--muted:#a1a1aa;--border:#27272a;--accent:#fafafa;--accent-text:#18181b;--error:#f87171}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:16px;background:var(--bg);color:var(--text);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif}
main{width:100%;max-width:400px;background:var(--card);border:1px solid var(--border);border-radius:16px;padding:28px}
h1{font-size:22px;margin:0 0 12px}
p{line-height:1.5;margin:0 0 16px}
.muted{color:var(--muted);font-size:14px}
.error{color:var(--error);font-size:14px}
label{display:block;font-size:14px;font-weight:600;margin-bottom:6px}
input{width:100%;font-size:16px;padding:12px;border-radius:10px;border:1px solid var(--border);background:transparent;color:var(--text);margin-bottom:16px}
input.code{font-size:24px;letter-spacing:8px;text-align:center;font-family:ui-monospace,monospace}
button{width:100%;font-size:16px;font-weight:600;padding:12px;border-radius:10px;border:1px solid var(--accent);background:var(--accent);color:var(--accent-text);cursor:pointer}
button.secondary{background:transparent;color:var(--text);border-color:var(--border);margin-top:8px}
form.inline{margin:0}
fieldset{border:0;padding:0;margin:0 0 16px}
label.option{display:flex;align-items:center;gap:10px;font-weight:500;padding:12px;border:1px solid var(--border);border-radius:10px;margin-bottom:8px;cursor:pointer}
label.option input{width:auto;margin:0}
</style>
</head>
<body>
<main>
{{if eq .Step "error"}}
  <h1>{{t .Locale "error_title"}}</h1>
  <p class="error">{{t .Locale .Error}}</p>
{{else}}
  <h1>{{t .Locale "heading" .ClientName}}</h1>
  <p>{{t .Locale "intro" .ClientName}}</p>
  <p class="muted">{{t .Locale "redirect" .RedirectHost}}</p>
  {{if .Error}}<p class="error" role="alert">{{t .Locale .Error}}</p>{{end}}
  {{if eq .Step "email"}}
  <form method="post" action="/oauth/authorize">
    <input type="hidden" name="request_id" value="{{.RequestID}}">
    <input type="hidden" name="locale" value="{{.Locale}}">
    <input type="hidden" name="action" value="send_code">
    <label for="email">{{t .Locale "email_label"}}</label>
    <input id="email" name="email" type="email" autocomplete="email" required autofocus value="{{.Email}}">
    <button type="submit">{{t .Locale "send_code"}}</button>
  </form>
  {{else if eq .Step "workspace"}}
  <p>{{t .Locale "workspace_intro" .ClientName}}</p>
  <form method="post" action="/oauth/authorize">
    <input type="hidden" name="request_id" value="{{.RequestID}}">
    <input type="hidden" name="locale" value="{{.Locale}}">
    <input type="hidden" name="action" value="choose_workspace">
    <fieldset>
    {{range .Workspaces}}
    <label class="option"><input type="radio" name="workspace_id" value="{{.ID}}" required{{if .Selected}} checked{{end}}> {{.Name}}</label>
    {{end}}
    </fieldset>
    <button type="submit">{{t .Locale "continue"}}</button>
  </form>
  {{else}}
  <p class="muted">{{t .Locale "code_sent" .Email}}</p>
  <form method="post" action="/oauth/authorize">
    <input type="hidden" name="request_id" value="{{.RequestID}}">
    <input type="hidden" name="locale" value="{{.Locale}}">
    <input type="hidden" name="action" value="verify">
    <label for="code">{{t .Locale "code_label"}}</label>
    <input id="code" class="code" name="code" inputmode="numeric" pattern="[0-9]{6}" maxlength="6" autocomplete="one-time-code" required autofocus>
    <button type="submit">{{t .Locale "approve"}}</button>
  </form>
  <form class="inline" method="post" action="/oauth/authorize">
    <input type="hidden" name="request_id" value="{{.RequestID}}">
    <input type="hidden" name="locale" value="{{.Locale}}">
    <input type="hidden" name="action" value="change_email">
    <button class="secondary" type="submit">{{t .Locale "change_email"}}</button>
  </form>
  {{end}}
  <form class="inline" method="post" action="/oauth/authorize">
    <input type="hidden" name="request_id" value="{{.RequestID}}">
    <input type="hidden" name="locale" value="{{.Locale}}">
    <input type="hidden" name="action" value="deny">
    <button class="secondary" type="submit">{{t .Locale "deny"}}</button>
  </form>
{{end}}
</main>
</body>
</html>`))

func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, status int, data pageData) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
	h.Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	if err := pageTemplate.Execute(w, data); err != nil {
		s.logger.ErrorContext(r.Context(), "oauth: render page", "error", err)
	}
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, locale string, status int, message string) {
	s.renderPage(w, r, status, pageData{Locale: locale, Step: stepError, Error: message})
}

func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
