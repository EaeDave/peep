import { settings } from './config';

export class ApiError extends Error {}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
	let res: Response;
	try {
		res = await fetch(settings.apiUrl + path, {
			method,
			headers: {
				Authorization: 'Bearer ' + settings.secret,
				...(body ? { 'Content-Type': 'application/json' } : {})
			},
			body: body ? JSON.stringify(body) : undefined
		});
	} catch {
		throw new ApiError('Sem conexão com o servidor');
	}
	if (res.ok) return res.json();
	if (res.status === 401) throw new ApiError('Chave de acesso ao servidor inválida. Abra as configurações para corrigi-la.');
	if (res.status === 404) throw new ApiError('Sala não existe');
	if (res.status === 429) throw new ApiError('Muitas tentativas, aguarde um minuto');
	if (res.status === 503) throw new ApiError('Servidor de chamadas indisponível no momento. Tente de novo em instantes.');
	const msg = await res
		.json()
		.then((j) => j.error as string)
		.catch(() => '');
	throw new ApiError(msg ? `Erro: ${msg}` : `Erro do servidor (${res.status})`);
}

export const createRoom = (name: string) =>
	call<{ code: string; expiresAt: string }>('POST', '/rooms', { name });

export const joinRoom = (code: string, name: string) =>
	call<{ token: string; url: string; room: string }>(
		'POST',
		`/rooms/${encodeURIComponent(code)}/join`,
		{ name }
	);

export const getRoom = (code: string) =>
	call<{ exists: boolean; participants: number }>('GET', `/rooms/${encodeURIComponent(code)}`);

/** Fire-and-forget quality sample (diagnostics); failures are ignored. */
export function reportStats(code: string, sample: object) {
	void fetch(`${settings.apiUrl}/rooms/${encodeURIComponent(code)}/stats`, {
		method: 'POST',
		headers: { Authorization: 'Bearer ' + settings.secret, 'Content-Type': 'application/json' },
		body: JSON.stringify(sample),
		keepalive: true
	}).catch(() => {});
}
