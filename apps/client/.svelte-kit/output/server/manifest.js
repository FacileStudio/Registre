export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "login/_app",
	assets: new Set(["favicon.svg"]),
	mimeTypes: {".svg":"image/svg+xml"},
	_: {
		client: {start:"_app/immutable/entry/start.BLyFs1K6.js",app:"_app/immutable/entry/app.CPddnNbF.js",imports:["_app/immutable/entry/start.BLyFs1K6.js","_app/immutable/chunks/CeFrwrZz.js","_app/immutable/chunks/BXxBgkhG.js","_app/immutable/chunks/DslqFO1_.js","_app/immutable/entry/app.CPddnNbF.js","_app/immutable/chunks/CeFrwrZz.js","_app/immutable/chunks/DwpkxYY-.js","_app/immutable/chunks/D_k0TvaL.js","_app/immutable/chunks/DslqFO1_.js","_app/immutable/chunks/D0gMWC09.js","_app/immutable/chunks/D922GGr5.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
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
