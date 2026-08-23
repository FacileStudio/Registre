export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "_app",
	assets: new Set(["apple-touch-icon.png","favicon.ico","favicon.svg","icon-192.png","icon-512.png","logo.svg","site.webmanifest"]),
	mimeTypes: {".png":"image/png",".svg":"image/svg+xml",".webmanifest":"application/manifest+json"},
	_: {
		client: {start:"_app/immutable/entry/start.C3pvREvu.js",app:"_app/immutable/entry/app.DaG8yA9g.js",imports:["_app/immutable/entry/start.C3pvREvu.js","_app/immutable/chunks/NfViqqFc.js","_app/immutable/chunks/BnW1LItH.js","_app/immutable/chunks/BDhTNmVl.js","_app/immutable/entry/app.DaG8yA9g.js","_app/immutable/chunks/NfViqqFc.js","_app/immutable/chunks/F74xo14v.js","_app/immutable/chunks/DzawSXhi.js","_app/immutable/chunks/BFAP45fo.js","_app/immutable/chunks/BDhTNmVl.js","_app/immutable/chunks/BOpYJB7z.js","_app/immutable/chunks/D5CgeNQV.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js')),
			__memo(() => import('./nodes/2.js')),
			__memo(() => import('./nodes/3.js'))
		],
		remotes: {
			
		},
		routes: [
			{
				id: "/",
				pattern: /^\/$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 2 },
				endpoint: null
			},
			{
				id: "/login",
				pattern: /^\/login\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 3 },
				endpoint: null
			}
		],
		prerendered_routes: new Set([]),
		matchers: async () => {
			
			return {  };
		},
		server_assets: {}
	}
}
})();
