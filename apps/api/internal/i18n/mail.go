package i18n

import (
	"html"
	"strings"
)

// Mail kinds. Each one exists in every supported locale; mailCatalog's test
// fails the build otherwise.
const (
	MailSignupVerify      = "signup.verify"
	MailSignupExisting    = "signup.existing"
	MailRecovery          = "recovery.reset"
	MailDeletionScheduled = "deletion.scheduled"
	MailDeletionDone      = "deletion.done"
)

// mailText is one mail in one language: a subject, paragraphs, and the label
// of the one link it carries. A "{name}" in a paragraph is replaced at
// render by vars["name"].
type mailText struct {
	subject    string
	paragraphs []string
	action     string
}

// Mail renders kind in locale around link, falling back to English for an
// unknown locale. Every piece of text is escaped after substitution, so a
// space name or a person's name in vars can never become markup.
func Mail(kind, locale, link string, vars map[string]string) (subject, body string) {
	byLocale := mailCatalog()[kind]
	text, ok := byLocale[Normalize(locale)]
	if !ok {
		text = byLocale[Fallback]
	}
	var out strings.Builder
	for _, paragraph := range text.paragraphs {
		for key, value := range vars {
			paragraph = strings.ReplaceAll(paragraph, "{"+key+"}", value)
		}
		out.WriteString("<p>" + html.EscapeString(paragraph) + "</p>")
	}
	if link != "" {
		out.WriteString(`<p><a href="` + html.EscapeString(link) + `">` + html.EscapeString(text.action) + `</a></p>`)
	}
	out.WriteString("<p>Facile</p>")
	return text.subject, out.String()
}

// mailCatalog is every mail in every language. It is a function rather than
// a package variable so nothing can mutate it at runtime.
func mailCatalog() map[string]map[string]mailText {
	return map[string]map[string]mailText{
		MailSignupVerify:      signupVerify(),
		MailSignupExisting:    signupExisting(),
		MailRecovery:          recoveryReset(),
		MailDeletionScheduled: deletionScheduled(),
		MailDeletionDone:      deletionDone(),
	}
}

func signupVerify() map[string]mailText {
	return map[string]mailText{
		"en": {"Create your Facile account", []string{
			"Someone asked to create a Facile account with this address. Follow the link to choose your name and how you sign in.",
			"The link works once and stops working in 30 minutes. If it was not you, ignore this mail: no account is created.",
		}, "Create my Facile account"},
		"fr": {"Créez votre compte Facile", []string{
			"Quelqu'un a demandé à créer un compte Facile avec cette adresse. Suivez le lien pour choisir votre nom et votre façon de vous connecter.",
			"Le lien ne fonctionne qu'une fois et expire dans 30 minutes. Si ce n'était pas vous, ignorez ce message : aucun compte n'est créé.",
		}, "Créer mon compte Facile"},
		"es": {"Crea tu cuenta Facile", []string{
			"Alguien pidió crear una cuenta Facile con esta dirección. Sigue el enlace para elegir tu nombre y cómo inicias sesión.",
			"El enlace funciona una sola vez y caduca en 30 minutos. Si no fuiste tú, ignora este correo: no se crea ninguna cuenta.",
		}, "Crear mi cuenta Facile"},
		"de": {"Erstelle dein Facile-Konto", []string{
			"Jemand möchte mit dieser Adresse ein Facile-Konto erstellen. Folge dem Link, um deinen Namen und deine Anmeldeart zu wählen.",
			"Der Link funktioniert einmal und läuft in 30 Minuten ab. Wenn du das nicht warst, ignoriere diese E-Mail: Es wird kein Konto erstellt.",
		}, "Mein Facile-Konto erstellen"},
	}
}

func signupExisting() map[string]mailText {
	return map[string]mailText{
		"en": {"You already have a Facile account", []string{
			"Someone tried to create a Facile account with this address, but you already have one.",
			"Sign in instead. If you forgot your password, the sign-in page lets you reset it. If it was not you, ignore this mail.",
		}, "Sign in with Facile"},
		"fr": {"Vous avez déjà un compte Facile", []string{
			"Quelqu'un a essayé de créer un compte Facile avec cette adresse, mais vous en avez déjà un.",
			"Connectez-vous plutôt. Si vous avez oublié votre mot de passe, la page de connexion permet de le réinitialiser. Si ce n'était pas vous, ignorez ce message.",
		}, "Se connecter avec Facile"},
		"es": {"Ya tienes una cuenta Facile", []string{
			"Alguien intentó crear una cuenta Facile con esta dirección, pero ya tienes una.",
			"Inicia sesión. Si olvidaste tu contraseña, la página de inicio de sesión te permite restablecerla. Si no fuiste tú, ignora este correo.",
		}, "Iniciar sesión con Facile"},
		"de": {"Du hast bereits ein Facile-Konto", []string{
			"Jemand hat versucht, mit dieser Adresse ein Facile-Konto zu erstellen, aber du hast bereits eines.",
			"Melde dich stattdessen an. Wenn du dein Passwort vergessen hast, kannst du es auf der Anmeldeseite zurücksetzen. Wenn du das nicht warst, ignoriere diese E-Mail.",
		}, "Mit Facile anmelden"},
	}
}

func recoveryReset() map[string]mailText {
	return map[string]mailText{
		"en": {"Reset your password", []string{
			"Someone asked to reset the password on this address. If it was not you, ignore this mail.",
			"The link stops working in one hour.",
		}, "Choose a new password"},
		"fr": {"Réinitialisez votre mot de passe", []string{
			"Quelqu'un a demandé à réinitialiser le mot de passe de cette adresse. Si ce n'était pas vous, ignorez ce message.",
			"Le lien expire dans une heure.",
		}, "Choisir un nouveau mot de passe"},
		"es": {"Restablece tu contraseña", []string{
			"Alguien pidió restablecer la contraseña de esta dirección. Si no fuiste tú, ignora este correo.",
			"El enlace caduca en una hora.",
		}, "Elegir una nueva contraseña"},
		"de": {"Setze dein Passwort zurück", []string{
			"Jemand möchte das Passwort für diese Adresse zurücksetzen. Wenn du das nicht warst, ignoriere diese E-Mail.",
			"Der Link läuft in einer Stunde ab.",
		}, "Neues Passwort wählen"},
	}
}

func deletionScheduled() map[string]mailText {
	return map[string]mailText{
		"en": {"Your Facile account will be deleted", []string{
			"You asked to delete your Facile account. It will be deleted on {date}, with every app signed out.",
			"Changed your mind? Cancel from your account page before that date.",
		}, "Open my account"},
		"fr": {"Votre compte Facile va être supprimé", []string{
			"Vous avez demandé la suppression de votre compte Facile. Il sera supprimé le {date}, et toutes les applications seront déconnectées.",
			"Vous avez changé d'avis ? Annulez depuis la page de votre compte avant cette date.",
		}, "Ouvrir mon compte"},
		"es": {"Tu cuenta Facile será eliminada", []string{
			"Pediste eliminar tu cuenta Facile. Se eliminará el {date} y se cerrará la sesión en todas las aplicaciones.",
			"¿Cambiaste de opinión? Cancela desde la página de tu cuenta antes de esa fecha.",
		}, "Abrir mi cuenta"},
		"de": {"Dein Facile-Konto wird gelöscht", []string{
			"Du hast die Löschung deines Facile-Kontos beantragt. Es wird am {date} gelöscht und in allen Apps abgemeldet.",
			"Meinung geändert? Brich die Löschung vor diesem Datum auf deiner Kontoseite ab.",
		}, "Mein Konto öffnen"},
	}
}

func deletionDone() map[string]mailText {
	return map[string]mailText{
		"en": {"Your Facile account was deleted", []string{
			"Your Facile account and the data Registre held about you have been deleted. Every app was told to erase what it holds within 30 days.",
		}, ""},
		"fr": {"Votre compte Facile a été supprimé", []string{
			"Votre compte Facile et les données que Registre détenait sur vous ont été supprimés. Chaque application a été prévenue d'effacer ce qu'elle détient sous 30 jours.",
		}, ""},
		"es": {"Tu cuenta Facile fue eliminada", []string{
			"Tu cuenta Facile y los datos que Registre tenía sobre ti han sido eliminados. Se avisó a cada aplicación para que borre lo que guarda en un plazo de 30 días.",
		}, ""},
		"de": {"Dein Facile-Konto wurde gelöscht", []string{
			"Dein Facile-Konto und die Daten, die Registre über dich gespeichert hatte, wurden gelöscht. Jede App wurde aufgefordert, ihre Daten innerhalb von 30 Tagen zu löschen.",
		}, ""},
	}
}
