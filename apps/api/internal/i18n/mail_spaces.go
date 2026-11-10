package i18n

import (
	"strconv"
	"time"
)

// spaceInvite is the notice to a person who has an account and was invited
// into a space. Like GitHub's, it names who invited them ({inviter}), to
// which space ({space}) and as what ({role}), and links to the invitation
// itself. Every one of those values is something a person typed, so Mail
// cleans and escapes them, the space name sits in quotation marks, and the
// per-recipient and per-inviter caps bound how often anyone can send one.
func spaceInvite() map[string]mailText {
	return map[string]mailText{
		"en": {"{inviter} invited you to join {space} on Facile", []string{
			"{inviter} invited you to join the space “{space}” on Facile as {role}.",
			"Open the invitation to accept or decline it. Nothing changes until you accept. The invitation expires on {date}.",
			"If you do not know this person, you can ignore this mail.",
		}, "View invitation"},
		"fr": {"{inviter} vous invite à rejoindre {space} sur Facile", []string{
			"{inviter} vous invite à rejoindre l'espace « {space} » sur Facile en tant que {role}.",
			"Ouvrez l'invitation pour l'accepter ou la refuser. Rien ne change tant que vous n'acceptez pas. L'invitation expire le {date}.",
			"Si vous ne connaissez pas cette personne, vous pouvez ignorer ce message.",
		}, "Voir l'invitation"},
		"es": {"{inviter} te invitó a unirte a {space} en Facile", []string{
			"{inviter} te invitó a unirte al espacio «{space}» de Facile como {role}.",
			"Abre la invitación para aceptarla o rechazarla. Nada cambia hasta que aceptes. La invitación caduca el {date}.",
			"Si no conoces a esta persona, puedes ignorar este correo.",
		}, "Ver la invitación"},
		"de": {"{inviter} hat dich zu {space} auf Facile eingeladen", []string{
			"{inviter} hat dich eingeladen, dem Bereich „{space}“ auf Facile als {role} beizutreten.",
			"Öffne die Einladung, um sie anzunehmen oder abzulehnen. Nichts ändert sich, bis du annimmst. Die Einladung läuft am {date} ab.",
			"Wenn du diese Person nicht kennst, kannst du diese E-Mail ignorieren.",
		}, "Einladung ansehen"},
	}
}

// spaceJoin is the notice to an address with no account, sent only while
// public sign-up is open. It carries the same names as spaceInvite, under
// the same cleaning and caps, and its link opens sign-up with the
// invitation attached, so the new account lands on it.
func spaceJoin() map[string]mailText {
	return map[string]mailText{
		"en": {"{inviter} invited you to join {space} on Facile", []string{
			"{inviter} invited you to join the space “{space}” on Facile as {role}.",
			"Create your Facile account with this address to see the invitation and accept it. It expires on {date}.",
			"If you do not know this person, ignore this mail: no account is created.",
		}, "Create my account and view the invitation"},
		"fr": {"{inviter} vous invite à rejoindre {space} sur Facile", []string{
			"{inviter} vous invite à rejoindre l'espace « {space} » sur Facile en tant que {role}.",
			"Créez votre compte Facile avec cette adresse pour voir l'invitation et l'accepter. Elle expire le {date}.",
			"Si vous ne connaissez pas cette personne, ignorez ce message : aucun compte n'est créé.",
		}, "Créer mon compte et voir l'invitation"},
		"es": {"{inviter} te invitó a unirte a {space} en Facile", []string{
			"{inviter} te invitó a unirte al espacio «{space}» de Facile como {role}.",
			"Crea tu cuenta Facile con esta dirección para ver la invitación y aceptarla. Caduca el {date}.",
			"Si no conoces a esta persona, ignora este correo: no se crea ninguna cuenta.",
		}, "Crear mi cuenta y ver la invitación"},
		"de": {"{inviter} hat dich zu {space} auf Facile eingeladen", []string{
			"{inviter} hat dich eingeladen, dem Bereich „{space}“ auf Facile als {role} beizutreten.",
			"Erstelle dein Facile-Konto mit dieser Adresse, um die Einladung zu sehen und anzunehmen. Sie läuft am {date} ab.",
			"Wenn du diese Person nicht kennst, ignoriere diese E-Mail: Es wird kein Konto erstellt.",
		}, "Konto erstellen und Einladung ansehen"},
	}
}

// roleNames is each space role in each language, as a mail sentence uses it.
func roleNames() map[string]map[string]string {
	return map[string]map[string]string{
		"en": {"owner": "owner", "admin": "admin", "member": "member"},
		"fr": {"owner": "propriétaire", "admin": "administrateur", "member": "membre"},
		"es": {"owner": "propietario", "admin": "administrador", "member": "miembro"},
		"de": {"owner": "Inhaber", "admin": "Admin", "member": "Mitglied"},
	}
}

// RoleName is role in locale's words, or role itself when it is unknown.
func RoleName(role, locale string) string {
	if name, ok := roleNames()[Normalize(locale)][role]; ok {
		return name
	}
	return role
}

// monthNames is each month in each language, January first.
func monthNames() map[string][12]string {
	return map[string][12]string{
		"en": {"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
		"fr": {"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
		"es": {"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"},
		"de": {"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
	}
}

// Date is day in locale's long form, in UTC: "24 November 2026",
// "24 novembre 2026", "24 de noviembre de 2026", "24. November 2026".
func Date(day time.Time, locale string) string {
	day = day.UTC()
	lang := Normalize(locale)
	month := monthNames()[lang][day.Month()-1]
	d, y := strconv.Itoa(day.Day()), strconv.Itoa(day.Year())
	switch lang {
	case "es":
		return d + " de " + month + " de " + y
	case "de":
		return d + ". " + month + " " + y
	}
	return d + " " + month + " " + y
}
