import { isPermissionGranted, sendNotification } from '@tauri-apps/plugin-notification';
import { isTauri } from '#lib/platform.ts';

/** Room events worth telling the people already there about. */
export type NoticeKind = 'join' | 'leave' | 'share';

let ctx: AudioContext | undefined;

/** Short, quiet two-note cue generated in place (no audio asset). */
export function chime(kind: NoticeKind) {
	try {
		ctx ??= new AudioContext();
		if (ctx.state === 'suspended') void ctx.resume();
		// Rising = someone arrived/shares, falling = someone left.
		const notes = kind === 'leave' ? [660, 440] : kind === 'join' ? [523, 784] : [784, 988];
		const t0 = ctx.currentTime;
		notes.forEach((freq, i) => {
			const osc = new OscillatorNode(ctx!, { type: 'sine', frequency: freq });
			const gain = new GainNode(ctx!, { gain: 0 });
			const start = t0 + i * 0.11;
			gain.gain.setValueAtTime(0, start);
			gain.gain.linearRampToValueAtTime(0.07, start + 0.015);
			gain.gain.exponentialRampToValueAtTime(0.0001, start + 0.18);
			osc.connect(gain).connect(ctx!.destination);
			osc.start(start);
			osc.stop(start + 0.2);
		});
	} catch {
		/* audio unavailable: the visual notice is enough */
	}
}

/**
 * Browsers only allow the permission prompt from a user gesture: ask on the
 * first click in the room. The Tauri app needs no prompt.
 */
export function askNotificationPermission() {
	if (isTauri || typeof Notification === 'undefined' || Notification.permission !== 'default') return;
	const ask = () => void Notification.requestPermission().catch(() => {});
	window.addEventListener('pointerdown', ask, { once: true });
}

/** OS notification, only when the room window isn't focused (in-app notice covers the rest). */
export async function systemNotify(title: string, body: string) {
	if (document.hasFocus()) return;
	try {
		if (isTauri) {
			if (await isPermissionGranted()) sendNotification({ title, body });
			return;
		}
		if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return;
		const n = new Notification(title, { body, icon: '/favicon.svg', tag: `peep-${body}` });
		n.onclick = () => {
			window.focus();
			n.close();
		};
	} catch {
		/* notifications blocked or unsupported */
	}
}
