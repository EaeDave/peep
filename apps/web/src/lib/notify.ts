import { isPermissionGranted, sendNotification } from '@tauri-apps/plugin-notification';
import { isTauri } from '#lib/platform.ts';

/** Room events worth telling the people already there about. */
export type NoticeKind = 'join' | 'leave' | 'share';
/** Room sounds: the notices plus your own arrival and departure. */
export type SoundKind = NoticeKind | 'selfJoin' | 'selfLeave';

/** Master volume of room sounds; the files are pre-normalized to -20 dB mean. */
const VOLUME = 0.15;

/** Kenney CC0 sounds (static/sounds); `leave` is synthesized below. */
const FILES: Partial<Record<SoundKind, string>> = {
	selfJoin: '/sounds/self-join.ogg',
	join: '/sounds/join.ogg',
	share: '/sounds/share.ogg',
	selfLeave: '/sounds/self-leave.ogg'
};

let ctx: AudioContext | undefined;
const buffers = new Map<string, Promise<AudioBuffer>>();

function load(c: AudioContext, url: string): Promise<AudioBuffer> {
	let b = buffers.get(url);
	if (!b) {
		b = fetch(url)
			.then((r) => r.arrayBuffer())
			.then((data) => c.decodeAudioData(data));
		b.catch(() => buffers.delete(url));
		buffers.set(url, b);
	}
	return b;
}

/** Plays a room sound; resolves when it ends (or right away if audio is unavailable). */
export async function roomSound(kind: SoundKind): Promise<void> {
	try {
		ctx ??= new AudioContext();
		if (ctx.state === 'suspended') await ctx.resume();
		const c = ctx;
		const out = new GainNode(c, { gain: VOLUME });
		out.connect(c.destination);
		const { promise: ended, resolve } = Promise.withResolvers<void>();
		const url = FILES[kind];
		if (url) {
			const src = new AudioBufferSourceNode(c, { buffer: await load(c, url) });
			src.connect(out);
			src.onended = () => resolve();
			src.start();
			return ended;
		}
		// Someone left: soft descending pair (triangle wave).
		const t0 = c.currentTime;
		for (const [freq, at, dur] of [[784, 0, 0.12], [523, 0.1, 0.3]]) {
			const osc = new OscillatorNode(c, { type: 'triangle', frequency: freq });
			const env = new GainNode(c, { gain: 0 });
			env.gain.setValueAtTime(0, t0 + at);
			env.gain.linearRampToValueAtTime(0.6, t0 + at + 0.008);
			env.gain.exponentialRampToValueAtTime(0.0001, t0 + at + dur);
			osc.connect(env).connect(out);
			osc.start(t0 + at);
			osc.stop(t0 + at + dur + 0.02);
		}
		setTimeout(resolve, 420);
		return ended;
	} catch {
		/* audio unavailable: the visual notice is enough */
	}
}

/** Warms the decoded sounds so the first one plays without a fetch delay. */
export function preloadRoomSounds() {
	try {
		ctx ??= new AudioContext();
		for (const url of Object.values(FILES)) void load(ctx, url).catch(() => {});
	} catch {
		/* no Web Audio */
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
