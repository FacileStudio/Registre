package i18n

// spaceInvite is the notice to a person who has an account and was invited
// into a space. vars: inviter, space, role.
func spaceInvite() map[string]mailText {
	return map[string]mailText{
		"en": {"You are invited to a Facile space", []string{
			"{inviter} invited you to join the space \"{space}\" as {role}.",
			"Nothing changes until you accept. Accept or decline from your spaces page; the invitation expires in 14 days.",
		}, "Open my spaces"},
		"fr": {"Vous êtes invité dans un espace Facile", []string{
			"{inviter} vous invite à rejoindre l'espace « {space} » en tant que {role}.",
			"Rien ne change tant que vous n'acceptez pas. Acceptez ou refusez depuis la page de vos espaces ; l'invitation expire dans 14 jours.",
		}, "Ouvrir mes espaces"},
		"es": {"Te han invitado a un espacio Facile", []string{
			"{inviter} te invitó a unirte al espacio «{space}» como {role}.",
			"Nada cambia hasta que aceptes. Acepta o rechaza desde la página de tus espacios; la invitación caduca en 14 días.",
		}, "Abrir mis espacios"},
		"de": {"Du wurdest in einen Facile-Bereich eingeladen", []string{
			"{inviter} hat dich eingeladen, dem Bereich „{space}“ als {role} beizutreten.",
			"Nichts ändert sich, bis du annimmst. Nimm die Einladung auf deiner Bereichsseite an oder lehne sie ab; sie läuft in 14 Tagen ab.",
		}, "Meine Bereiche öffnen"},
	}
}

// spaceJoin is the notice to an address with no account, sent only while
// public sign-up is open. vars: inviter, space.
func spaceJoin() map[string]mailText {
	return map[string]mailText{
		"en": {"You are invited to join Facile", []string{
			"{inviter} invited you to work together in the space \"{space}\" on Facile.",
			"Create your Facile account with this address, then accept the invitation from your spaces page. It expires in 14 days. If you do not know this person, ignore this mail.",
		}, "Create my Facile account"},
		"fr": {"Vous êtes invité à rejoindre Facile", []string{
			"{inviter} vous invite à travailler ensemble dans l'espace « {space} » sur Facile.",
			"Créez votre compte Facile avec cette adresse, puis acceptez l'invitation depuis la page de vos espaces. Elle expire dans 14 jours. Si vous ne connaissez pas cette personne, ignorez ce message.",
		}, "Créer mon compte Facile"},
		"es": {"Te han invitado a unirte a Facile", []string{
			"{inviter} te invitó a trabajar juntos en el espacio «{space}» de Facile.",
			"Crea tu cuenta Facile con esta dirección y acepta la invitación desde la página de tus espacios. Caduca en 14 días. Si no conoces a esta persona, ignora este correo.",
		}, "Crear mi cuenta Facile"},
		"de": {"Du wurdest zu Facile eingeladen", []string{
			"{inviter} hat dich eingeladen, im Bereich „{space}“ auf Facile zusammenzuarbeiten.",
			"Erstelle dein Facile-Konto mit dieser Adresse und nimm die Einladung dann auf deiner Bereichsseite an. Sie läuft in 14 Tagen ab. Wenn du diese Person nicht kennst, ignoriere diese E-Mail.",
		}, "Mein Facile-Konto erstellen"},
	}
}
