<script lang="ts">
	import { tick } from 'svelte';
	import { type ChatFile, type ChatMessage, fileUrl, UPLOAD_TYPES } from './api';
	import type { Chat } from './chat.svelte';
	import Icon from './Icon.svelte';

	let { chat, onclose, onerror }: { chat: Chat; onclose: () => void; onerror: (msg: string) => void } = $props();

	let text = $state('');
	let sending = $state(false);
	let dragging = $state(false);
	let viewing = $state<ChatFile | null>(null);
	let list = $state<HTMLOListElement>();
	let picker = $state<HTMLInputElement>();
	let field = $state<HTMLTextAreaElement>();
	/** Follow new messages only while the reader is at the bottom. */
	let stick = true;

	const time = new Intl.DateTimeFormat('pt-BR', { hour: '2-digit', minute: '2-digit' });
	const size = (n: number) => (n < 1 << 20 ? `${Math.ceil(n / 1024)} KB` : `${(n / (1 << 20)).toFixed(1)} MB`);
	/** Discord-style: consecutive messages by the same person within 5 min share one header. */
	const grouped = (m: ChatMessage, prev?: ChatMessage) =>
		!!prev && prev.identity === m.identity && Date.parse(m.at) - Date.parse(prev.at) < 5 * 60_000;

	let typingText = $derived.by(() => {
		const names = Object.values(chat.typing);
		if (names.length === 0) return '';
		if (names.length === 1) return `${names[0]} está digitando…`;
		if (names.length === 2) return `${names[0]} e ${names[1]} estão digitando…`;
		return 'Várias pessoas estão digitando…';
	});
	const toBottom = () => list && (list.scrollTop = list.scrollHeight);
	$effect(() => {
		void chat.messages;
		void chat.uploads.length;
		if (stick) tick().then(toBottom);
	});

	async function submit(e?: Event) {
		e?.preventDefault();
		const t = text.trim();
		if (!t || sending) return;
		sending = true;
		try {
			await chat.send(t);
			text = '';
			stick = true;
		} catch (err) {
			onerror((err as Error).message);
		} finally {
			sending = false;
			field?.focus();
		}
	}

	function sendFiles(files: Iterable<File>) {
		stick = true;
		for (const f of files) chat.sendFile(f).catch((err) => onerror((err as Error).message));
	}

	function onKey(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) submit(e);
	}

	function onPaste(e: ClipboardEvent) {
		const files = [...(e.clipboardData?.files ?? [])];
		if (!files.length) return;
		e.preventDefault();
		sendFiles(files);
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		if (e.dataTransfer?.files.length) sendFiles(e.dataTransfer.files);
	}
</script>

<aside
	class="chat"
	class:dragging
	aria-label="Chat da sala"
	ondragover={(e) => {
		if (!e.dataTransfer?.types.includes('Files')) return;
		e.preventDefault();
		dragging = true;
	}}
	ondragleave={(e) => !(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node) && (dragging = false)}
	ondrop={onDrop}
>
	<header>
		<h2>Chat</h2>
		<button class="icon" onclick={onclose} aria-label="Fechar chat"><Icon name="close" size={16} /></button>
	</header>

	<ol class="messages" bind:this={list} onscroll={() => list && (stick = list.scrollHeight - list.scrollTop - list.clientHeight < 80)} aria-live="polite">
		{#each chat.messages as m, i (m.id)}
			<li class:grouped={grouped(m, chat.messages[i - 1])} class:mine={m.identity === chat.identity}>
				{#if !grouped(m, chat.messages[i - 1])}
					<p class="meta"><span class="author">{m.author}</span><time datetime={m.at}>{time.format(new Date(m.at))}</time></p>
				{/if}
				{#if m.text}<p class="text">{m.text}</p>{/if}
				{#if m.file}
					{#if m.file.type.startsWith('image/')}
						<button class="media" onclick={() => (viewing = m.file ?? null)} aria-label={`Ampliar ${m.file.name}`}>
							<img src={fileUrl(m.file.id)} alt={m.file.name} loading="lazy" onload={() => stick && toBottom()} />
						</button>
					{:else}
						<!-- svelte-ignore a11y_media_has_caption -->
						<video class="media" src={fileUrl(m.file.id)} controls preload="metadata" playsinline onloadedmetadata={() => stick && toBottom()}></video>
					{/if}
					<p class="file">
						<span>{m.file.name} · {size(m.file.size)}</span>
						<a href={fileUrl(m.file.id, true)} download={m.file.name} aria-label={`Baixar ${m.file.name}`} title="Baixar"><Icon name="download" size={14} /></a>
					</p>
				{/if}
			</li>
		{:else}
			<li class="empty">Nenhuma mensagem ainda. Mensagens e arquivos somem quando a sala acaba.</li>
		{/each}
		{#each chat.uploads as u (u.key)}
			<li class="upload mine">
				<p class="file">Enviando {u.name} · {Math.round(u.progress * 100)}%</p>
				<div class="bar"><span style:width="{u.progress * 100}%"></span></div>
				<button class="cancel" onclick={() => u.abort.abort()}>Cancelar</button>
			</li>
		{/each}
	</ol>

	<form class="composer" onsubmit={submit}>
		<button type="button" class="icon" onclick={() => picker?.click()} aria-label="Enviar imagem ou vídeo (até 100 MB)" title="Imagem ou vídeo (até 100 MB)"><Icon name="attach" /></button>
		<input
			bind:this={picker}
			type="file"
			accept={UPLOAD_TYPES.join(',')}
			multiple
			hidden
			onchange={(e) => {
				sendFiles([...(e.currentTarget.files ?? [])]);
				e.currentTarget.value = '';
			}}
		/>
		<textarea
			bind:this={field}
			bind:value={text}
			rows="1"
			maxlength="2000"
			placeholder="Mensagem"
			aria-label="Mensagem"
			onkeydown={onKey}
			onpaste={onPaste}
			oninput={() => (text.trim() ? chat.typed() : chat.stoppedTyping())}
			onblur={() => chat.stoppedTyping()}
		></textarea>
		<button type="submit" class="icon" disabled={!text.trim() || sending} aria-label="Enviar mensagem"><Icon name="send" /></button>
	</form>
	<p class="typing" aria-live="polite">{typingText}</p>

	{#if dragging}<div class="drop" aria-hidden="true">Solte para enviar · imagens e vídeos até 100 MB</div>{/if}
</aside>

{#if viewing}
	<dialog
		class="viewer"
		aria-label={viewing.name}
		{@attach (el: HTMLDialogElement) => el.showModal()}
		onclose={() => (viewing = null)}
		onclick={(e) => (e.currentTarget as HTMLDialogElement).close()}
	>
		<img src={fileUrl(viewing.id)} alt={viewing.name} />
	</dialog>
{/if}

<style>
	.chat {
		position: relative;
		display: grid;
		grid-template-rows: auto minmax(0, 1fr) auto auto;
		min-height: 0;
		background: color-mix(in srgb, var(--card) 45%, var(--bg));
		border: 1px solid var(--border);
		border-radius: 16px;
		overflow: hidden;
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 10px 10px 16px;
		border-bottom: 1px solid var(--border);
	}
	h2 { margin: 0; font-size: 14px; font-weight: 600; }
	.icon {
		display: grid;
		place-items: center;
		width: 36px;
		height: 36px;
		padding: 0;
		flex-shrink: 0;
		background: transparent;
		border-color: transparent;
	}
	.messages {
		list-style: none;
		margin: 0;
		padding: 12px 14px;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
	}
	li { padding-top: 12px; min-width: 0; }
	li.grouped { padding-top: 3px; }
	li:first-child { margin-top: auto; }
	.meta { display: flex; align-items: baseline; gap: 8px; margin: 0 0 3px; }
	.author { font-weight: 600; font-size: 13px; }
	.mine .author { color: var(--accent); }
	time { color: var(--muted); font-size: 11px; }
	.text { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; line-height: 1.45; }
	.media {
		display: block;
		max-width: 100%;
		max-height: 280px;
		margin-top: 4px;
		padding: 0;
		border: none;
		border-radius: 10px;
		background: #000;
		overflow: hidden;
	}
	.media img { display: block; max-width: 100%; max-height: 280px; object-fit: contain; }
	video.media { width: 100%; }
	.file { display: flex; align-items: center; gap: 6px; margin: 3px 0 0; color: var(--muted); font-size: 11px; }
	.file span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.file a { display: grid; place-items: center; flex-shrink: 0; width: 24px; height: 24px; border-radius: 6px; color: inherit; }
	.file a:hover { color: var(--fg); background: var(--border); }
	.file a:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
	.empty { margin: auto 0; padding: 24px 8px; color: var(--muted); font-size: 13px; text-align: center; line-height: 1.5; }
	.bar { height: 4px; margin-top: 6px; border-radius: 2px; background: var(--border); overflow: hidden; }
	.bar span { display: block; height: 100%; background: var(--accent); transition: width 0.2s; }
	.cancel { margin-top: 6px; padding: 4px 10px; font-size: 11px; }
	.typing { height: 22px; margin: 0; padding: 0 14px 6px; color: var(--muted); font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.composer {
		display: flex;
		align-items: flex-end;
		gap: 4px;
		padding: 8px;
		border-top: 1px solid var(--border);
	}
	textarea {
		flex: 1;
		min-width: 0;
		min-height: 36px;
		max-height: 160px;
		field-sizing: content;
		resize: none;
		padding: 8px 10px;
		font: inherit;
		color: inherit;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 10px;
		outline: none;
	}
	textarea:focus { border-color: var(--accent); }
	button:focus-visible, textarea:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
	.drop {
		position: absolute;
		inset: 8px;
		display: grid;
		place-items: center;
		padding: 16px;
		text-align: center;
		font-size: 13px;
		background: color-mix(in srgb, var(--accent) 18%, var(--bg));
		border: 2px dashed var(--accent);
		border-radius: 12px;
		pointer-events: none;
	}
	.viewer {
		max-width: 95vw;
		max-height: 95vh;
		padding: 0;
		border: none;
		background: transparent;
	}
	.viewer::backdrop { background: #000d; }
	.viewer img { display: block; max-width: 95vw; max-height: 95vh; object-fit: contain; }
	@media (prefers-reduced-motion: reduce) {
		.bar span { transition: none; }
	}
</style>
