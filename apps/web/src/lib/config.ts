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
	get configured() {
		return !!(this.secret && this.apiUrl);
	}
};

/**
 * Absorbs ?name=, ?secret= and ?api= (used by the Quickshell launcher) into
 * settings and strips secret/api from the address bar.
 */
export function absorbUrlParams() {
	const url = new URL(location.href);
	const p = url.searchParams;
	if (p.get('name')) settings.name = p.get('name')!;
	if (p.get('secret')) settings.secret = p.get('secret')!;
	if (p.get('api')) settings.apiUrl = p.get('api')!;
	if (p.has('secret') || p.has('api')) {
		p.delete('secret');
		p.delete('api');
		history.replaceState(history.state, '', url);
	}
}

const ALPHABET = 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';

/** Input mask: keeps valid chars, uppercases, inserts the hyphen: "kj729f" → "KJ7-29F". */
export function maskCode(raw: string): string {
	const s = [...raw.toUpperCase()].filter((c) => ALPHABET.includes(c)).join('').slice(0, 6);
	return s.length > 3 ? s.slice(0, 3) + '-' + s.slice(3) : s;
}

export const isFullCode = (c: string) => /^[A-Z0-9]{3}-[A-Z0-9]{3}$/.test(c);
