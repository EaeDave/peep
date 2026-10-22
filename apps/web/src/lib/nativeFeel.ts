import { isTauri } from '#lib/platform.ts';

/**
 * True inside the desktop apps: Tauri (Windows) or the Omarchy Chromium
 * `--app` window, which reports `display-mode: standalone`. A normal browser
 * tab keeps zoom and its context menu (accessibility, user's own browser).
 */
export const isAppWindow =
	typeof window !== 'undefined' && (isTauri || matchMedia('(display-mode: standalone)').matches);

const editable = (target: EventTarget | null) =>
	target instanceof Element && !!target.closest('input, textarea, [contenteditable="true"]');

/** Makes the app window behave like a native app: no zoom, no browser context menu. */
export function nativeFeel() {
	if (!isAppWindow) return;
	document.documentElement.classList.add('app-window');
	// Text fields keep the menu for copy/paste; the room's own right-click menus prevent it themselves.
	window.addEventListener('contextmenu', (e) => {
		if (!editable(e.target)) e.preventDefault();
	});
	// Ctrl+wheel is also what touchpad pinch-zoom sends.
	window.addEventListener('wheel', (e) => e.ctrlKey && e.preventDefault(), { passive: false });
	window.addEventListener('keydown', (e) => {
		if ((e.ctrlKey || e.metaKey) && ['+', '=', '-', '_', '0'].includes(e.key)) e.preventDefault();
	});
}
