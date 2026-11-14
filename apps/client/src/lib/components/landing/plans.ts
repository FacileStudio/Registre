/**
 * PLANS mirrors the public catalog Ardoise serves at
 * https://api-ardoise.facile.studio/api/catalog, read on 2026-11-14. The
 * landing renders this copy instead of fetching it, because Registre's
 * content security policy keeps `connect-src` to its own origin. Update it
 * when the published plans or prices change.
 */
export const PLANS = [
	{
		key: "solo",
		name: "Solo",
		features: {
			"nuage.storage_gb": 5,
			"courrier.mailboxes": -1,
			"echo.max_call_minutes": -1,
			"echo.max_participants": 4,
			"echo.recording": false,
			"glouton.access": false,
			"ai.access": false
		},
		prices: []
	},
	{
		key: "pro",
		name: "Pro",
		features: {
			"nuage.storage_gb": 100,
			"courrier.mailboxes": -1,
			"echo.max_call_minutes": -1,
			"echo.max_participants": 25,
			"echo.recording": true,
			"glouton.access": true,
			"ai.access": true
		},
		prices: [
			{ interval: "month", amount: 900, currency: "eur", lookup_key: "plan_pro_month" },
			{ interval: "year", amount: 9000, currency: "eur", lookup_key: "plan_pro_year" }
		]
	},
	{
		key: "studio",
		name: "Studio",
		features: {
			"nuage.storage_gb": 500,
			"courrier.mailboxes": -1,
			"echo.max_call_minutes": -1,
			"echo.max_participants": 100,
			"echo.recording": true,
			"glouton.access": true,
			"ai.access": true
		},
		prices: [
			{ interval: "month", amount: 2900, currency: "eur", lookup_key: "plan_studio_month" },
			{ interval: "year", amount: 29000, currency: "eur", lookup_key: "plan_studio_year" }
		]
	}
];
