package mail

import (
	"fmt"
	"time"
)

type securityCopy struct {
	AppConnectedSubject, AppConnectedIntro, AppConnectedAccess, AppConnectedWorkspace, AppConnectedDate, AppConnectedWarning, AppConnectedButton string
	AppUnnamed, AccessRead, AccessWrite                                                                                                          string

	MemberJoinedSubject, MemberJoinedIntro, MemberJoinedRole, MemberJoinedWarning, MemberJoinedButton string
	RoleOwner, RoleMember                                                                             string
}

var securityCopies = map[string]securityCopy{
	"en": {
		AppConnectedSubject:   "%s was connected to your Finance Wingman account",
		AppConnectedIntro:     "The app “%s” was connected to your Finance Wingman account.",
		AppConnectedAccess:    "Access: %s.",
		AppConnectedWorkspace: "Workspace: %s.",
		AppConnectedDate:      "Date: %s.",
		AppConnectedWarning:   "If you didn't authorize it, disconnect the app now in Settings → Security.",
		AppConnectedButton:    "Review connected apps",
		AppUnnamed:            "An unnamed app",
		AccessRead:            "Read only",
		AccessWrite:           "Read and write",

		MemberJoinedSubject: "%s joined %s",
		MemberJoinedIntro:   "%s accepted an invitation and joined the workspace “%s”.",
		MemberJoinedRole:    "Role: %s.",
		MemberJoinedWarning: "You're receiving this because you're an owner of the workspace. If you don't recognize this person, remove them in the workspace settings.",
		MemberJoinedButton:  "Open workspace settings",
		RoleOwner:           "Owner",
		RoleMember:          "Member",
	},
	"es": {
		AppConnectedSubject:   "%s se conectó a tu cuenta de Finance Wingman",
		AppConnectedIntro:     "La aplicación «%s» se conectó a tu cuenta de Finance Wingman.",
		AppConnectedAccess:    "Acceso: %s.",
		AppConnectedWorkspace: "Espacio: %s.",
		AppConnectedDate:      "Fecha: %s.",
		AppConnectedWarning:   "Si no la autorizaste, desconéctala ahora en Configuración → Seguridad.",
		AppConnectedButton:    "Revisar aplicaciones conectadas",
		AppUnnamed:            "Una aplicación sin nombre",
		AccessRead:            "Solo lectura",
		AccessWrite:           "Lectura y escritura",

		MemberJoinedSubject: "%s se unió a %s",
		MemberJoinedIntro:   "%s aceptó una invitación y se unió al espacio «%s».",
		MemberJoinedRole:    "Rol: %s.",
		MemberJoinedWarning: "Recibes este aviso porque eres propietario del espacio. Si no reconoces a esta persona, quítala en la configuración del espacio.",
		MemberJoinedButton:  "Abrir configuración del espacio",
		RoleOwner:           "Propietario",
		RoleMember:          "Miembro",
	},
}

// AppConnectedEmail tells a user that an AI app was connected to their
// account through OAuth.
type AppConnectedEmail struct {
	To, Name, Locale string
	// AppName is the name the app registered with; may be empty.
	AppName string
	// CanWrite is true when the app may record and change data.
	CanWrite      bool
	WorkspaceName string
	// Date is when the app was connected, in the display time zone.
	Date time.Time
	// Link points to the page where connected apps are managed.
	Link string
}

// RenderAppConnected warns the user that an app was connected.
func RenderAppConnected(e AppConnectedEmail) (Message, error) {
	locale := pickLocale(e.Locale)
	c, common := securityCopies[locale], deletionCopies[locale]
	app := e.AppName
	if app == "" {
		app = c.AppUnnamed
	}
	access := c.AccessRead
	if e.CanWrite {
		access = c.AccessWrite
	}
	paragraphs := []string{fmt.Sprintf(c.AppConnectedIntro, app), fmt.Sprintf(c.AppConnectedAccess, access)}
	if e.WorkspaceName != "" {
		paragraphs = append(paragraphs, fmt.Sprintf(c.AppConnectedWorkspace, e.WorkspaceName))
	}
	paragraphs = append(paragraphs, fmt.Sprintf(c.AppConnectedDate, formatDate(e.Date, locale)), c.AppConnectedWarning)
	return renderNotice(notice{
		To: e.To, Subject: fmt.Sprintf(c.AppConnectedSubject, app), Greeting: common.Greeting, Name: e.Name,
		Paragraphs: paragraphs, Button: c.AppConnectedButton, Link: e.Link,
	})
}

// MemberJoinedEmail tells a workspace owner that someone joined.
type MemberJoinedEmail struct {
	To, Name, Locale string
	// Member is the display name (or email) of who joined, and MemberEmail
	// their address.
	Member, MemberEmail string
	Role                string
	WorkspaceName       string
	Link                string
}

// RenderMemberJoined tells an owner that an invitation was accepted.
func RenderMemberJoined(e MemberJoinedEmail) (Message, error) {
	locale := pickLocale(e.Locale)
	c, common := securityCopies[locale], deletionCopies[locale]
	who := e.Member
	if e.MemberEmail != "" && e.MemberEmail != e.Member {
		who = fmt.Sprintf("%s (%s)", e.Member, e.MemberEmail)
	}
	role := c.RoleMember
	if e.Role == "owner" {
		role = c.RoleOwner
	}
	return renderNotice(notice{
		To: e.To, Subject: fmt.Sprintf(c.MemberJoinedSubject, e.Member, e.WorkspaceName), Greeting: common.Greeting, Name: e.Name,
		Paragraphs: []string{
			fmt.Sprintf(c.MemberJoinedIntro, who, e.WorkspaceName),
			fmt.Sprintf(c.MemberJoinedRole, role),
			c.MemberJoinedWarning,
		},
		Button: c.MemberJoinedButton, Link: e.Link,
	})
}
