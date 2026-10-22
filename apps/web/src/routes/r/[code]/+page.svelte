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
	/** Per-person volume popover, opened from a grid cell or a strip thumbnail. */
	let volumeMenu = $state<{ identity: string; x: number; y: number } | null>(null);
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
		hideTimer = setTimeout(() => !menu && !micMenu && !settingsOpen && !volumeMenu && !!focus && (controls = false), 3000);
	}

	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape' && volumeMenu) {
			volumeMenu = null;
			return;
		}
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
	// Discord-style: without a shared screen the stage is a grid of everyone in the room.
	let grid = $derived(!!session && !session.tiles.some((t) => t.screen));
	let gridCols = $derived.by(() => {
		const n = session?.people.length ?? 1;
		return n <= 1 ? 1 : n <= 4 ? 2 : n <= 9 ? 3 : 4;
	});
	const cameraOf = (identity: string) => session?.tiles.find((t) => t.id === `${identity}:camera`);
	let volumePerson = $derived(volumeMenu && session?.people.find((p) => p.identity === volumeMenu?.identity));

	/** Opens the volume popover near the click (or the button, from the keyboard). */
	function openVolume(identity: string, e: MouseEvent) {
		e.preventDefault();
		e.stopPropagation();
		const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
		const x = e.detail ? e.clientX : r.left + r.width / 2;
		const y = e.detail ? e.clientY : r.top + r.height / 2;
		volumeMenu = { identity, x: Math.min(x, innerWidth - 272), y: Math.min(y, innerHeight - 190) };
	}

	function closeVolumeOutside(e: PointerEvent) {
		poke();
		if (volumeMenu && !(e.target as HTMLElement).closest('.volume-menu, .vol-btn')) volumeMenu = null;
	}
</script>

<svelte:window onkeydown={onKey} onmousemove={poke} onpointerdown={closeVolumeOutside} />

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
				<button class="icon-button" onclick={() => (settingsOpen = true)} aria-label="Configurações"><Icon name="settings" /></button>
			</div>
		</header>
		<main class="room-content">
		<section class="stage" ondblclick={toggleFullscreen} role="presentation">
			{#if grid}
				<div class="people-grid" style:--cols={gridCols}>
					{#each session.people as p (p.identity)}
						{@const cam = cameraOf(p.identity)}
						<div
							class="cell"
							class:speaking={p.speaking}
							class:remote={!p.local}
							role="presentation"
							title={p.local ? undefined : 'Clique direito para ajustar o volume'}
							oncontextmenu={(e) => !p.local && openVolume(p.identity, e)}
						>
							{#if cam}
								{#key cam.track}
									<video {@attach attachTrack(cam.track)} class:mirror={cam.local} autoplay playsinline muted></video>
								{/key}
							{:else}
								<span class="cell-avatar" aria-hidden="true">{p.name.trim().slice(0, 2).toLocaleUpperCase()}</span>
							{/if}
							<span class="cell-name">
								<span class="q" style:background={QUALITY_COLOR[p.quality]} title="Qualidade da conexão"></span>
								{#if !p.mic}<Icon name="mic-off" size={13} />{/if}
								{p.name}{p.local ? ' (você)' : ''}{#if p.speaking}<span class="sr-only">, falando</span>{/if}
							</span>
							{#if !p.local}
								<button class="vol-btn" aria-label={`Volume de ${p.name}`} onclick={(e) => openVolume(p.identity, e)}><Icon name="volume" size={16} /></button>
							{/if}
						</div>
					{/each}
				</div>
			{:else if focus}
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

		{#if !grid}
			<aside class="strip" class:closed={!stripOpen} aria-label="Transmissões e participantes">
				<button class="handle" aria-label={stripOpen ? 'Recolher miniaturas' : 'Mostrar miniaturas'} aria-expanded={stripOpen} onclick={() => (stripOpen = !stripOpen)}>
					<span class:rotate={!stripOpen}><Icon name="chevron-down" /></span>
				</button>
				{#if stripOpen}
					{#each session.tiles.filter((t) => t.screen) as t (t.id)}
						<button
							class="thumb"
							class:active={t.id === session.focusId}
							onclick={() => session && (session.focusId = t.id)}
							aria-label={`Ver tela de ${t.name}`}
						>
							<video {@attach attachTrack(t.track)} autoplay playsinline muted></video>
							<span class="thumb-name"><Icon name="screen" size={13} /> {t.name}</span>
						</button>
					{/each}
					<!-- Everyone in the room: camera or initials, green while talking. -->
					{#each session.people as p (p.identity)}
						{@const cam = cameraOf(p.identity)}
						<div class="thumb-wrap" role="presentation" oncontextmenu={(e) => !p.local && openVolume(p.identity, e)}>
							<button
								class="thumb"
								class:active={!!cam && cam.id === session.focusId}
								class:speaking={p.speaking}
								onclick={(e) => (cam ? session && (session.focusId = cam.id) : !p.local && openVolume(p.identity, e))}
								aria-label={cam ? `Ver câmera de ${p.name}` : p.local ? `${p.name} (você)` : `Volume de ${p.name}`}
							>
								{#if cam}
									<video {@attach attachTrack(cam.track)} class:mirror={cam.local} autoplay playsinline muted></video>
								{:else}
									<span class="thumb-avatar" aria-hidden="true">{p.name.trim().slice(0, 2).toLocaleUpperCase()}</span>
								{/if}
								<span class="thumb-name">
									<span class="q" style:background={QUALITY_COLOR[p.quality]}></span>
									{#if !p.mic}<Icon name="mic-off" size={12} />{/if}
									{p.name}{p.local ? ' (você)' : ''}
								</span>
							</button>
							{#if !p.local}
								<button class="vol-btn" aria-label={`Volume de ${p.name}`} onclick={(e) => openVolume(p.identity, e)}><Icon name="volume" size={14} /></button>
							{/if}
						</div>
					{/each}
				{/if}
			</aside>
		{/if}

		{#if volumeMenu && volumePerson}
			<div class="volume-menu" role="dialog" aria-label={`Volume de ${volumePerson.name}`} style:left="{volumeMenu.x}px" style:top="{volumeMenu.y}px">
				<p class="vm-name">{volumePerson.name}</p>
				<label class="volume-label">Voz · {Math.round((session.voiceVolumes[volumePerson.identity] ?? 1) * 100)}%
					<input
						type="range"
						min="0"
						max="2"
						step="0.05"
						value={session.voiceVolumes[volumePerson.identity] ?? 1}
						oninput={(e) => volumePerson && session?.setVoiceVolume(volumePerson.identity, +e.currentTarget.value)}
						aria-label={`Volume da voz de ${volumePerson.name}`}
					/>
				</label>
				{#if volumePerson.screenAudio}
					<label class="volume-label">Áudio da tela · {Math.round((session.volumes[volumePerson.identity] ?? 1) * 100)}%
						<input
							type="range"
							min="0"
							max="1"
							step="0.05"
							value={session.volumes[volumePerson.identity] ?? 1}
							oninput={(e) => volumePerson && session?.setScreenVolume(volumePerson.identity, +e.currentTarget.value)}
							aria-label={`Volume do áudio da tela de ${volumePerson.name}`}
						/>
					</label>
				{/if}
			</div>
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
		aria-label={session ? 'Configurações' : 'Configurações do servidor'}
		{@attach (el: HTMLDialogElement) => { el.showModal(); }}
		oncancel={() => (settingsOpen = false)}
		onclose={() => (settingsOpen = false)}
	>
		{#if session}
			<section class="room-prefs">
				<h2>Sala</h2>
				<label class="notify-toggle">
					<input type="checkbox" bind:checked={notifyOn} onchange={() => (settings.notify = notifyOn)} />
					Avisar entradas, saídas e telas
				</label>
			</section>
			<p class="settings-context">As alterações do servidor serão usadas na próxima entrada. A conexão desta sala será mantida.</p>
		{/if}
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
	.thumb-name {
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
	/* Idle: hide the cursor everywhere in the room, over the app's branded cursors too. */
	.viewer.cursor-hidden, .viewer.cursor-hidden :global(*) { cursor: none !important; }
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
	.stage { container-type: size; }
	.stage:has(> video) { background: #000; }
	/* Absolute: a % height inside the grid cell resolves to auto, so the video
	   kept its 16:9 height and the stage clipped it on wide screens. */
	.stage video { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; }
	.people-grid {
		position: absolute; inset: 12px; display: grid; gap: 12px;
		grid-template-columns: repeat(var(--cols), minmax(0, 1fr)); grid-auto-rows: minmax(0, 1fr);
	}
	/* Stack only when the stage is portrait; side by side otherwise (Discord-like). */
	@container (max-aspect-ratio: 3/4) { .people-grid { grid-template-columns: minmax(0, 1fr); } }
	.cell {
		position: relative; display: grid; place-items: center; overflow: hidden; min-height: 0;
		container-type: size;
		background: var(--card); border: 2px solid transparent; border-radius: 12px;
		transition: border-color 0.15s, box-shadow 0.15s;
	}
	.cell.speaking { border-color: var(--ok); box-shadow: 0 0 18px color-mix(in srgb, var(--ok) 30%, transparent); }
	.q { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
	/* Volume button: appears on hover/focus (Discord-style), always reachable by keyboard. */
	.vol-btn {
		position: absolute; top: 8px; right: 8px; z-index: 1; width: 30px; height: 30px; padding: 0;
		background: #000a; color: #fff; border: none; border-radius: 8px; opacity: 0; transition: opacity 0.15s;
	}
	.cell:hover .vol-btn, .thumb-wrap:hover .vol-btn, .vol-btn:focus { opacity: 1; }
	.thumb-wrap { position: relative; flex: 0 0 auto; display: flex; }
	.thumb-wrap .vol-btn { top: 6px; right: 6px; width: 26px; height: 26px; }
	.thumb.speaking { border-color: var(--ok); }
	.thumb-avatar {
		position: absolute; inset: 0; margin: auto; display: grid; place-items: center; width: 44px; height: 44px; border-radius: 13px;
		background: color-mix(in srgb, var(--accent) 16%, var(--card)); color: var(--fg); font-size: 13px; font-weight: 600;
	}
	.volume-menu {
		position: fixed; z-index: 20; display: grid; gap: 10px; width: 260px; padding: 14px 16px 4px;
		background: var(--card); border: 1px solid var(--border); border-radius: 14px; box-shadow: 0 16px 48px #0006;
	}
	.vm-name { margin: 0; font-size: 13px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.room-prefs { margin-bottom: 22px; padding-bottom: 18px; border-bottom: 1px solid var(--border); }
	.room-prefs h2 { margin: 0 0 12px; font-size: 15px; font-weight: 600; }
	.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
	.cell video { object-fit: cover; }
	.cell-avatar {
		/* Sized from the cell's smaller side so short cells never clip it. */
		display: grid; place-items: center; width: min(112px, 42cqmin); aspect-ratio: 1; border-radius: 28%;
		background: color-mix(in srgb, var(--accent) 16%, var(--card)); color: var(--fg);
		font-size: clamp(12px, 13cqmin, 34px); font-weight: 600;
	}
	.cell-name {
		position: absolute; left: 10px; bottom: 10px; display: flex; align-items: center; gap: 5px;
		max-width: calc(100% - 20px); overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
		background: #000a; color: #fff; padding: 4px 9px; border-radius: 7px; font-size: 12px;
	}
	.mirror { transform: scaleX(-1); }
	.tag {
		position: absolute; left: 16px; top: 16px;
		display: flex; align-items: center; gap: 8px;
		background: #111b; color: #fff;
		padding: 8px 12px; border: 1px solid #ffffff20; border-radius: 8px; font-size: 12px;
	}
	.tag > span { color: #ccc; margin-left: 8px; }
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
	.thumb-name { position: absolute; left: 6px; bottom: 6px; max-width: calc(100% - 12px); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; background: #000b; padding: 3px 6px; border-radius: 5px; color: #fff; }
	.notify-toggle { display: flex; align-items: center; gap: 8px; color: var(--muted); font-size: 13px; cursor: pointer; }
	/* Undo the global text-input style (width: 100%, padding) for the checkbox. */
	.notify-toggle input { flex: none; width: 14px; height: 14px; padding: 0; margin: 0; accent-color: var(--accent); }
	.avatar {
		position: relative; width: 36px; height: 36px; flex-shrink: 0; display: grid; place-items: center;
		border-radius: 12px; background: color-mix(in srgb, var(--accent) 12%, var(--card)); color: var(--fg); font-size: 11px; font-weight: 600;
	}
	.avatar.small { width: 34px; height: 34px; border-radius: 10px; }
	.local-state small { display: block; font-size: 11px; color: var(--muted); margin-top: 4px; }
	/* Sliders are not text fields: drop the global input box (padding, border, background). */
	.volume-menu input[type='range'] { width: 100%; margin: 0; padding: 0; border: none; background: none; accent-color: var(--accent); }
	.room-footer { display: flex; align-items: center; justify-content: space-between; gap: 20px; min-height: 88px; padding: 12px 28px; border-top: 1px solid var(--border); }
	.local-state { display: flex; align-items: center; gap: 10px; min-width: 0; font-size: 12px; }
	.local-state > span:last-child { max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.controls { display: flex; align-items: center; gap: 8px; }
	.controls button { min-height: 44px; padding: 10px 14px; background: var(--card); border-color: var(--border); font-size: 12px; white-space: nowrap; border-radius: 10px; }
	.controls button.on { background: color-mix(in srgb, var(--accent) 18%, var(--card)); border-color: var(--accent); }
	.controls .leave { background: color-mix(in srgb, var(--danger) 12%, var(--card)); border-color: color-mix(in srgb, var(--danger) 35%, var(--border)); color: var(--danger); }
	.control-divider { height: 24px; width: 1px; background: var(--border); margin: 0 4px; }
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
		.controls button > span { display: none; }
		.control-divider { margin: 0; }
		.thumb { width: 128px; }
	}
	.volume-label { display: grid; gap: 8px; color: var(--muted); font-size: 11px; padding-bottom: 12px; }
	@media (prefers-reduced-motion: reduce) {
		.room-header, .room-footer, .cell {
			transition: none;
		}
	}
</style>
