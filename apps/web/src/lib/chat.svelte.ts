import { type Room, RoomEvent } from 'livekit-client';
import { ApiError, chatHistory, type ChatMessage, MAX_UPLOAD, postMessage, uploadFile, UPLOAD_TYPES } from './api';

/** Must match ChatTopic in apps/api/internal/http/chat.go. */
const TOPIC = 'chat';

export interface Upload {
	key: number;
	name: string;
	progress: number;
	abort: AbortController;
}

/**
 * Room chat. The room-api is the source of truth (it keeps the history until the
 * room dies) and pushes each new message to the room over LiveKit data.
 */
export class Chat {
	/** Oldest first, unique by id. */
	messages = $state.raw<ChatMessage[]>([]);
	uploads = $state<Upload[]>([]);
	/** Messages from others that arrived while the panel was hidden. */
	unread = $state(0);
	#visible = false;
	#nextKey = 0;

	constructor(
		readonly room: Room,
		readonly code: string
	) {
		const decoder = new TextDecoder();
		room
			.on(RoomEvent.DataReceived, (payload, _from, _kind, topic) => {
				if (topic !== TOPIC) return;
				try {
					this.#add([JSON.parse(decoder.decode(payload))]);
				} catch {
					// Malformed packet: nothing to show.
				}
			})
			// Pushes sent while the connection was down are lost; the history has them.
			.on(RoomEvent.Reconnected, () => void this.load().catch(() => {}));
	}

	get identity() {
		return this.room.localParticipant.identity;
	}

	/** The name this session joined with (settings.name is shared by every open tab). */
	#author() {
		return { name: this.room.localParticipant.name || this.identity, identity: this.identity };
	}

	setVisible(v: boolean) {
		this.#visible = v;
		if (v) this.unread = 0;
	}

	async load() {
		const { messages } = await chatHistory(this.code);
		this.#add(messages, false);
	}

	#add(list: ChatMessage[], count = true) {
		const known = new Set(this.messages.map((m) => m.id));
		const fresh = list.filter((m) => !known.has(m.id));
		if (!fresh.length) return;
		if (count && !this.#visible) this.unread += fresh.filter((m) => m.identity !== this.identity).length;
		this.messages = [...this.messages, ...fresh].sort((a, b) => a.id - b.id);
	}

	async send(text: string) {
		this.#add([await postMessage(this.code, { ...this.#author(), text })], false);
	}

	/** Uploads an image/video and posts it as a message. Rejects with a readable error. */
	async sendFile(file: File) {
		if (!UPLOAD_TYPES.includes(file.type)) throw new ApiError(`${file.name}: só imagens (PNG, JPEG, GIF, WebP) e vídeos (MP4, WebM, MOV).`);
		if (file.size > MAX_UPLOAD) throw new ApiError(`${file.name}: arquivo maior que 100 MB.`);
		const upload: Upload = $state({ key: this.#nextKey++, name: file.name, progress: 0, abort: new AbortController() });
		this.uploads.push(upload);
		try {
			const f = await uploadFile(this.code, file, (p) => (upload.progress = p), upload.abort.signal);
			this.#add([await postMessage(this.code, { ...this.#author(), text: '', file: f.id })], false);
		} catch (e) {
			if ((e as Error).name === 'AbortError') return;
			throw new ApiError(`${file.name}: ${(e as Error).message}`);
		} finally {
			this.uploads = this.uploads.filter((u) => u.key !== upload.key);
		}
	}
}
