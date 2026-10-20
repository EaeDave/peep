<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { ConnectionQuality, type Track } from 'livekit-client';
	import { joinRoom } from '#lib/api.ts';
	import { absorbUrlParams, maskCode, settings } from '#lib/config.ts';
	import { QUALITY, type QualityMode, Session } from '#lib/livekit/session.svelte.ts';
	import { leaveViewer } from '#lib/platform.ts';

	const code = maskCode(page.params.code ?? '');
	let session = $state<Session | null>(null);
	let needName = $state(false);
	let name = $state('');
	let secret = $state('');
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
		secret = settings.secret;
		if (name && settings.configured) connect();
		else needName = true;
		poke();
	});

	onDestroy(() => session?.room.disconnect());

	async function connect() {
		error = '';
		settings.name = name;
		if (secret) settings.secret = secret;
		needName = false;
		try {
			const { token, url } = await joinRoom(code, settings.name);
			const s = new Session();
			session = s;
			await s.connect(url, token);
		} catch (e) {
			session?.room.disconnect();
			session = null;
			error = (e as Error).message || 'Falha ao conectar';
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
		hideTimer = setTimeout(() => !menu && (controls = false), 3000);
	}

	function onKey(e: KeyboardEvent) {
		if ((e.target as HTMLElement).tagName === 'INPUT' || !session) return;
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
			<h2>Sala <span class="mono">{code}</span></h2>
			{#if error}<p class="error">{error}</p>{/if}
			<label>Nome <input bind:value={name} maxlength="24" required /></label>
			{#if !settings.configured}
				<label>
					Segredo de convite <input bind:value={secret} type="password" required />
				</label>
			{/if}
			<button class="primary" type="submit">Entrar</button>
			<button type="button" onclick={leaveViewer}>Voltar</button>
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
					🔊 Clique para ativar o áudio
				</button>
			{/if}
		</section>

		{#if session.tiles.length > 1 || (session.tiles.length === 1 && !focus)}
			<aside class="strip" class:closed={!stripOpen}>
				<button class="handle" onclick={() => (stripOpen = !stripOpen)}>
					{stripOpen ? '▾' : '▴'}
				</button>
				{#if stripOpen}
					{#each session.tiles as t (t.id)}
						<button
							class="thumb"
							class:active={t.id === session.focusId}
							onclick={() => session && (session.focusId = t.id)}
						>
							<video
								{@attach attachTrack(t.track)}
								class:mirror={t.local && !t.screen}
								autoplay
								playsinline
								muted
							></video>
							<span>{t.screen ? '🖥️' : '📷'} {t.name}</span>
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
						<span>{p.screen ? '🖥️' : ''}{p.camera ? '📷' : ''}{p.mic ? '🎙️' : ''}</span>
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
						/>
					{/if}
				{/each}
			</aside>
		{/if}

		<nav class="controls" class:hidden={!controls} onmouseenter={() => clearTimeout(hideTimer)}>
			<div class="menu-anchor">
				<button
					class:on={session.screenOn}
					onclick={() => (session?.screenOn ? toggleScreen() : (menu = !menu))}
					title="Ctrl+Shift+S"
				>
					🖥️ Tela
				</button>
				{#if menu}
					<div class="menu">
						{#each Object.entries(QUALITY) as [m, q] (m)}
							<button onclick={() => toggleScreen(m as QualityMode)}>{q.label}</button>
						{/each}
					</div>
				{/if}
			</div>
			<button class:on={session.cameraOn} onclick={toggleCamera} title="Ctrl+Shift+C">📷 Câmera</button>
			<button class:on={session.micOn} onclick={toggleMic}>{session.micOn ? '🎙️' : '🔇'} Mic</button>
			<button class:on={peopleOpen} onclick={() => (peopleOpen = !peopleOpen)}>
				👥 {session.people.length}
			</button>
			<button onclick={copyCode}><span class="mono">{code}</span></button>
			<button onclick={toggleFullscreen} title="F / duplo clique">⛶</button>
			<button class="leave" onclick={leave}>Sair</button>
		</nav>

		{#if toast}<div class="toast">{toast}</div>{/if}
	</div>
{:else}
	<div class="center"><p>Conectando…</p></div>
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
		gap: 12px;
		width: min(340px, 100%);
		background: var(--card);
		padding: 20px;
		border-radius: 14px;
		border: 1px solid var(--border);
	}
	h2 {
		margin: 0;
		font-size: 16px;
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
</style>
