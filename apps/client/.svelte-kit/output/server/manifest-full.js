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
		client: {start:"_app/immutable/entry/start.CcE_ALZ5.js",app:"_app/immutable/entry/app.DiqlBvdx.js",imports:["_app/immutable/entry/start.CcE_ALZ5.js","_app/immutable/chunks/ChCl7fFr.js","_app/immutable/chunks/DttA_BOH.js","_app/immutable/chunks/DwL2DErn.js","_app/immutable/entry/app.DiqlBvdx.js","_app/immutable/chunks/ChCl7fFr.js","_app/immutable/chunks/DhvMAwZs.js","_app/immutable/chunks/MFEXDae9.js","_app/immutable/chunks/DwL2DErn.js","_app/immutable/chunks/Dw-IPWKq.js","_app/immutable/chunks/msEYsEgC.js","_app/immutable/chunks/CJjGPKA3.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
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
