export interface SuiteApp {
	name: string;
	url: string;
	description: string;
	logo?: string;
	icon?: string;
}

export interface SuiteGroup {
	title: string;
	apps: SuiteApp[];
}

const logo = (slug: string) => `/logos/${slug}.svg`;

export const suite: SuiteGroup[] = [
	{
		title: 'Suite',
		apps: [
			{ name: 'Opus', url: 'https://opus.facile.studio', description: 'Gestion de projet, simple et performante.', icon: 'solar:slider-vertical-minimalistic-bold-duotone', logo: logo('opus') },
			{ name: 'Courrier', url: 'https://courrier.facile.studio', description: 'Client email du studio.', icon: 'solar:letter-opened-bold-duotone', logo: logo('courrier') },
			{ name: 'Agenda', url: 'https://agenda.facile.studio', description: 'Calendrier partagé, rendez-vous et deadlines.', icon: 'solar:calendar-add-bold-duotone', logo: logo('agenda') },
			{ name: 'Nuage', url: 'https://nuage.facile.studio', description: "Fichiers partagés et espace cloud de l'équipe.", icon: 'solar:cloud-bold-duotone', logo: logo('nuage') },
			{ name: 'Sablier', url: 'https://sablier.facile.studio', description: 'Suivi du temps de travail par projet.', icon: 'solar:hourglass-bold-duotone', logo: logo('sablier') },
			{ name: 'Plume', url: 'https://plume.facile.studio', description: 'Signature électronique, simple et moderne.', icon: 'solar:pen-new-square-bold-duotone', logo: logo('plume') },
			{ name: 'Sonar', url: 'https://sonar.facile.studio', description: 'Moteur de recherche web privé et rapide.', icon: 'brand:sonar', logo: logo('sonar') },
			{ name: 'Bulle', url: 'https://bulle.facile.studio', description: 'Assistant du studio, branché sur toute la suite.', icon: 'solar:smile-circle-bold-duotone', logo: logo('bulle') },
			{ name: 'Vision', url: 'https://vision.facile.studio', description: 'Analytics web avec suivi en temps réel.', icon: 'solar:panorama-bold-duotone', logo: logo('vision') },
			{ name: 'Scribe', url: 'https://scribe.facile.studio', description: 'Transcription de réunions, en temps réel et en local.', icon: 'solar:microphone-2-bold-duotone', logo: logo('scribe') },
			{ name: 'Echo', url: 'https://echo.facile.studio', description: 'Visioconférence simple et sécurisée.', icon: 'solar:videocamera-record-bold-duotone', logo: logo('echo') },
			{ name: 'Place', url: 'https://place.facile.studio', description: 'Forum de la communauté et échanges de l’équipe.', icon: 'solar:cup-hot-bold-duotone', logo: logo('place') },
			{ name: 'Casier', url: 'https://casier.facile.studio', description: "Secrets et variables d'environnement.", icon: 'solar:key-minimalistic-square-bold-duotone', logo: logo('casier') },
			{ name: 'Capsule', url: 'https://capsule.facile.studio', description: 'Partage de secrets chiffrés de bout en bout.', icon: 'solar:pill-bold-duotone', logo: logo('capsule') },
			{ name: 'Journal', url: 'https://journal.facile.studio', description: 'Logs centralisés de la suite.', icon: 'solar:notebook-bold-duotone', logo: logo('journal') },
			{ name: 'Mycelium', url: 'https://mycelium.facile.studio', description: 'Mémoire persistante et activité des agents.', icon: 'solar:structure-bold-duotone', logo: logo('mycelium') },
			{ name: 'Prisme', url: 'https://prisme.facile.studio', description: "Messagerie d'équipe et salons connectés sur Matrix.", icon: 'solar:chat-round-bold-duotone' },
			{ name: 'Glouton', url: 'https://glouton.facile.studio', description: 'Prospection automatisée et qualification de leads.', icon: 'solar:magnet-wave-bold-duotone' }
		]
	},
	{
		title: 'Workspace',
		apps: [
			{ name: 'Docs', url: 'https://docs.facile.studio', description: 'Documents collaboratifs, édités à plusieurs en direct.', icon: 'solar:documents-bold-duotone' },
			{ name: 'Feuilles', url: 'https://feuilles.facile.studio', description: 'Tableurs collaboratifs, édités à plusieurs en direct.', icon: 'solar:table-minimalistic-bold-duotone' },
			{ name: 'Toile', url: 'https://toile.facile.studio', description: 'Tableau blanc infini et collaboratif.', icon: 'solar:tablet-bold-duotone' },
			{ name: 'Ardoise', url: 'https://ardoise.facile.studio', description: 'Factures, devis et contrats freelance.', icon: 'solar:bill-list-bold-duotone', logo: logo('ardoise') }
		]
	},
	{
		title: 'Infrastructure',
		apps: [
			{ name: 'Sonde', url: 'https://sonde.facile.studio', description: "Monitoring d'uptime et pages de statut.", icon: 'solar:radar-bold-duotone', logo: logo('sonde') },
			{ name: 'Antenne', url: 'https://antenne.facile.studio', description: "Bus d'alertes et de notifications.", icon: 'solar:station-bold-duotone', logo: logo('antenne') },
			{ name: 'Boîte', url: 'https://boite.facile.studio', description: 'Des machines jetables pour votre code et vos agents IA.', icon: 'solar:box-minimalistic-bold-duotone' }
		]
	}
];

export function filterSuite(groups: SuiteGroup[], query: string): SuiteGroup[] {
	const needle = query.trim().toLowerCase();
	if (!needle) return groups;
	return groups
		.map((group) => ({
			...group,
			apps: group.apps.filter((app) =>
				[app.name, app.description, group.title].some((text) => text.toLowerCase().includes(needle))
			)
		}))
		.filter((group) => group.apps.length > 0);
}
