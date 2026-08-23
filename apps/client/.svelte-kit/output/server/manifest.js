export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "login/_app",
	assets: new Set(["apple-touch-icon.png","favicon.ico","favicon.svg","icon-192.png","icon-512.png","logo.svg","site.webmanifest"]),
	mimeTypes: {".png":"image/png",".svg":"image/svg+xml",".webmanifest":"application/manifest+json"},
	_: {
		client: {start:"_app/immutable/entry/start.C3GKAfT-.js",app:"_app/immutable/entry/app.Cr5reEjm.js",imports:["_app/immutable/entry/start.C3GKAfT-.js","_app/immutable/chunks/CeFrwrZz.js","_app/immutable/chunks/DITHSqAA.js","_app/immutable/chunks/DslqFO1_.js","_app/immutable/entry/app.Cr5reEjm.js","_app/immutable/chunks/CeFrwrZz.js","_app/immutable/chunks/DwpkxYY-.js","_app/immutable/chunks/D_k0TvaL.js","_app/immutable/chunks/DslqFO1_.js","_app/immutable/chunks/D0gMWC09.js","_app/immutable/chunks/D922GGr5.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js')),
			__memo(() => import('./nodes/2.js'))
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
