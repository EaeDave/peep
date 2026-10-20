import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		sveltekit({
			compilerOptions: {
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			// SPA: one static build served by Caddy (web), bundled by Tauri, opened by Chromium --app.
			adapter: adapter({ fallback: 'index.html' })
		})
	],
	// Tauri dev expects a fixed port.
	server: { port: 5173, strictPort: true }
});
