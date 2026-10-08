package mail

import (
	"fmt"
	"time"
)

type accountDeletionCopy struct {
	AccountScheduledSubject, AccountScheduledIntro, AccountScheduledWorkspaces, AccountScheduledCancel, AccountScheduledButton string
	AccountDeletedSubject, AccountDeletedIntro, AccountDeletedWorkspaces                                                       string
	AccountBlockedSubject, AccountBlockedIntro, AccountBlockedHow, AccountBlockedButton                                        string
}

var accountDeletionCopies = map[string]accountDeletionCopy{
	"en": {
		AccountScheduledSubject:    "Your Finance Wingman account will be deleted on %s",
		AccountScheduledIntro:      "Your Finance Wingman account is scheduled for deletion on %s. You'll leave every workspace you belong to, your sessions will end and connected apps will be disconnected.",
		AccountScheduledWorkspaces: "You're the only member of %s, so it will be deleted with all its data. Export it from Settings first if you want to keep a copy.",
		AccountScheduledCancel:     "Changed your mind? Sign in before then and cancel the deletion in Settings → Security.",
		AccountScheduledButton:     "Review the deletion",
		AccountDeletedSubject:      "Your Finance Wingman account was deleted",
		AccountDeletedIntro:        "Your Finance Wingman account has been deleted: you left every workspace, your sessions ended and connected apps were disconnected.",
		AccountDeletedWorkspaces:   "These workspaces, where you were the only member, were deleted with all their data: %s.",
		AccountBlockedSubject:      "We couldn't delete your Finance Wingman account",
		AccountBlockedIntro:        "Your account was scheduled for deletion, but you're the only owner of %s, which has other members. A workspace always needs an owner, so the deletion was cancelled and nothing was deleted.",
		AccountBlockedHow:          "Make another member an owner, or delete the workspace, and then delete your account again.",
		AccountBlockedButton:       "Open settings",
	},
	"es": {
		AccountScheduledSubject:    "Tu cuenta de Finance Wingman se eliminará el %s",
		AccountScheduledIntro:      "Tu cuenta de Finance Wingman se eliminará el %s. Saldrás de todos tus espacios, tus sesiones se cerrarán y las aplicaciones conectadas se desconectarán.",
		AccountScheduledWorkspaces: "Eres el único miembro de %s, así que se eliminará con todos sus datos. Expórtalo antes desde Configuración si quieres conservar una copia.",
		AccountScheduledCancel:     "¿Cambiaste de opinión? Entra antes de esa fecha y cancela la eliminación en Configuración → Seguridad.",
		AccountScheduledButton:     "Revisar la eliminación",
		AccountDeletedSubject:      "Tu cuenta de Finance Wingman fue eliminada",
		AccountDeletedIntro:        "Tu cuenta de Finance Wingman fue eliminada: saliste de todos tus espacios, tus sesiones se cerraron y las aplicaciones conectadas se desconectaron.",
		AccountDeletedWorkspaces:   "Estos espacios, donde eras el único miembro, se eliminaron con todos sus datos: %s.",
		AccountBlockedSubject:      "No pudimos eliminar tu cuenta de Finance Wingman",
		AccountBlockedIntro:        "Tu cuenta estaba programada para eliminarse, pero eres el único propietario de %s, que tiene otros miembros. Un espacio siempre necesita un propietario, así que la eliminación se canceló y no se borró nada.",
		AccountBlockedHow:          "Haz propietario a otro miembro, o elimina el espacio, y luego vuelve a eliminar tu cuenta.",
		AccountBlockedButton:       "Abrir configuración",
	},
}

// AccountDeletionEmail describes a user's account deletion.
type AccountDeletionEmail struct {
	To, Name, Locale string
	// Date is when the deletion is scheduled for (scheduled emails only).
	Date time.Time
	// Workspaces are the workspaces deleted along with the account (the
	// user is their only member), or the ones blocking the deletion.
	Workspaces []string
	// Link points to the settings page where the deletion is managed.
	Link string
}

// RenderAccountDeletionScheduled confirms that the account will be deleted
// after the grace period and how to cancel.
func RenderAccountDeletionScheduled(e AccountDeletionEmail) (Message, error) {
	locale := pickLocale(e.Locale)
	c, common := accountDeletionCopies[locale], deletionCopies[locale]
	date := formatDate(e.Date, locale)
	paragraphs := []string{fmt.Sprintf(c.AccountScheduledIntro, date)}
	if len(e.Workspaces) > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf(c.AccountScheduledWorkspaces, quoteList(e.Workspaces, locale)))
	}
	paragraphs = append(paragraphs, c.AccountScheduledCancel, common.Backups)
	return renderNotice(notice{
		To: e.To, Subject: fmt.Sprintf(c.AccountScheduledSubject, date), Greeting: common.Greeting, Name: e.Name,
		Paragraphs: paragraphs, Button: c.AccountScheduledButton, Link: e.Link,
	})
}

// RenderAccountDeleted confirms that the account was deleted.
func RenderAccountDeleted(e AccountDeletionEmail) (Message, error) {
	locale := pickLocale(e.Locale)
	c, common := accountDeletionCopies[locale], deletionCopies[locale]
	paragraphs := []string{c.AccountDeletedIntro}
	if len(e.Workspaces) > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf(c.AccountDeletedWorkspaces, quoteList(e.Workspaces, locale)))
	}
	paragraphs = append(paragraphs, common.Backups)
	return renderNotice(notice{To: e.To, Subject: c.AccountDeletedSubject, Greeting: common.Greeting, Name: e.Name, Paragraphs: paragraphs})
}

// RenderAccountDeletionBlocked explains that a scheduled account deletion
// was cancelled because the user became the last owner of a workspace with
// other members.
func RenderAccountDeletionBlocked(e AccountDeletionEmail) (Message, error) {
	locale := pickLocale(e.Locale)
	c, common := accountDeletionCopies[locale], deletionCopies[locale]
	return renderNotice(notice{
		To: e.To, Subject: c.AccountBlockedSubject, Greeting: common.Greeting, Name: e.Name,
		Paragraphs: []string{fmt.Sprintf(c.AccountBlockedIntro, quoteList(e.Workspaces, locale)), c.AccountBlockedHow},
		Button:     c.AccountBlockedButton, Link: e.Link,
	})
}
