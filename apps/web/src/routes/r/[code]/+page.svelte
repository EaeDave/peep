<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { ConnectionQuality, type Track } from 'livekit-client';
	import { joinRoom, reportStats } from '#lib/api.ts';
	import { absorbUrlParams, maskCode, normalizeServerUrl, settings } from '#lib/config.ts';
	import { QUALITY, type QualityMode, Session } from '#lib/livekit/session.svelte.ts';
	import type { NoiseMode } from '#lib/livekit/noiseFilter.ts';
	import { isTauri, leaveViewer } from '#lib/platform.ts';
	import { askNotificationPermission, chime, type NoticeKind, systemNotify } from '#lib/notify.ts';
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
	let micMenu = $state(false);
	let stripOpen = $state(true);
	// Open by default at any width; below 640px it overlays the video (closable).
	let peopleOpen = $state(true);
	let notifyOn = $state(settings.notify);
	let controls = $state(true);
	let hideTimer: ReturnType<typeof setTimeout>;
	let statsTimer: ReturnType<typeof setInterval>;

	onMount(() => {
		absorbUrlParams();
		name = settings.name;
		if (!validConfig()) settingsOpen = true;
		else if (name) connect();
		poke();
		askNotificationPermission();
		// Quality sample every 15 s while connected; read server-side to diagnose drops.
		statsTimer = setInterval(async () => {
			const s = session;
			if (s?.status !== 'connected') return;
			const sample = await s.sampleStats().catch(() => null);
			if (sample)
				reportStats(code, { name: settings.name, client: isTauri ? 'desktop' : 'web', noise: s.micOn ? s.noiseActive : null, ...sample });
		}, 15_000);
	});

	onDestroy(() => {
		destroyed = true;
		clearTimeout(hideTimer);
		clearInterval(statsTimer);
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
			next.onNotice = notice;
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

	const NOTICE_TEXT: Record<NoticeKind, string> = {
		join: 'entrou na sala',
		leave: 'saiu da sala',
		share: 'começou a compartilhar a tela'
	};

	function notice(kind: NoticeKind, who: string) {
		if (!notifyOn) return;
		const text = `${who} ${NOTICE_TEXT[kind]}`;
		flash(text);
		chime(kind);
		void systemNotify(`Peep · ${code}`, text);
	}

	function flash(msg: string) {
		toast = msg;
		// Long warnings (e.g. a filter fallback) need time to be read.
		setTimeout(() => toast === msg && (toast = ''), Math.min(8000, 2000 + msg.length * 50));
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
			const warning = await session?.setScreen(!session.screenOn, m);
			if (warning) flash(warning);
		});
	const toggleCamera = () => act(async () => void (await session?.setCamera(!session.cameraOn)));
	const toggleMic = () =>
		act(async () => {
			const warning = await session?.setMic(!session.micOn);
			if (warning) flash(warning);
		});
	const NOISE_MODES: { mode: NoiseMode; label: string }[] = [
		{ mode: 'strong', label: 'Forte (IA)' },
		{ mode: 'light', label: 'Leve' },
		{ mode: 'off', label: 'Desligada' }
	];
	const pickNoise = (mode: NoiseMode) =>
		act(async () => {
			micMenu = false;
			const warning = await session?.setNoiseMode(mode);
			if (warning) flash(warning);
		});
	const openMicMenu = () =>
		act(async () => {
			micMenu = !micMenu;
			if (micMenu) await session?.loadMics();
		});
	const pickMic = (id: string) =>
		act(async () => {
			micMenu = false;
			await session?.setMicDevice(id);
		});

	async function copyCode() {
		try {
			await navigator.clipboard.writeText(code);
			flash('Código copiado');
		} catch {
			flash(`Não foi possível copiar. Código da sala: ${code}`);
		}
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
		hideTimer = setTimeout(() => !menu && !micMenu && !settingsOpen && !!focus && (controls = false), 3000);
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

<svelte:window onkeydown={onKey} onmousemove={poke} onpointerdown={poke} />

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
		<header class="room-header" class:quiet={!controls} onfocusin={poke}>
			<span class="brand">peep<span>.</span></span>
			<div class="room-identity">
				<span class="room-label">Sua sala</span>
				<button class="room-code" onclick={copyCode} aria-label={`Copiar código da sala ${code}`}>
					<span class="mono">{code}</span><Icon name="copy" size={15} />
				</button>
			</div>
			<div class="room-tools">
				<span class="connection"><span class="dot" class:connected={session.status === 'connected'}></span>{session.status === 'connected' ? 'Conectado' : session.status === 'connecting' ? 'Conectando' : session.status === 'reconnecting' ? 'Reconectando' : 'Desconectado'}</span>
				<button class="icon-button" onclick={toggleFullscreen} title="Tela cheia (F)" aria-label="Alternar tela cheia"><Icon name="fullscreen" /></button>
				<button class="icon-button" onclick={() => (settingsOpen = true)} aria-label="Configurações do servidor"><Icon name="settings" /></button>
			</div>
		</header>
		<main class="room-content" class:with-people={peopleOpen}>
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
				<span class="tag"><Icon name={focus.screen ? 'screen' : 'camera'} size={14} /> {focus.name}<span>{focus.screen ? 'Tela compartilhada' : 'Câmera'}</span></span>
			{:else}
				<div class="empty">
					<div class="empty-screen"><Icon name="screen" size={42} /><span class="empty-camera"><Icon name="camera" size={20} /></span></div>
					<h1>A sala está pronta.</h1>
					<p>Compartilhe sua tela ou ligue a câmera.<br />O próximo momento acontece aqui.</p>
					<button class="invite-code" onclick={copyCode} aria-label={`Copiar código da sala ${code}`}>
						<span><span class="invite-label">Convide com este código</span><strong class="mono">{code}</strong></span>
						<Icon name="copy" size={20} />
					</button>
					<span class="empty-note">Seu microfone começa desligado.</span>
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
			<aside class="strip" class:closed={!stripOpen} aria-label="Transmissões da sala">
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
				<div class="people-heading"><h2>Na sala <span>{session.people.length}</span></h2><button class="icon-button" aria-label="Fechar participantes" onclick={() => (peopleOpen = false)}><Icon name="close" size={18} /></button></div>
				<label class="notify-toggle">
					<input type="checkbox" bind:checked={notifyOn} onchange={() => (settings.notify = notifyOn)} />
					Avisar entradas, saídas e telas
				</label>
				{#each session.people as p (p.identity)}
					<div class="person">
						<span class="avatar" aria-hidden="true">{p.name.trim().slice(0, 2).toLocaleUpperCase()}<span class="dot" style:background={QUALITY_COLOR[p.quality]}></span></span>
						<span class="pname">{p.name}<small>{p.local ? 'Você' : 'Participante'}</small></span>
						<span class="media-indicators">
							{#if p.screen}<span role="img" aria-label="Compartilhando tela"><Icon name="screen" size={15} /></span>{/if}
							{#if p.camera}<span role="img" aria-label="Câmera ligada"><Icon name="camera" size={15} /></span>{/if}
							<span role="img" aria-label={p.mic ? 'Microfone ligado' : 'Microfone desligado'}><Icon name={p.mic ? 'mic' : 'mic-off'} size={15} /></span>
						</span>
					</div>
					{#if p.voice && !p.local}
						<label class="volume-label">Voz · {Math.round((session.voiceVolumes[p.identity] ?? 1) * 100)}%
						<input
							type="range"
							min="0"
							max="2"
							step="0.05"
							value={session.voiceVolumes[p.identity] ?? 1}
							oninput={(e) => session?.setVoiceVolume(p.identity, +e.currentTarget.value)}
							aria-label={`Volume da voz de ${p.name}`}
						/>
						</label>
					{/if}
					{#if p.screenAudio && !p.local}
						<label class="volume-label">Áudio da tela
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
						</label>
					{/if}
				{/each}
			</aside>
		{/if}
		</main>
		<footer class="room-footer" class:quiet={!controls} onfocusin={poke} onmouseenter={() => clearTimeout(hideTimer)} onmouseleave={poke}>
		<div class="local-state"><span class="avatar small" aria-hidden="true">{name.trim().slice(0, 2).toLocaleUpperCase()}</span><span>{name}<small>{session.micOn ? 'Microfone ligado' : 'Microfone desligado'}</small></span></div>
		<nav class="controls" aria-label="Controles da sala">
			<div class="menu-anchor">
				<button
					class:on={session.screenOn}
					onclick={() => (session?.screenOn ? toggleScreen() : (menu = !menu))}
					title="Compartilhar tela (Ctrl+Shift+S)"
					aria-label={session.screenOn ? 'Parar de compartilhar tela' : 'Compartilhar tela'}
					aria-pressed={session.screenOn}
				>
					<Icon name="screen" size={20} /><span>{session.screenOn ? 'Parar tela' : 'Compartilhar'}</span>{#if !session.screenOn}<Icon name="chevron-down" size={13} />{/if}
				</button>
				{#if menu}
					<div class="menu">
						<p>Compartilhar sua tela</p>
						{#each Object.entries(QUALITY) as [m, q] (m)}
							<button onclick={() => toggleScreen(m as QualityMode)}>{q.label}</button>
						{/each}
					</div>
				{/if}
			</div>
			<button class:on={session.cameraOn} onclick={toggleCamera} title="Câmera (Ctrl+Shift+C)" aria-pressed={session.cameraOn} aria-label={session.cameraOn ? 'Desligar câmera' : 'Ligar câmera'}><Icon name="camera" size={20} /><span>Câmera</span></button>
			<div class="menu-anchor mic-group">
				<button class:on={session.micOn} onclick={toggleMic} aria-pressed={session.micOn} aria-label={session.micOn ? 'Desligar microfone' : 'Ligar microfone'}><Icon name={session.micOn ? 'mic' : 'mic-off'} size={20} /><span>{session.micOn ? 'Microfone' : 'Sem áudio'}</span></button>
				<button class="mic-pick" class:on={session.micOn} onclick={openMicMenu} aria-label="Escolher microfone" title="Escolher microfone" aria-expanded={micMenu}><Icon name="chevron-down" size={13} /></button>
				{#if micMenu}
					<div class="menu mic-menu">
						<p>Microfone</p>
						{#each session.micDevices as d (d.deviceId)}
							<button aria-pressed={d.deviceId === session.micDeviceId} onclick={() => pickMic(d.deviceId)}>
								<span class="check">{#if d.deviceId === session.micDeviceId}<Icon name="check" size={14} />{/if}</span>
								<span class="label">{d.label || 'Microfone sem nome'}</span>
							</button>
						{:else}
							<p>Nenhum microfone encontrado.</p>
						{/each}
						<p>Supressão de ruído{session.micOn && session.noiseActive !== session.noiseMode ? ` (em uso: ${NOISE_MODES.find((n) => n.mode === session?.noiseActive)?.label.toLowerCase()})` : ''}</p>
						{#each NOISE_MODES as n (n.mode)}
							<button aria-pressed={n.mode === session.noiseMode} onclick={() => pickNoise(n.mode)}>
								<span class="check">{#if n.mode === session.noiseMode}<Icon name="check" size={14} />{/if}</span>
								<span class="label">{n.label}</span>
							</button>
						{/each}
					</div>
				{/if}
			</div>
			<span class="control-divider"></span>
			<button class:on={peopleOpen} aria-label={`Participantes: ${session.people.length}`} aria-expanded={peopleOpen} onclick={() => (peopleOpen = !peopleOpen)}>
				<Icon name="users" size={20} /><span class="people-count">{session.people.length}</span>
			</button>
			<button class="leave" onclick={leave}><Icon name="exit" size={20} /><span>Sair</span></button>
		</nav>
		</footer>

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
	.viewer {
		position: fixed;
		inset: 0;
		display: grid;
		grid-template-rows: 72px minmax(0, 1fr) auto;
		background: var(--bg);
		color: var(--fg);
	}
	.cursor-hidden { cursor: none; }
	.room-header {
		display: flex;
		align-items: center;
		gap: 28px;
		padding: 0 28px;
		border-bottom: 1px solid var(--border);
	}
	.room-header .brand { font-size: 29px; }
	.room-identity, .room-tools { display: flex; align-items: center; gap: 12px; }
	.room-label, .connection { color: var(--muted); font-size: 12px; }
	.room-code { background: transparent; border: none; padding: 8px; font-size: 13px; }
	.room-tools { margin-left: auto; gap: 8px; }
	.room-tools .icon-button { background: transparent; border-color: transparent; }
	.connection { display: flex; align-items: center; gap: 7px; margin-right: 14px; }
	.dot { width: 7px; height: 7px; border-radius: 50%; background: var(--muted); flex-shrink: 0; }
	.dot.connected { background: var(--ok); }
	.room-content {
		min-height: 0;
		display: grid;
		grid-template: minmax(0, 1fr) auto / minmax(0, 1fr);
		gap: 12px;
		padding: 16px 20px;
	}
	.room-content.with-people { grid-template-columns: minmax(0, 1fr) 264px; }
	.stage {
		position: relative;
		grid-area: 1 / 1;
		min-height: 0;
		display: grid;
		place-items: center;
		overflow: hidden;
		background: color-mix(in srgb, var(--card) 45%, var(--bg));
		border: 1px solid var(--border);
		border-radius: 16px;
	}
	.stage:has(video) { background: #000; }
	/* Absolute: a % height inside the grid cell resolves to auto, so the video
	   kept its 16:9 height and the stage clipped it on wide screens. */
	.stage video { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; }
	.mirror { transform: scaleX(-1); }
	.tag {
		position: absolute; left: 16px; top: 16px;
		display: flex; align-items: center; gap: 8px;
		background: #111b; color: #fff;
		padding: 8px 12px; border: 1px solid #ffffff20; border-radius: 8px; font-size: 12px;
	}
	.tag > span { color: #ccc; margin-left: 8px; }
	.empty { text-align: center; padding: 24px; max-width: 420px; }
	.empty-screen {
		position: relative; display: grid; place-items: center;
		width: 96px; height: 80px; margin: 0 auto 28px;
		color: var(--accent); background: color-mix(in srgb, var(--accent) 9%, var(--card));
		border: 1px solid color-mix(in srgb, var(--accent) 25%, var(--border)); border-radius: 20px;
	}
	.empty-camera {
		position: absolute; right: -12px; bottom: -10px; display: grid; place-items: center;
		width: 38px; height: 34px; background: var(--card); border: 1px solid var(--border); border-radius: 10px; color: var(--muted);
	}
	.empty h1 { font-size: clamp(24px, 3vw, 32px); }
	.empty p { color: var(--muted); font-size: 14px; line-height: 1.8; margin: 12px 0 26px; }
	.invite-code {
		width: 100%; max-width: 280px; margin: auto; padding: 14px 18px;
		justify-content: space-between; text-align: left; background: var(--card); border-radius: 12px;
	}
	.invite-code > span { display: grid; gap: 7px; }
	.invite-label { color: var(--muted); font-size: 11px; }
	.invite-code strong { font-size: 21px; font-weight: 550; }
	.invite-code > :global(svg) { color: var(--muted); }
	.empty-note { display: block; color: var(--muted); font-size: 11px; margin-top: 18px; }
	.banner {
		position: absolute; top: 16px; left: 50%; translate: -50% 0;
		background: var(--card); color: var(--fg); padding: 10px 16px; border: 1px solid var(--border); border-radius: 10px;
		display: flex; gap: 10px; align-items: center; font-size: 13px; max-width: calc(100% - 24px); z-index: 2;
	}
	.banner.audio { top: 68px; border-color: var(--accent); }
	.strip {
		grid-area: 2 / 1; display: flex; align-items: center; gap: 10px;
		padding: 2px; overflow-x: auto; min-width: 0;
	}
	.handle { flex: 0 0 32px; width: 32px; height: 40px; padding: 0; background: transparent; border-color: transparent; }
	.thumb {
		position: relative; flex: 0 0 auto; width: 164px; aspect-ratio: 16 / 9;
		padding: 0; overflow: hidden; background: #15151b; border: 2px solid transparent; border-radius: 10px;
	}
	.thumb.active { border-color: var(--accent); }
	.thumb video { width: 100%; height: 100%; object-fit: cover; }
	.thumb span { position: absolute; left: 6px; bottom: 6px; max-width: calc(100% - 12px); font-size: 11px; background: #000b; padding: 3px 6px; border-radius: 5px; color: #fff; }
	.people { grid-area: 1 / 2 / 3 / 3; min-height: 0; overflow-y: auto; padding: 16px; border: 1px solid var(--border); border-radius: 16px; background: var(--card); }
	.people-heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
	.people-heading h2 { margin: 0; font-size: 14px; font-weight: 600; }
	.people-heading h2 span { margin-left: 7px; color: var(--muted); font-size: 12px; font-weight: 400; }
	.people-heading button { background: transparent; border: none; width: 30px; height: 30px; }
	.notify-toggle { display: flex; align-items: center; gap: 8px; color: var(--muted); font-size: 11px; margin: 6px 0 18px; cursor: pointer; }
	.notify-toggle input { accent-color: var(--accent); margin: 0; }
	.person { display: flex; align-items: center; gap: 10px; padding: 12px 0; border-top: 1px solid var(--border); }
	.avatar {
		position: relative; width: 36px; height: 36px; flex-shrink: 0; display: grid; place-items: center;
		border-radius: 12px; background: color-mix(in srgb, var(--accent) 12%, var(--card)); color: var(--fg); font-size: 11px; font-weight: 600;
	}
	.avatar .dot { position: absolute; bottom: -1px; right: -1px; border: 2px solid var(--card); box-sizing: content-box; }
	.avatar.small { width: 34px; height: 34px; border-radius: 10px; }
	.pname { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }
	.pname small, .local-state small { display: block; font-size: 11px; color: var(--muted); margin-top: 4px; }
	.media-indicators { color: var(--muted); gap: 6px; }
	.people input[type='range'] { width: 100%; accent-color: var(--accent); }
	.room-footer { display: flex; align-items: center; justify-content: space-between; gap: 20px; min-height: 88px; padding: 12px 28px; border-top: 1px solid var(--border); }
	.local-state { display: flex; align-items: center; gap: 10px; min-width: 0; font-size: 12px; }
	.local-state > span:last-child { max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.controls { display: flex; align-items: center; gap: 8px; }
	.controls button { min-height: 44px; padding: 10px 14px; background: var(--card); border-color: var(--border); font-size: 12px; white-space: nowrap; border-radius: 10px; }
	.controls button.on { background: color-mix(in srgb, var(--accent) 18%, var(--card)); border-color: var(--accent); }
	.controls .leave { background: color-mix(in srgb, var(--danger) 12%, var(--card)); border-color: color-mix(in srgb, var(--danger) 35%, var(--border)); color: var(--danger); }
	.control-divider { height: 24px; width: 1px; background: var(--border); margin: 0 4px; }
	.people-count { font-variant-numeric: tabular-nums; }
	.menu-anchor { position: relative; }
	.menu { position: absolute; bottom: calc(100% + 12px); left: 0; display: grid; gap: 4px; padding: 8px; background: var(--card); border: 1px solid var(--border); border-radius: 14px; width: max-content; z-index: 5; box-shadow: 0 12px 40px #0004; }
	.menu p { color: var(--muted); font-size: 11px; margin: 6px 8px; }
	.menu button { border-color: transparent; justify-content: flex-start; }
	.mic-group { display: flex; }
	.controls .mic-group > button:first-child { border-top-right-radius: 0; border-bottom-right-radius: 0; }
	.controls .mic-pick { border-left: none; border-top-left-radius: 0; border-bottom-left-radius: 0; padding: 10px 8px; min-width: 0; }
	.mic-menu { left: auto; right: 0; }
	.mic-menu button { max-width: min(360px, 80vw); }
	.mic-menu .label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.mic-menu .check { width: 14px; flex-shrink: 0; display: grid; place-items: center; }
	.quiet { opacity: 0; pointer-events: none; }
	.quiet:focus-within { opacity: 1; pointer-events: auto; cursor: default; }
	.room-header, .room-footer { transition: opacity 0.2s; }
	.toast { position: fixed; bottom: 110px; left: 50%; translate: -50% 0; background: var(--card); color: var(--fg); padding: 12px 18px; border: 1px solid var(--border); border-radius: 10px; z-index: 11; max-width: calc(100% - 32px); font-size: 13px; }
	@media (max-width: 850px) {
		.local-state { display: none; }
		.room-footer { justify-content: center; }
		.room-header { padding: 0 20px; gap: 18px; }
		.room-label { display: none; }
	}
	@media (max-width: 850px) {
		.room-content.with-people { grid-template-columns: minmax(0, 1fr) 220px; }
	}
	@media (max-width: 640px) {
		.room-content.with-people { grid-template-columns: minmax(0, 1fr); }
		.people { grid-area: auto; position: fixed; right: 20px; top: 88px; bottom: 104px; width: min(280px, calc(100% - 40px)); z-index: 4; box-shadow: 0 16px 48px #0005; }
	}
	@media (max-width: 480px) {
		.center {
			padding: 16px;
		}
		.card {
			padding: 22px;
		}
		.viewer { grid-template-rows: 62px minmax(0, 1fr) auto; }
		.room-header { padding: 0 12px; gap: 12px; }
		.room-header .brand { font-size: 25px; }
		.room-code { font-size: 11px; padding: 6px 0; }
		.room-tools { gap: 0; }
		.connection { display: none; }
		.room-content { padding: 10px; gap: 8px; }
		.room-footer { padding: 12px 8px max(12px, env(safe-area-inset-bottom)); min-height: 80px; }
		.controls { gap: 6px; width: 100%; justify-content: center; }
		.controls button { padding: 10px; min-width: 42px; }
		.controls button > span:not(.people-count) { display: none; }
		.control-divider { margin: 0; }
		.thumb { width: 128px; }
		.empty { padding: 16px; }
		.empty-screen { margin-bottom: 22px; width: 80px; height: 68px; }
		.empty p { font-size: 12px; }
		.people { top: 72px; right: 10px; bottom: 90px; }
	}
	@media (max-height: 600px) {
		.empty-screen, .empty-note { display: none; }
		.empty { padding: 16px; }
		.empty p { margin: 8px 0 14px; }
		.empty h1 { font-size: 24px; }
		.invite-code { padding: 10px 16px; }
	}
	.volume-label { display: grid; gap: 8px; color: var(--muted); font-size: 11px; padding-bottom: 12px; }
	@media (prefers-reduced-motion: reduce) {
		.room-header, .room-footer {
			transition: none;
		}
	}
</style>
