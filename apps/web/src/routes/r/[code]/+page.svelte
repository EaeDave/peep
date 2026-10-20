<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { ConnectionQuality, type Track } from 'livekit-client';
	import { joinRoom } from '#lib/api.ts';
	import { absorbUrlParams, maskCode, normalizeServerUrl, settings } from '#lib/config.ts';
	import { QUALITY, type QualityMode, Session } from '#lib/livekit/session.svelte.ts';
	import { leaveViewer } from '#lib/platform.ts';
	import Icon from '#lib/Icon.svelte';
	import ServerSettings from '#lib/ServerSettings.svelte';

	const code = maskCode(page.params.code ?? '');
	let session = $state<Session | null>(null);
	let needName = $state(true);
	let name = $state('');
	let settingsOpen = $state(false);
	let settingsNotice = $state('');
	let connecting = $state(false);
	let destroyed = false;
	let error = $state('');
	let toast = $state('');
	let mode = $state<QualityMode>('jogo');
	let menu = $state(false);
	let stripOpen = $state(true);
	let peopleOpen = $state(false);
	let controls = $state(true);
	let hideTimer: ReturnType<typeof setTimeout>;

	onMount(() => {
		absorbUrlParams();
		name = settings.name;
		if (!validConfig()) settingsOpen = true;
		else if (name) connect();
		poke();
	});

	onDestroy(() => {
		destroyed = true;
		clearTimeout(hideTimer);
		void session?.room.disconnect();
	});

	function validConfig() {
		try {
			normalizeServerUrl(settings.apiUrl);
			return settings.configured;
		} catch {
			return false;
		}
	}

	function savedSettings() {
		settingsOpen = false;
		settingsNotice = session
			? 'Configurações salvas. Serão usadas na próxima entrada; esta sala não mudou.'
			: 'Configurações salvas. Você já pode entrar na sala.';
	}

	async function connect() {
		if (connecting || destroyed) return;
		if (!validConfig()) {
			settingsOpen = true;
			return;
		}
		connecting = true;
		error = '';
		settingsNotice = '';
		settings.name = name;
		needName = false;
		const previous = session;
		session = null;
		let next: Session | null = null;
		try {
			await previous?.room.disconnect();
			const { token, url } = await joinRoom(code, settings.name);
			if (destroyed) return;
			next = new Session();
			session = next;
			await next.connect(url, token);
			if (destroyed) await next.room.disconnect();
		} catch (e) {
			await next?.room.disconnect();
			if (!destroyed) {
				session = null;
				error = (e as Error).message || 'Falha ao conectar';
			}
		} finally {
			connecting = false;
		}
	}

	function flash(msg: string) {
		toast = msg;
		setTimeout(() => toast === msg && (toast = ''), 2000);
	}

	async function act(fn: () => Promise<void>) {
		try {
			await fn();
		} catch (e) {
			flash((e as Error).message);
		}
	}

	const toggleScreen = (m = mode) =>
		act(async () => {
			menu = false;
			mode = m;
			await session?.setScreen(!session.screenOn, m);
		});
	const toggleCamera = () => act(async () => void (await session?.setCamera(!session.cameraOn)));
	const toggleMic = () => act(async () => void (await session?.setMic(!session.micOn)));

	async function copyCode() {
		await navigator.clipboard.writeText(code).catch(() => {});
		flash('Código copiado');
	}

	async function leave() {
		await session?.room.disconnect();
		await leaveViewer();
	}

	function toggleFullscreen() {
		if (document.fullscreenElement) document.exitFullscreen();
		else document.documentElement.requestFullscreen().catch(() => {});
	}

	// VW-5: controls auto-hide after 3 s without mouse movement.
	function poke() {
		controls = true;
		clearTimeout(hideTimer);
		hideTimer = setTimeout(() => !menu && !settingsOpen && (controls = false), 3000);
	}

	function onKey(e: KeyboardEvent) {
		if (settingsOpen || (e.target as HTMLElement).closest('input, textarea, select') || !session) return;
		const k = e.key.toLowerCase();
		if (e.ctrlKey && e.shiftKey && k === 's') {
			e.preventDefault();
			toggleScreen();
		} else if (e.ctrlKey && e.shiftKey && k === 'c') {
			e.preventDefault();
			toggleCamera();
		} else if (k === 'f' && !e.ctrlKey && !e.altKey && !e.metaKey) toggleFullscreen();
	}

	const attachTrack = (track: Track) => (el: HTMLVideoElement) => {
		track.attach(el);
		return () => void track.detach(el);
	};

	const QUALITY_COLOR: Record<ConnectionQuality, string> = {
		[ConnectionQuality.Excellent]: 'var(--ok)',
		[ConnectionQuality.Good]: 'var(--warn)',
		[ConnectionQuality.Poor]: 'var(--danger)',
		[ConnectionQuality.Lost]: 'var(--muted)',
		[ConnectionQuality.Unknown]: 'var(--muted)'
	};

	let focus = $derived(session?.tiles.find((t) => t.id === session?.focusId));
</script>

<svelte:window onkeydown={onKey} onmousemove={poke} />

{#if needName || error}
	<div class="center">
		<form
			class="card"
			onsubmit={(e) => {
				e.preventDefault();
				connect();
			}}
		>
			<header class="setup-header">
				<span class="brand">peep<span>.</span></span>
				<button class="icon-button" type="button" aria-label="Configurações do servidor" onclick={() => (settingsOpen = true)}><Icon name="settings" /></button>
			</header>
			<div>
				<h1>Entrar na sala</h1>
				<p class="setup-help">Você vai entrar na sala <strong class="mono">{code}</strong>.</p>
			</div>
			{#if error}<p class="error" role="alert">{error}</p>{/if}
			{#if settingsNotice}<p class="notice" role="status">{settingsNotice}</p>{/if}
			<label>Seu nome <input bind:value={name} maxlength="24" autocomplete="nickname" required /></label>
			<button class="primary" type="submit" disabled={connecting}>Entrar na sala <Icon name="arrow-right" /></button>
			<button type="button" onclick={leaveViewer}><Icon name="arrow-left" /> Voltar</button>
		</form>
	</div>
{:else if session}
	<div class="viewer" class:cursor-hidden={!controls}>
		<section class="stage" ondblclick={toggleFullscreen} role="presentation">
			{#if focus}
				{#key focus.id}
					<video
						{@attach attachTrack(focus.track)}
						class:mirror={focus.local && !focus.screen}
						autoplay
						playsinline
						muted
					></video>
				{/key}
				<span class="tag">{focus.name}{focus.screen ? ' · tela' : ''}</span>
			{:else}
				<div class="empty">
					<p>Ninguém compartilhando ainda</p>
					<p class="mono big">{code}</p>
					<button onclick={copyCode}>Copiar código</button>
				</div>
			{/if}

			{#if session.status === 'reconnecting' || session.status === 'connecting'}
				<div class="banner">{session.status === 'connecting' ? 'Conectando…' : 'Reconectando…'}</div>
			{:else if session.status === 'disconnected'}
				<div class="banner">
					Desconectado <button onclick={connect}>Reconectar</button>
				</div>
			{/if}
			{#if session.audioBlocked}
				<button class="banner audio" onclick={() => session?.room.startAudio()}>
					Clique para ativar o áudio
				</button>
			{/if}
		</section>

		{#if session.tiles.length > 1 || (session.tiles.length === 1 && !focus)}
			<aside class="strip" class:closed={!stripOpen}>
				<button class="handle" aria-label={stripOpen ? 'Recolher miniaturas' : 'Mostrar miniaturas'} aria-expanded={stripOpen} onclick={() => (stripOpen = !stripOpen)}>
					<span class:rotate={!stripOpen}><Icon name="chevron-down" /></span>
				</button>
				{#if stripOpen}
					{#each session.tiles as t (t.id)}
						<button
							class="thumb"
							class:active={t.id === session.focusId}
							onclick={() => session && (session.focusId = t.id)}
							aria-label={`Ver ${t.screen ? 'tela' : 'câmera'} de ${t.name}`}
						>
							<video
								{@attach attachTrack(t.track)}
								class:mirror={t.local && !t.screen}
								autoplay
								playsinline
								muted
							></video>
							<span><Icon name={t.screen ? 'screen' : 'camera'} size={13} /> {t.name}</span>
						</button>
					{/each}
				{/if}
			</aside>
		{/if}

		{#if peopleOpen}
			<aside class="people">
				<h3>Na sala ({session.people.length})</h3>
				{#each session.people as p (p.identity)}
					<div class="person">
						<span class="dot" style:background={QUALITY_COLOR[p.quality]} title="Conexão"></span>
						<span class="pname">{p.name}{p.local ? ' (você)' : ''}</span>
						<span class="media-indicators">
							{#if p.screen}<span role="img" aria-label="Compartilhando tela"><Icon name="screen" size={15} /></span>{/if}
							{#if p.camera}<span role="img" aria-label="Câmera ligada"><Icon name="camera" size={15} /></span>{/if}
							{#if p.mic}<span role="img" aria-label="Microfone ligado"><Icon name="mic" size={15} /></span>{/if}
						</span>
					</div>
					{#if p.screenAudio && !p.local}
						<input
							type="range"
							min="0"
							max="1"
							step="0.05"
							value={session.volumes[p.identity] ?? 1}
							oninput={(e) => session?.setScreenVolume(p.identity, +e.currentTarget.value)}
							title="Volume do áudio da tela"
							aria-label={`Volume do áudio da tela de ${p.name}`}
						/>
					{/if}
				{/each}
			</aside>
		{/if}

		<nav class="controls" aria-label="Controles da sala" class:hidden={!controls} onfocusin={poke} onmouseenter={() => clearTimeout(hideTimer)}>
			<div class="menu-anchor">
				<button
					class:on={session.screenOn}
					onclick={() => (session?.screenOn ? toggleScreen() : (menu = !menu))}
					title="Ctrl+Shift+S"
					aria-label={session.screenOn ? 'Parar de compartilhar tela' : 'Compartilhar tela'}
					aria-pressed={session.screenOn}
				>
					<Icon name="screen" /> Tela
				</button>
				{#if menu}
					<div class="menu">
						{#each Object.entries(QUALITY) as [m, q] (m)}
							<button onclick={() => toggleScreen(m as QualityMode)}>{q.label}</button>
						{/each}
					</div>
				{/if}
			</div>
			<button class:on={session.cameraOn} onclick={toggleCamera} title="Ctrl+Shift+C" aria-pressed={session.cameraOn} aria-label={session.cameraOn ? 'Desligar câmera' : 'Ligar câmera'}><Icon name="camera" /> Câmera</button>
			<button class:on={session.micOn} onclick={toggleMic} aria-pressed={session.micOn} aria-label={session.micOn ? 'Desligar microfone' : 'Ligar microfone'}><Icon name={session.micOn ? 'mic' : 'mic-off'} /> Mic</button>
			<button class:on={peopleOpen} aria-label={`Participantes: ${session.people.length}`} aria-expanded={peopleOpen} onclick={() => (peopleOpen = !peopleOpen)}>
				<Icon name="users" /> {session.people.length}
			</button>
			<button onclick={copyCode} aria-label={`Copiar código da sala ${code}`}><Icon name="copy" /><span class="mono">{code}</span></button>
			<button onclick={toggleFullscreen} title="F / duplo clique" aria-label="Alternar tela cheia"><Icon name="fullscreen" /></button>
			<button onclick={() => (settingsOpen = true)} aria-label="Configurações do servidor"><Icon name="settings" /></button>
			<button class="leave" onclick={leave}><Icon name="exit" /> Sair</button>
		</nav>

		{#if toast}<div class="toast" role="status">{toast}</div>{/if}
		{#if settingsNotice}<div class="settings-notice" role="status">{settingsNotice}<button aria-label="Fechar aviso" onclick={() => (settingsNotice = '')}><Icon name="close" size={16} /></button></div>{/if}
	</div>
{:else}
	<div class="center"><p>Conectando…</p></div>
{/if}

{#if settingsOpen}
	<dialog
		class="settings-dialog"
		aria-label="Configurações do servidor"
		{@attach (el: HTMLDialogElement) => { el.showModal(); }}
		oncancel={() => (settingsOpen = false)}
		onclose={() => (settingsOpen = false)}
	>
		{#if session}<p class="settings-context">As alterações serão usadas na próxima entrada. A conexão desta sala será mantida.</p>{/if}
		<ServerSettings onsaved={savedSettings} oncancel={() => (settingsOpen = false)} />
	</dialog>
{/if}

<style>
	.center {
		min-height: 100%;
		display: grid;
		place-items: center;
		padding: 20px;
	}
	.card {
		display: grid;
		gap: 20px;
		width: min(400px, 100%);
		background: var(--card);
		padding: 28px;
		border-radius: 20px;
		border: 1px solid var(--border);
	}
	h1 {
		margin: 0;
		font-size: 25px;
		font-weight: 650;
		letter-spacing: -0.04em;
	}
	.setup-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.brand {
		font-size: 28px;
		font-weight: 750;
		letter-spacing: -0.07em;
	}
	.brand span {
		color: var(--accent);
	}
	.setup-help, .settings-context {
		color: var(--muted);
		font-size: 13px;
		line-height: 1.6;
		margin: 8px 0 0;
	}
	.setup-help strong {
		color: var(--fg);
		font-weight: 500;
	}
	.card label {
		display: grid;
		gap: 8px;
		font-size: 13px;
	}
	.card input {
		min-height: 44px;
	}
	button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
	}
	button:focus-visible, input:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 3px;
	}
	.icon-button {
		width: 38px;
		height: 38px;
		padding: 0;
	}
	.error, .notice {
		margin: 0;
		padding: 12px;
		border: 1px solid var(--border);
		border-radius: 10px;
		font-size: 13px;
		line-height: 1.6;
	}
	.error {
		color: var(--danger);
	}
	.notice {
		color: var(--ok);
	}
	.settings-dialog {
		width: min(440px, calc(100% - 32px));
		max-height: calc(100dvh - 32px);
		overflow-y: auto;
		padding: 24px;
		border: 1px solid var(--border);
		border-radius: 20px;
		background: var(--card);
		color: var(--fg);
		box-shadow: 0 24px 80px #0008;
	}
	.settings-dialog::backdrop {
		background: #08080db3;
	}
	.settings-context {
		margin: 0 0 20px;
	}
	.settings-notice {
		position: fixed;
		top: 16px;
		left: 50%;
		translate: -50% 0;
		z-index: 12;
		display: flex;
		align-items: center;
		gap: 12px;
		width: min(520px, calc(100% - 32px));
		padding: 12px 16px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--card);
		font-size: 13px;
		line-height: 1.5;
	}
	.media-indicators, .thumb span {
		display: inline-flex;
		align-items: center;
		gap: 5px;
	}
	.handle span {
		display: flex;
	}
	.rotate {
		transform: rotate(180deg);
	}
	.mono {
		font-family: ui-monospace, monospace;
		letter-spacing: 0.1em;
	}
	.big {
		font-size: 28px;
		font-weight: 600;
	}
	.viewer {
		position: fixed;
		inset: 0;
		display: grid;
		grid-template: 1fr auto / 1fr auto;
		background: #000;
		color: #eee;
	}
	.cursor-hidden {
		cursor: none;
	}
	.stage {
		position: relative;
		grid-area: 1 / 1;
		min-height: 0;
		display: grid;
		place-items: center;
		overflow: hidden;
	}
	.stage video {
		width: 100%;
		height: 100%;
		object-fit: contain;
	}
	.mirror {
		transform: scaleX(-1);
	}
	.tag {
		position: absolute;
		left: 12px;
		top: 12px;
		background: #0008;
		padding: 4px 10px;
		border-radius: 8px;
		font-size: 12px;
	}
	.empty {
		text-align: center;
		color: #aaa;
	}
	.banner {
		position: absolute;
		top: 12px;
		left: 50%;
		translate: -50% 0;
		background: #000c;
		color: #fff;
		padding: 8px 14px;
		border-radius: 10px;
		display: flex;
		gap: 10px;
		align-items: center;
	}
	.banner.audio {
		top: 56px;
		border: 1px solid var(--accent);
	}
	.strip {
		grid-area: 2 / 1;
		display: flex;
		gap: 8px;
		padding: 8px;
		overflow-x: auto;
		background: #111;
		position: relative;
	}
	.strip.closed {
		padding: 0;
	}
	.handle {
		position: absolute;
		right: 8px;
		top: -28px;
		padding: 2px 10px;
		background: #000a;
		border: none;
		color: #fff;
	}
	.thumb {
		position: relative;
		flex: 0 0 auto;
		width: 192px;
		aspect-ratio: 16 / 9;
		padding: 0;
		overflow: hidden;
		background: #222;
		border: 2px solid transparent;
	}
	.thumb.active {
		border-color: var(--accent);
	}
	.thumb video {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.thumb span {
		position: absolute;
		left: 6px;
		bottom: 4px;
		font-size: 11px;
		background: #0009;
		padding: 1px 6px;
		border-radius: 6px;
		color: #fff;
	}
	.people {
		grid-area: 1 / 2 / 3 / 3;
		width: 240px;
		background: #141418;
		padding: 12px;
		overflow-y: auto;
		display: grid;
		align-content: start;
		gap: 8px;
	}
	.people h3 {
		margin: 0 0 4px;
		font-size: 13px;
		color: #aaa;
	}
	.person {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.pname {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}
	.controls {
		position: fixed;
		bottom: 16px;
		left: 50%;
		translate: -50% 0;
		display: flex;
		gap: 6px;
		flex-wrap: wrap;
		justify-content: center;
		width: max-content;
		max-width: calc(100% - 24px);
		padding: 8px;
		background: #18181ee6;
		border: 1px solid #333;
		border-radius: 14px;
		transition: opacity 0.2s;
		z-index: 10;
	}
	.controls.hidden {
		opacity: 0;
		pointer-events: none;
	}
	.controls.hidden:focus-within {
		opacity: 1;
		pointer-events: auto;
	}
	.controls:focus-within {
		cursor: default;
	}
	.controls button {
		background: #26262e;
		border-color: #333;
		color: #eee;
		white-space: nowrap;
	}
	.controls button.on {
		background: var(--accent);
		border-color: var(--accent);
	}
	.controls .leave {
		background: #a3243b;
		border-color: #a3243b;
	}
	.menu-anchor {
		position: relative;
	}
	.menu {
		position: absolute;
		bottom: calc(100% + 8px);
		left: 0;
		display: grid;
		gap: 4px;
		padding: 6px;
		background: #18181e;
		border: 1px solid #333;
		border-radius: 12px;
		width: max-content;
	}
	.toast {
		position: fixed;
		bottom: 80px;
		left: 50%;
		translate: -50% 0;
		background: #000d;
		color: #fff;
		padding: 6px 12px;
		border-radius: 8px;
		z-index: 11;
	}
	@media (max-width: 480px) {
		.center {
			padding: 16px;
		}
		.card {
			padding: 22px;
		}
		.controls {
			bottom: 12px;
			gap: 4px;
			padding: 6px;
		}
		.controls button {
			padding: 9px;
			font-size: 12px;
		}
		.people {
			width: 180px;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.controls {
			transition: none;
		}
	}
</style>
