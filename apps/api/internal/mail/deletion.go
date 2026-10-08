package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	textTemplate "text/template"
	"time"
)

// notice is a plain informational email: a greeting, a few paragraphs and
// an optional button.
type notice struct {
	To, Subject, Greeting, Name string
	Paragraphs                  []string
	Button, Link                string
}

var noticeHTML = template.Must(template.New("notice").Parse(`<!doctype html>
<html><body style="margin:0;padding:24px;background:#f4f4f5;font-family:-apple-system,Segoe UI,Roboto,sans-serif;color:#18181b">
<table role="presentation" width="100%" style="max-width:480px;margin:0 auto;background:#ffffff;border-radius:12px;padding:32px">
<tr><td>
<p style="font-size:16px;margin:0 0 16px">{{.Greeting}}{{if .Name}} {{.Name}}{{end}},</p>
{{range .Paragraphs}}<p style="font-size:16px;margin:0 0 16px">{{.}}</p>
{{end}}{{if .Link}}<p style="margin:8px 0 0"><a href="{{.Link}}" style="display:inline-block;background:#18181b;color:#ffffff;text-decoration:none;padding:12px 24px;border-radius:8px;font-weight:600">{{.Button}}</a></p>{{end}}
</td></tr></table>
</body></html>`))

var noticeText = textTemplate.Must(textTemplate.New("notice").Parse(`{{.Greeting}}{{if .Name}} {{.Name}}{{end}},
{{range .Paragraphs}}
{{.}}
{{end}}{{if .Link}}
{{.Button}}: {{.Link}}
{{end}}`))

func renderNotice(n notice) (Message, error) {
	var html, text bytes.Buffer
	if err := noticeHTML.Execute(&html, n); err != nil {
		return Message{}, err
	}
	if err := noticeText.Execute(&text, n); err != nil {
		return Message{}, err
	}
	return Message{To: n.To, Subject: n.Subject, HTML: html.String(), Text: text.String()}, nil
}

var spanishMonths = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

// formatDate writes a day in the reader's language: "October 15, 2026" or
// "15 de octubre de 2026".
func formatDate(t time.Time, locale string) string {
	if locale == "es" {
		return fmt.Sprintf("%d de %s de %d", t.Day(), spanishMonths[t.Month()-1], t.Year())
	}
	return t.Format("January 2, 2006")
}

func quoteList(names []string, locale string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "“" + n + "”"
	}
	if len(quoted) <= 1 {
		return strings.Join(quoted, "")
	}
	and := " and "
	if locale == "es" {
		and = " y "
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + and + quoted[len(quoted)-1]
}

func pickLocale(locale string) string {
	if locale == "es" {
		return "es"
	}
	return "en"
}

type deletionCopy struct {
	Greeting, Backups string

	WorkspaceScheduledSubject, WorkspaceScheduledIntro, WorkspaceScheduledIntroUnknown, WorkspaceScheduledCancel, WorkspaceScheduledButton string
	WorkspaceDeletedSubject, WorkspaceDeletedIntro                                                                                         string
}

var deletionCopies = map[string]deletionCopy{
	"en": {
		Greeting: "Hi",
		Backups:  "Copies of deleted data stay in our backups until those backups expire; they are not restored into the app.",

		WorkspaceScheduledSubject:      "The workspace %s will be deleted on %s",
		WorkspaceScheduledIntro:        "%s scheduled the workspace “%s” for deletion on %s. Its accounts, transactions, categories, budgets and subscriptions will be erased for every member, and connected apps will lose access to it.",
		WorkspaceScheduledIntroUnknown: "The workspace “%s” is scheduled for deletion on %s. Its accounts, transactions, categories, budgets and subscriptions will be erased for every member, and connected apps will lose access to it.",
		WorkspaceScheduledCancel:       "Until then, any owner can cancel the deletion in the workspace settings. Export the data from Settings if you want to keep a copy.",
		WorkspaceScheduledButton:       "Open workspace settings",

		WorkspaceDeletedSubject: "The workspace %s was deleted",
		WorkspaceDeletedIntro:   "The workspace “%s” and all its data were deleted from Finance Wingman.",
	},
	"es": {
		Greeting: "Hola",
		Backups:  "Las copias de los datos eliminados permanecen en nuestras copias de seguridad hasta que estas caducan; no se restauran en la aplicación.",

		WorkspaceScheduledSubject:      "El espacio %s se eliminará el %s",
		WorkspaceScheduledIntro:        "%s programó la eliminación del espacio «%s» para el %s. Sus cuentas, transacciones, categorías, presupuestos y suscripciones se borrarán para todos los miembros, y las aplicaciones conectadas perderán el acceso.",
		WorkspaceScheduledIntroUnknown: "El espacio «%s» se eliminará el %s. Sus cuentas, transacciones, categorías, presupuestos y suscripciones se borrarán para todos los miembros, y las aplicaciones conectadas perderán el acceso.",
		WorkspaceScheduledCancel:       "Hasta entonces, cualquier propietario puede cancelar la eliminación en la configuración del espacio. Exporta los datos desde Configuración si quieres conservar una copia.",
		WorkspaceScheduledButton:       "Abrir configuración del espacio",

		WorkspaceDeletedSubject: "El espacio %s fue eliminado",
		WorkspaceDeletedIntro:   "El espacio «%s» y todos sus datos se eliminaron de Finance Wingman.",
	},
}

// WorkspaceDeletionEmail tells a member about a workspace deletion.
type WorkspaceDeletionEmail struct {
	To, Name, Locale string
	WorkspaceName    string
	// RequestedBy is who scheduled the deletion; empty when unknown.
	RequestedBy string
	// Date is when the deletion is scheduled for (scheduled emails only).
	Date time.Time
	Link string
}

// RenderWorkspaceDeletionScheduled warns a member that the workspace will
// be deleted after the grace period.
func RenderWorkspaceDeletionScheduled(e WorkspaceDeletionEmail) (Message, error) {
	locale := pickLocale(e.Locale)
	c := deletionCopies[locale]
	date := formatDate(e.Date, locale)
	intro := fmt.Sprintf(c.WorkspaceScheduledIntroUnknown, e.WorkspaceName, date)
	if e.RequestedBy != "" {
		intro = fmt.Sprintf(c.WorkspaceScheduledIntro, e.RequestedBy, e.WorkspaceName, date)
	}
	return renderNotice(notice{
		To: e.To, Subject: fmt.Sprintf(c.WorkspaceScheduledSubject, e.WorkspaceName, date), Greeting: c.Greeting, Name: e.Name,
		Paragraphs: []string{intro, c.WorkspaceScheduledCancel, c.Backups},
		Button:     c.WorkspaceScheduledButton, Link: e.Link,
	})
}

// RenderWorkspaceDeleted tells a member that the workspace was deleted.
func RenderWorkspaceDeleted(e WorkspaceDeletionEmail) (Message, error) {
	locale := pickLocale(e.Locale)
	c := deletionCopies[locale]
	return renderNotice(notice{
		To: e.To, Subject: fmt.Sprintf(c.WorkspaceDeletedSubject, e.WorkspaceName), Greeting: c.Greeting, Name: e.Name,
		Paragraphs: []string{fmt.Sprintf(c.WorkspaceDeletedIntro, e.WorkspaceName), c.Backups},
	})
}
