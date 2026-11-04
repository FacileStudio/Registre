/**
 * A space's plan, as the spaces page shows it.
 *
 * Registre calls Ardoise server to server and hands the page only what it
 * needs: the plan, and for owners the published catalog. The page never sees
 * the billing token. Checkout and portal answer a Stripe URL the server has
 * already checked; `isStripeURL` checks it again before the page leaves,
 * because a redirect is the one thing here an attacker would want to steer.
 */

import { request } from './account';

/** Price is one way to pay for a plan; `amount` is in minor units. */
export interface Price {
	interval: 'month' | 'year' | string;
	amount: number;
	currency: string;
	lookup_key: string;
}

/** CatalogPlan is one published plan. */
export interface CatalogPlan {
	key: string;
	name: string;
	prices: Price[];
}

/** SpacePlan is the plan a space is on. */
export interface SpacePlan {
	plan: string;
	status: string;
	current_period_end?: string;
	comped: boolean;
}

/** PlanView is the plan section; `enabled` false hides it entirely. */
export interface PlanView {
	enabled: boolean;
	available: boolean;
	owner: boolean;
	plan?: SpacePlan;
	catalog?: CatalogPlan[];
}

const base = (id: string) => `/api/spaces/${encodeURIComponent(id)}/billing`;

/** billing is the plan section's API, one method per route. */
export const billing = {
	get: (id: string) => request<PlanView>(base(id)),
	checkout: (id: string, lookup_key: string) => request<{ url: string }>(`${base(id)}/checkout`, { lookup_key }),
	portal: (id: string) => request<{ url: string }>(`${base(id)}/portal`, {})
};

/** STRIPE_ORIGINS are the only places the page will send a human to pay. */
const STRIPE_ORIGINS: ReadonlySet<string> = new Set(['https://checkout.stripe.com', 'https://billing.stripe.com']);

/** isStripeURL reports whether `raw` is an https URL on one of Stripe's billing origins. */
export function isStripeURL(raw: string): boolean {
	try {
		const url = new URL(raw);
		return url.username === '' && url.password === '' && STRIPE_ORIGINS.has(url.origin);
	} catch {
		return false;
	}
}

/** formatPrice renders a price such as "12 EUR / month". */
export function formatPrice(price: Price): string {
	const amount = new Intl.NumberFormat(undefined, {
		style: 'currency',
		currency: price.currency.toUpperCase(),
		minimumFractionDigits: price.amount % 100 === 0 ? 0 : 2
	}).format(price.amount / 100);
	return `${amount} / ${price.interval}`;
}
