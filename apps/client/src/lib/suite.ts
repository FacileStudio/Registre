import { icons } from '@facile/muse';

/**
 * SuiteApp is one destination on the signed-in home page. An app with its own
 * brand glyph names it in `logo`, a file under static/logos copied from the
 * app's repository; the few with no glyph of their own get a muse `icon`
 * instead, never a favicon fetched from their domain.
 */
export interface SuiteApp {
	name: string;
	url: string;
	description: string;
	logo?: string;
	icon?: string;
}

/** SuiteGroup is one titled block of apps. */
export interface SuiteGroup {
	title: string;
	apps: SuiteApp[];
}

/** logo is the path of an app's brand glyph. */
const logo = (slug: string) => `/logos/${slug}.svg`;

/** suite is every app the home page links to, in display order. */
export const suite: SuiteGroup[] = [
	{
		title: 'Suite',
		apps: [
			{ name: 'Opus', url: 'https://opus.facile.studio', description: 'Gestion de projet, simple et performante.', logo: logo('opus') },
			{ name: 'Courrier', url: 'https://courrier.facile.studio', description: 'Client email du studio.', logo: logo('courrier') },
			{ name: 'Agenda', url: 'https://agenda.facile.studio', description: 'Calendrier partagé, rendez-vous et deadlines.', logo: logo('agenda') },
			{ name: 'Nuage', url: 'https://nuage.facile.studio', description: "Fichiers partagés et espace cloud de l'équipe.", logo: logo('nuage') },
			{ name: 'Sablier', url: 'https://sablier.facile.studio', description: 'Suivi du temps de travail par projet.', logo: logo('sablier') },
			{ name: 'Plume', url: 'https://plume.facile.studio', description: 'Signature électronique, simple et moderne.', logo: logo('plume') },
			{ name: 'Sonar', url: 'https://sonar.facile.studio', description: 'Moteur de recherche web privé et rapide.', logo: logo('sonar') },
			{ name: 'Bulle', url: 'https://bulle.facile.studio', description: 'Assistant du studio, branché sur toute la suite.', logo: logo('bulle') },
			{ name: 'Vision', url: 'https://vision.facile.studio', description: 'Analytics web avec suivi en temps réel.', logo: logo('vision') },
			{ name: 'Perception', url: 'https://perception.facile.studio', description: 'Mémoire métier et intelligence contextuelle.', logo: logo('perception') },
			{ name: 'Scribe', url: 'https://scribe.facile.studio', description: 'Transcription de réunions, en temps réel et en local.', logo: logo('scribe') },
			{ name: 'Echo', url: 'https://echo.facile.studio', description: 'Visioconférence simple et sécurisée.', logo: logo('echo') },
			{ name: 'Place', url: 'https://place.facile.studio', description: 'Forum de la communauté et échanges de l’équipe.', logo: logo('place') },
			{ name: 'Casier', url: 'https://casier.facile.studio', description: "Secrets et variables d'environnement.", logo: logo('casier') },
			{ name: 'Capsule', url: 'https://capsule.facile.studio', description: 'Partage de secrets chiffrés de bout en bout.', logo: logo('capsule') },
			{ name: 'Journal', url: 'https://journal.facile.studio', description: 'Logs centralisés de la suite.', logo: logo('journal') },
			{ name: 'Mycelium', url: 'https://mycelium.facile.studio', description: "Mémoire persistante et activité des agents.", logo: logo('mycelium') }
		]
	},
	{
		title: 'Workspace',
		apps: [
			{ name: 'Docs', url: 'https://docs.facile.studio', description: 'Documents collaboratifs, propulsé par Outline.', icon: icons.text },
			{ name: 'Ardoise', url: 'https://ardoise.facile.studio', description: 'Factures, devis et contrats freelance.', logo: logo('ardoise') }
		]
	},
	{
		title: 'Infrastructure',
		apps: [
			{ name: 'Sonde', url: 'https://sonde.facile.studio', description: "Monitoring d'uptime et pages de statut.", logo: logo('sonde') },
			{ name: 'Antenne', url: 'https://antenne.facile.studio', description: "Bus d'alertes et de notifications.", logo: logo('antenne') }
		]
	}
];

/**
 * filterSuite keeps the apps whose name, description or group title contains
 * query, dropping groups left empty. A blank query keeps everything.
 */
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
