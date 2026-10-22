<script lang="ts">
	import { onMount } from 'svelte';
	import { nativeFeel } from '#lib/nativeFeel.ts';

	let { children } = $props();
	onMount(nativeFeel);
</script>

{@render children()}

<style>
	:global {
		:root {
			--bg: #0f0f13;
			--card: #1a1a22;
			--fg: #ececf1;
			--muted: #8b8b99;
			--border: #2c2c38;
			--accent: #7c5cff;
			--accent-fg: #fff;
			--danger: #ff5c7a;
			--ok: #3ecf8e;
			--warn: #f5b84a;
			color-scheme: dark;
		}
		@media (prefers-color-scheme: light) {
			:root {
				--bg: #f6f6f9;
				--card: #ffffff;
				--fg: #18181f;
				--muted: #6b6b78;
				--border: #e1e1e8;
				--accent: #6748d3;
				--danger: #b72346;
				--ok: #16734b;
				color-scheme: light;
			}
		}
		* {
			box-sizing: border-box;
		}
		html,
		body {
			margin: 0;
			height: 100%;
			background: var(--bg);
			color: var(--fg);
			font:
				14px/1.4 system-ui,
				-apple-system,
				'Segoe UI',
				sans-serif;
		}
		button,
		input,
		select {
			font: inherit;
			color: inherit;
		}
		input {
			width: 100%;
			padding: 10px 12px;
			border-radius: 10px;
			border: 1px solid var(--border);
			background: var(--bg);
			outline: none;
		}
		input:focus {
			border-color: var(--accent);
		}
		button {
			cursor: pointer;
			border: 1px solid var(--border);
			background: var(--card);
			border-radius: 10px;
			padding: 10px 14px;
		}
		button:disabled {
			opacity: 0.5;
			cursor: default;
		}
		button:focus-visible,
		a:focus-visible,
		input:focus-visible {
			outline: 2px solid var(--accent);
			outline-offset: 3px;
		}
		button.primary {
			background: var(--accent);
			border-color: var(--accent);
			color: var(--accent-fg);
		}
		label {
			display: grid;
			gap: 4px;
			font-size: 12px;
			color: var(--muted);
		}
		.error {
			color: var(--danger);
			font-size: 13px;
		}
		/* Desktop app: UI text isn't selectable like a web page; fields still are. */
		.app-window body {
			user-select: none;
			cursor: url('/cursors/arrow.svg') 4 3, default;
		}
		.app-window input,
		.app-window textarea {
			user-select: text;
			caret-color: var(--accent);
		}
		/* Branded cursors (desktop app only). !important: components set their own
		   `cursor: pointer` with higher-specificity scoped selectors. */
		.app-window :is(button, a[href], label, select, summary, [role='button'], input[type='checkbox'], input[type='range']) {
			cursor: url('/cursors/pointer.svg') 10 2, pointer !important;
		}
		.app-window :is(button:disabled, input:disabled) {
			cursor: url('/cursors/arrow.svg') 4 3, default !important;
		}
		.app-window :is(input:not([type='checkbox'], [type='range']), textarea) {
			cursor: url('/cursors/text.svg') 12 12, text !important;
		}
	}
</style>
