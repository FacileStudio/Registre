export interface PortalApp {
	name: string;
	url: string;
	desc: string;
	icon?: string;
}

export interface LinkMatch extends PortalApp {
	category: string;
	domain: string;
}

export const categories: Record<string, PortalApp[]> = {
	Suite: [
		{
			name: 'Opus',
			url: 'https://opus.facile.studio',
			desc: 'Gestion de projet auto-hébergée, simple et performante.',
			icon: '/icons/opus.ico'
		},
		{
			name: 'Plume',
			url: 'https://plume.facile.studio',
			desc: 'Signature électronique auto-hébergée, simple et moderne.'
		},
		{
			name: 'Sablier',
			url: 'https://sablier.facile.studio',
			desc: 'Outil de suivi de temps de travail par projet.'
		},
		{
			name: 'Nuage',
			url: 'https://nuage.facile.studio',
			desc: "Fichiers partagés, docs et espace cloud de l'équipe."
		},
		{
			name: 'Vision',
			url: 'https://vision.facile.studio',
			desc: 'Analytics web auto-hébergé avec suivi temps réel.'
		},
		{
			name: 'Perception',
			url: 'https://perception.facile.studio',
			desc: 'Mémoire métier et intelligence contextuelle du studio.'
		},
		{
			name: 'Courrier',
			url: 'https://courrier.facile.studio',
			desc: 'Client email auto-hébergé pour le studio.'
		},
		{
			name: 'Agenda',
			url: 'https://agenda.facile.studio',
			desc: 'Calendrier partagé du studio, rendez-vous et deadlines.'
		},
		{
			name: 'Scribe',
			url: 'https://scribe.facile.studio',
			desc: 'Transcription de réunions par IA, en temps réel et 100% local.'
		},
		{
			name: 'Echo',
			url: 'https://echo.facile.studio',
			desc: 'Visioconférence simple et sécurisée.',
			icon: '/icons/echo.ico'
		}
	],
	Infrastructure: [
		{
			name: 'Gare',
			url: 'https://gare.facile.studio',
			desc: 'Déploiement et gestion des apps de la ruche.',
			icon: '/icons/gare.ico'
		},
		{
			name: 'Abeille',
			url: 'https://abeille.facile.studio',
			desc: 'Dashboard du reverse proxy et du routage.',
			icon: '/icons/abeille.ico'
		},
		{
			name: 'Miel',
			url: 'https://miel.facile.studio',
			desc: 'Monitoring serveur, ressources et santé machine.',
			icon: '/icons/miel.ico'
		},
		{
			name: 'Nook',
			url: 'https://nook.facile.studio',
			desc: 'Uptime et serveur de relai de notifications centralisé.'
		}
	],
	Workspace: [
		{
			name: 'Docs',
			url: 'https://docs.facile.studio',
			desc: 'Éditeur de documents collaboratif propulsé par Outline.',
			icon: '/icons/docs.ico'
		},
		{
			name: 'Coffre',
			url: 'https://coffre.facile.studio',
			desc: 'Coffre de mots de passe et secrets partagés.'
		}
	],
	Studio: [
		{
			name: 'Vitrine',
			url: 'https://facile.studio',
			desc: 'Site public du studio et vitrine principale.'
		}
	]
};

export function getDomain(url: string): string {
	try {
		return new URL(url).hostname;
	} catch {
		return '';
	}
}

export function domainColor(d: string): string {
	let h = 0;
	for (let i = 0; i < d.length; i++) {
		h = d.charCodeAt(i) + ((h << 5) - h);
	}
	return `hsl(${Math.abs(h % 360)} 45% 45%)`;
}
