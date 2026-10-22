import { replaceState } from '$app/navigation';
import { page } from '$app/state';

// Local client settings (localStorage, never fatal if unavailable).

const DEFAULT_API = import.meta.env.VITE_API_URL ?? '';

function get(k: string): string {
	try {
		return localStorage.getItem('peep.' + k) ?? '';
	} catch {
		return '';
	}
}

function set(k: string, v: string) {
	try {
		localStorage.setItem('peep.' + k, v);
	} catch {
		/* private mode etc. — keep going in memory */
	}
}

export const settings = {
	get name() {
		return get('name');
	},
	set name(v: string) {
		set('name', v.trim());
	},
	get secret() {
		return get('secret');
	},
	set secret(v: string) {
		set('secret', v.trim());
	},
	get apiUrl() {
		return (get('apiUrl') || DEFAULT_API).replace(/\/+$/, '');
	},
	set apiUrl(v: string) {
		set('apiUrl', v.trim());
	},
	/** Chosen microphone (deviceId); empty = system default. */
	get micId() {
		return get('micId');
	},
	set micId(v: string) {
		set('micId', v);
	},
	/** Noise suppression: strong (GTCRN, default), light (RNNoise) or off. */
	get noiseMode(): 'strong' | 'light' | 'off' {
		const v = get('noiseMode');
		return v === 'light' || v === 'off' ? v : 'strong';
	},
	set noiseMode(v: 'strong' | 'light' | 'off') {
		set('noiseMode', v);
	},
	/** Join/leave/share notices (in-app + system); on unless turned off. */
	get notify() {
		return get('notify') !== 'off';
	},
	set notify(on: boolean) {
		set('notify', on ? 'on' : 'off');
	},
	/** Voice volume (0..2, Discord-style boost) per display name: identities change on every join. */
	voiceVolume(name: string): number {
		const v = readVolumes()[name];
		return typeof v === 'number' && v >= 0 && v <= 2 ? v : 1;
	},
	setVoiceVolume(name: string, v: number) {
		const all = readVolumes();
		if (v === 1) delete all[name];
		else all[name] = v;
		set('voiceVolumes', JSON.stringify(all));
	},
	get configured() {
		return !!(this.secret && this.apiUrl);
	}
};

function readVolumes(): Record<string, unknown> {
	try {
		const v: unknown = JSON.parse(get('voiceVolumes') || '{}');
		return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
	} catch {
		return {};
	}
}

/** Validates a user-configured API endpoint before sending a server access key. */
export function normalizeServerUrl(raw: string): string {
	let url: URL;
	try {
		url = new URL(raw.trim());
	} catch {
		throw new Error('Informe a URL completa da API, começando com https://.');
	}
	const local = ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname);
	if (url.protocol !== 'https:' && !(url.protocol === 'http:' && local)) {
		throw new Error('Use HTTPS para acessar o servidor. HTTP só é permitido em localhost.');
	}
	if (url.username || url.password || url.search || url.hash) {
		throw new Error('A URL da API não deve conter credenciais, parâmetros ou fragmentos.');
	}
	return url.toString().replace(/\/+$/, '');
}

/**
 * Absorbs launcher parameters (Quickshell): `?name=` in the query and
 * `#secret=…&api=…` in the fragment — fragments never reach the server or its
 * logs. The fragment is stripped from the address bar afterwards.
 */
export function absorbUrlParams() {
	const url = new URL(location.href);
	const q = url.searchParams;
	const h = new URLSearchParams(url.hash.slice(1));
	if (q.get('name')) settings.name = q.get('name')!;
	if (h.get('secret')) settings.secret = h.get('secret')!;
	if (h.get('api')) settings.apiUrl = h.get('api')!;
	if (url.hash) {
		url.hash = '';
		replaceState(url, page.state);
	}
}

const ALPHABET = 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';

/** Input mask: keeps valid chars, uppercases, inserts the hyphen: "kj729f" → "KJ7-29F". */
export function maskCode(raw: string): string {
	const s = [...raw.toUpperCase()].filter((c) => ALPHABET.includes(c)).join('').slice(0, 6);
	return s.length > 3 ? s.slice(0, 3) + '-' + s.slice(3) : s;
}

export const isFullCode = (c: string) => /^[A-Z0-9]{3}-[A-Z0-9]{3}$/.test(c);
