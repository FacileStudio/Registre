package i18n

// spaceInvite is the notice to a person who has an account and was invited
// into a space. It carries no text anyone typed, neither the inviter's name
// nor the space's: the person reads who invited them, to what and as what on
// their spaces page, signed in, where the words cannot be passed off as
// Facile's own.
func spaceInvite() map[string]mailText {
	return map[string]mailText{
		"en": {"You have a new invitation on Facile", []string{
			"Someone invited you to join a shared space on Facile.",
			"Open your spaces page to see who sent it and to accept or decline. Nothing changes until you accept; the invitation expires in 14 days.",
		}, "Open my spaces"},
		"fr": {"Vous avez une nouvelle invitation sur Facile", []string{
			"Quelqu'un vous invite à rejoindre un espace partagé sur Facile.",
			"Ouvrez la page de vos espaces pour voir qui l'a envoyée, puis acceptez ou refusez. Rien ne change tant que vous n'acceptez pas ; l'invitation expire dans 14 jours.",
		}, "Ouvrir mes espaces"},
		"es": {"Tienes una nueva invitación en Facile", []string{
			"Alguien te invitó a unirte a un espacio compartido de Facile.",
			"Abre la página de tus espacios para ver quién la envió y aceptarla o rechazarla. Nada cambia hasta que aceptes; la invitación caduca en 14 días.",
		}, "Abrir mis espacios"},
		"de": {"Du hast eine neue Einladung auf Facile", []string{
			"Jemand hat dich eingeladen, einem geteilten Bereich auf Facile beizutreten.",
			"Öffne deine Bereichsseite, um zu sehen, wer sie gesendet hat, und sie anzunehmen oder abzulehnen. Nichts ändert sich, bis du annimmst; die Einladung läuft in 14 Tagen ab.",
		}, "Meine Bereiche öffnen"},
	}
}

// spaceJoin is the notice to an address with no account, sent only while
// public sign-up is open. It carries no text anyone typed, neither the
// inviter's name nor the space's, so it cannot be used to mail strangers a
// message of the sender's choosing.
func spaceJoin() map[string]mailText {
	return map[string]mailText{
		"en": {"You are invited to join Facile", []string{
			"Someone invited you to work together in a shared space on Facile.",
			"Create your Facile account with this address, then accept the invitation from your spaces page. It expires in 14 days. If you do not know this person, ignore this mail.",
		}, "Create my Facile account"},
		"fr": {"Vous êtes invité à rejoindre Facile", []string{
			"Quelqu'un vous invite à travailler ensemble dans un espace partagé sur Facile.",
			"Créez votre compte Facile avec cette adresse, puis acceptez l'invitation depuis la page de vos espaces. Elle expire dans 14 jours. Si vous ne connaissez pas cette personne, ignorez ce message.",
		}, "Créer mon compte Facile"},
		"es": {"Te han invitado a unirte a Facile", []string{
			"Alguien te invitó a trabajar juntos en un espacio compartido de Facile.",
			"Crea tu cuenta Facile con esta dirección y acepta la invitación desde la página de tus espacios. Caduca en 14 días. Si no conoces a esta persona, ignora este correo.",
		}, "Crear mi cuenta Facile"},
		"de": {"Du wurdest zu Facile eingeladen", []string{
			"Jemand hat dich eingeladen, in einem geteilten Bereich auf Facile zusammenzuarbeiten.",
			"Erstelle dein Facile-Konto mit dieser Adresse und nimm die Einladung dann auf deiner Bereichsseite an. Sie läuft in 14 Tagen ab. Wenn du diese Person nicht kennst, ignoriere diese E-Mail.",
		}, "Mein Facile-Konto erstellen"},
	}
}
