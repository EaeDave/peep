import { goto } from '$app/navigation';
import { invoke } from '@tauri-apps/api/core';
import { listen } from '@tauri-apps/api/event';
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

/** Desktop updater (Tauri): [current version, available version or null]. */
export type UpdateState = [string, string | null];
export const updateStatus = () => invoke<UpdateState>('update_status');
export const checkUpdate = () => invoke<UpdateState>('check_update');
export const installUpdate = () => invoke<void>('install_update');
export const onUpdateAvailable = (fn: (version: string) => void) =>
	listen<string>('update-available', (e) => fn(e.payload));
