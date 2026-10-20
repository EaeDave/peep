import { goto } from '$app/navigation';
import { invoke } from '@tauri-apps/api/core';
import { getCurrentWindow } from '@tauri-apps/api/window';

export const isTauri = typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window;

/** Mini → Viewer. Tauri opens/focuses the separate viewer window (and hides the popup). */
export async function openViewer(code: string) {
	if (isTauri) await invoke('open_viewer', { code });
	else await goto(`/r/${code}`);
}

/** Viewer "Sair": Tauri closes the viewer window; the browser returns to the Mini screen. */
export async function leaveViewer() {
	if (isTauri) await getCurrentWindow().close();
	else await goto('/join');
}
