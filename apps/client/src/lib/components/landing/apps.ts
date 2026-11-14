import type { WorksWithApp } from '@facile/muse';
import type { Locale } from '$lib/i18n/i18n.svelte';

interface OpenApp {
	name: string;
	icon: string;
	href: string;
	line: Record<Locale, string>;
}

/**
 * OPEN_APPS are the hosted apps any new Facile account can sign in to, as the
 * 2026-11-14 access survey found them (access policy `everyone`). Ardoise,
 * Journal and Antenne admit assigned accounts only and stay off the list;
 * Scribe, Boîte, Fusible and Pastille need no account. Each line is the app's
 * one-liner from the suite's canonical copy.
 */
const OPEN_APPS: readonly OpenApp[] = [
	{
		name: "Bulle",
		icon: "solar:smile-circle-bold-duotone",
		href: "https://bulle.facile.studio",
		line: {
			fr: "Un assistant IA qui fait le travail dans vos apps Facile.",
			en: "An AI assistant that gets the work done in your Facile apps.",
			es: "Un asistente de IA que hace el trabajo en tus apps de Facile.",
			de: "Ein KI-Assistent, der die Arbeit in Ihren Facile-Apps erledigt."
		}
	},
	{
		name: "Prisme",
		icon: "solar:chat-round-bold-duotone",
		href: "https://prisme.facile.studio",
		line: {
			fr: "La messagerie de l'équipe, un salon par projet.",
			en: "Team chat, one room per project.",
			es: "El chat del equipo, una sala por proyecto.",
			de: "Der Team-Chat, ein Raum pro Projekt."
		}
	},
	{
		name: "Courrier",
		icon: "solar:letter-opened-bold-duotone",
		href: "https://courrier.facile.studio",
		line: {
			fr: "Toutes vos adresses mail dans une seule boîte, au calme.",
			en: "All your email addresses in one calm inbox.",
			es: "Todas tus direcciones de correo en una bandeja tranquila.",
			de: "Alle Ihre E-Mail-Adressen in einem ruhigen Postfach."
		}
	},
	{
		name: "Agenda",
		icon: "solar:calendar-add-bold-duotone",
		href: "https://agenda.facile.studio",
		line: {
			fr: "L'agenda partagé de toute l'équipe, sur tous vos appareils.",
			en: "One shared calendar for the whole team, on every device.",
			es: "La agenda compartida de todo el equipo, en todos tus dispositivos.",
			de: "Der gemeinsame Kalender des Teams, auf jedem Gerät."
		}
	},
	{
		name: "Echo",
		icon: "solar:videocamera-record-bold-duotone",
		href: "https://echo.facile.studio",
		line: {
			fr: "Un appel vidéo en un lien, rien à installer.",
			en: "Video calls from a link, nothing to install.",
			es: "Videollamadas con un enlace, sin instalar nada.",
			de: "Videoanrufe per Link, ohne Installation."
		}
	},
	{
		name: "Feuilles",
		icon: "solar:table-minimalistic-bold-duotone",
		href: "https://feuilles.facile.studio",
		line: {
			fr: "Des tableurs à plusieurs, vos fichiers Excel compris.",
			en: "Spreadsheets you edit together, Excel files included.",
			es: "Hojas de cálculo en equipo, con tus archivos de Excel.",
			de: "Tabellen im Team, Ihre Excel-Dateien inklusive."
		}
	},
	{
		name: "Docs",
		icon: "solar:documents-bold-duotone",
		href: "https://docs.facile.studio",
		line: {
			fr: "Des documents écrits à plusieurs, en direct.",
			en: "Documents you write together, live.",
			es: "Documentos escritos en equipo, en directo.",
			de: "Dokumente gemeinsam schreiben, live."
		}
	},
	{
		name: "Toile",
		icon: "solar:tablet-bold-duotone",
		href: "https://toile.facile.studio",
		line: {
			fr: "Un tableau blanc infini à partager avec l'équipe.",
			en: "An endless whiteboard to share with your team.",
			es: "Una pizarra infinita para compartir con tu equipo.",
			de: "Ein endloses Whiteboard für Ihr ganzes Team."
		}
	},
	{
		name: "Nuage",
		icon: "solar:cloud-bold-duotone",
		href: "https://nuage.facile.studio",
		line: {
			fr: "Vos fichiers partout, partagés en un lien.",
			en: "Your files everywhere, shared with a link.",
			es: "Tus archivos en todas partes, compartidos con un enlace.",
			de: "Ihre Dateien überall, geteilt mit einem Link."
		}
	},
	{
		name: "Opus",
		icon: "solar:slider-vertical-minimalistic-bold-duotone",
		href: "https://opus.facile.studio",
		line: {
			fr: "Les projets de l'équipe, clairs et suivis en direct.",
			en: "Your team's projects, clear and followed live.",
			es: "Los proyectos del equipo, claros y seguidos en directo.",
			de: "Die Projekte des Teams, klar und live verfolgt."
		}
	},
	{
		name: "Sablier",
		icon: "solar:hourglass-bold-duotone",
		href: "https://sablier.facile.studio",
		line: {
			fr: "Vos heures facturables, suivies en un clic.",
			en: "Your billable hours, tracked in one click.",
			es: "Tus horas facturables, registradas con un clic.",
			de: "Ihre abrechenbaren Stunden, mit einem Klick erfasst."
		}
	},
	{
		name: "Vision",
		icon: "solar:panorama-bold-duotone",
		href: "https://vision.facile.studio",
		line: {
			fr: "Les visites de votre site, sans bandeau cookies.",
			en: "Your site's visitors, with no cookie banner.",
			es: "Las visitas de tu web, sin banner de cookies.",
			de: "Die Besucher Ihrer Website, ohne Cookie-Banner."
		}
	},
	{
		name: "Glouton",
		icon: "solar:magnet-wave-bold-duotone",
		href: "https://glouton.facile.studio",
		line: {
			fr: "Vos prochains clients, trouvés pendant que vous dormez.",
			en: "Your next clients, found while you sleep.",
			es: "Tus próximos clientes, encontrados mientras duermes.",
			de: "Ihre nächsten Kunden, gefunden, während Sie schlafen."
		}
	},
	{
		name: "Plume",
		icon: "solar:pen-new-square-bold-duotone",
		href: "https://plume.facile.studio",
		line: {
			fr: "Faites signer un PDF en un lien.",
			en: "Get a PDF signed with one link.",
			es: "Haz firmar un PDF con un enlace.",
			de: "Ein PDF per Link unterschreiben lassen."
		}
	},
	{
		name: "Casier",
		icon: "solar:key-minimalistic-square-bold-duotone",
		href: "https://casier.facile.studio",
		line: {
			fr: "Les secrets de l'équipe, chiffrés et injectés dans vos apps.",
			en: "Your team's secrets, encrypted and injected into your apps.",
			es: "Los secretos del equipo, cifrados e inyectados en tus apps.",
			de: "Team-Secrets, verschlüsselt und direkt in Ihre Apps."
		}
	},
	{
		name: "Capsule",
		icon: "solar:pill-bold-duotone",
		href: "https://capsule.facile.studio",
		line: {
			fr: "Partagez un secret qui disparaît après lecture.",
			en: "Share a secret that disappears once read.",
			es: "Comparte un secreto que desaparece tras leerlo.",
			de: "Teilen Sie ein Geheimnis, das nach dem Lesen verschwindet."
		}
	},
	{
		name: "Sonde",
		icon: "solar:radar-bold-duotone",
		href: "https://sonde.facile.studio",
		line: {
			fr: "Sachez qu'un site tombe avant vos clients.",
			en: "Know a site is down before your clients do.",
			es: "Entérate de que una web cae antes que tus clientes.",
			de: "Wissen, dass eine Website ausfällt, bevor Kunden es merken."
		}
	},
	{
		name: "Place",
		icon: "solar:cup-hot-bold-duotone",
		href: "https://place.facile.studio",
		line: {
			fr: "Le forum de la communauté Facile, relié à Discord.",
			en: "The Facile community forum, linked to Discord.",
			es: "El foro de la comunidad Facile, conectado a Discord.",
			de: "Das Forum der Facile-Community, mit Discord verbunden."
		}
	}
];

/** openApps answers the open apps as muse `WorksWith` rows, in locale. */
export function openApps(locale: Locale): WorksWithApp[] {
	return OPEN_APPS.map(({ line, ...app }) => ({ ...app, line: line[locale] }));
}
