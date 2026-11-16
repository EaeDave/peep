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
	const msg = await res
		.json()
		.then((j) => j.error as string)
		.catch(() => '');
	throw apiError(res.status, msg);
}

function apiError(status: number, msg: string): ApiError {
	if (status === 401) return new ApiError('Chave de acesso ao servidor inválida. Abra as configurações para corrigi-la.');
	if (status === 404) return new ApiError('Sala não existe');
	if (status === 413) return new ApiError(msg.includes('quota') ? 'Limite de arquivos da sala atingido (1 GB).' : 'Arquivo maior que 100 MB.');
	if (status === 415) return new ApiError('Só imagens (PNG, JPEG, GIF, WebP) e vídeos (MP4, WebM, MOV).');
	if (status === 429) return new ApiError('Muitas tentativas, aguarde um minuto');
	if (status === 503) return new ApiError('Servidor de chamadas indisponível no momento. Tente de novo em instantes.');
	return new ApiError(msg ? `Erro: ${msg}` : `Erro do servidor (${status})`);
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

export interface ChatFile {
	id: string;
	name: string;
	type: string;
	size: number;
}

export interface ChatMessage {
	id: number;
	author: string;
	identity: string;
	text?: string;
	file?: ChatFile;
	at: string;
}

export const MAX_UPLOAD = 100 * 1024 * 1024;
export const UPLOAD_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'video/mp4', 'video/webm', 'video/quicktime'];

export const chatHistory = (code: string) =>
	call<{ messages: ChatMessage[] }>('GET', `/rooms/${encodeURIComponent(code)}/messages`);

export const postMessage = (code: string, body: { name: string; identity: string; text: string; file?: string }) =>
	call<ChatMessage>('POST', `/rooms/${encodeURIComponent(code)}/messages`, body);

export const fileUrl = (id: string, download = false) => `${settings.apiUrl}/files/${encodeURIComponent(id)}${download ? '?download' : ''}`;

/** Uploads one image/video; XHR because fetch can't report upload progress. */
export function uploadFile(code: string, file: File, onProgress: (fraction: number) => void, signal?: AbortSignal) {
	const { promise, resolve, reject } = Promise.withResolvers<ChatFile>();
	const xhr = new XMLHttpRequest();
	xhr.open('POST', `${settings.apiUrl}/rooms/${encodeURIComponent(code)}/files?name=${encodeURIComponent(file.name)}`);
	xhr.setRequestHeader('Authorization', 'Bearer ' + settings.secret);
	xhr.setRequestHeader('Content-Type', file.type);
	xhr.upload.onprogress = (e) => e.lengthComputable && onProgress(e.loaded / e.total);
	xhr.onload = () => {
		let body: { error?: string } & Partial<ChatFile> = {};
		try {
			body = JSON.parse(xhr.responseText);
		} catch {}
		if (xhr.status >= 200 && xhr.status < 300) resolve(body as ChatFile);
		else reject(apiError(xhr.status, body.error ?? ''));
	};
	xhr.onerror = () => reject(new ApiError('Sem conexão com o servidor'));
	xhr.onabort = () => reject(new DOMException('Envio cancelado', 'AbortError'));
	signal?.addEventListener('abort', () => xhr.abort());
	xhr.send(file);
	return promise;
}
