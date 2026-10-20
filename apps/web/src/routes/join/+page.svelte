<script lang="ts">
	import { onMount } from 'svelte';
	import { createRoom, getRoom } from '#lib/api.ts';
	import { absorbUrlParams, isFullCode, maskCode, normalizeServerUrl, settings } from '#lib/config.ts';
	import { checkUpdate, installUpdate, isTauri, onUpdateAvailable, openViewer, updateStatus } from '#lib/platform.ts';
	import Icon from '#lib/Icon.svelte';
	import ServerSettings from '#lib/ServerSettings.svelte';

	let name = $state('');
	let code = $state('');
	let showSetup = $state(false);
	let configured = $state(false);
	let serverLabel = $state('');
	let ready = $state(false);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let nameInput = $state<HTMLInputElement>();
	let codeInput = $state<HTMLInputElement>();
	let version = $state('');
	let available = $state<string | null>(null);
	let updateState = $state<'idle' | 'checking' | 'current' | 'installing' | 'error'>('idle');
	let updateError = $state('');

	function readServer() {
		configured = settings.configured;
		try {
			serverLabel = new URL(normalizeServerUrl(settings.apiUrl)).host;
		} catch {
			serverLabel = '';
			configured = false;
		}
	}

	onMount(() => {
		absorbUrlParams();
		name = settings.name;
		readServer();
		showSetup = !configured;
		ready = true;
		if (!isTauri) return;
		void updateStatus().then(([v, a]) => ((version = v), (available = a)));
		const unlisten = onUpdateAvailable((v) => (available = v));
		return () => void unlisten.then((stop) => stop());
	});

	async function refreshUpdate() {
		updateState = 'checking';
		try {
			[version, available] = await checkUpdate();
			updateState = available ? 'idle' : 'current';
		} catch {
			updateState = 'error';
			updateError = 'Não foi possível verificar.';
		}
	}

	async function applyUpdate() {
		updateState = 'installing';
		try {
			await installUpdate();
		} catch (e) {
			updateState = 'error';
			updateError = String(e) || 'Falha ao atualizar.';
		}
	}

	async function run(action: () => Promise<void>) {
		error = '';
		notice = '';
		if (!name.trim()) {
			error = 'Informe como você quer aparecer na sala.';
			nameInput?.focus();
			return;
		}
		readServer();
		if (!configured) {
			showSetup = true;
			return;
		}
		settings.name = name;
		busy = true;
		try {
			await action();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	const join = () => run(async () => {
		if (!isFullCode(code)) {
			codeInput?.focus();
			throw new Error('Use o código de 6 caracteres enviado por quem criou a sala.');
		}
		await getRoom(code);
		await openViewer(code);
	});

	const create = () => run(async () => {
		const room = await createRoom(name.trim());
		await navigator.clipboard.writeText(room.code).catch(() => {});
		code = room.code;
		await openViewer(room.code);
	});

	async function paste() {
		try {
			code = maskCode(await navigator.clipboard.readText());
			codeInput?.focus();
		} catch {
			error = 'Não foi possível ler a área de transferência. Cole o código no campo.';
			codeInput?.focus();
		}
	}
</script>

<svelte:head><title>Entrar em uma sala — Peep</title></svelte:head>

<main class="join-page" class:desktop={isTauri}>
	<section class="mini" aria-label="Peep">
		<header data-tauri-drag-region={isTauri || undefined}>
			<div class="brand" data-tauri-drag-region={isTauri || undefined}><img src="/favicon.svg" alt="" width="30" height="30" /><span>peep<span class="brand-dot">.</span></span></div>
			<button class="settings-button" type="button" aria-label="Configurações do servidor" title="Configurações do servidor"
				aria-pressed={showSetup} disabled={busy} onclick={() => { showSetup = !showSetup; error = ''; notice = ''; }}>
				<Icon name="settings" size={18} />
			</button>
		</header>

		<div class="content">
			{#if ready && showSetup}
				<ServerSettings oncancel={() => (showSetup = false)} onsaved={() => {
					readServer(); showSetup = false; error = ''; notice = 'Configurações salvas neste dispositivo.';
				}} />
			{:else if ready}
				<div class="view-intro"><h1>Entre na sala</h1><p>Um código. Sua turma. A mesma tela.</p></div>
				<form novalidate onsubmit={(event) => { event.preventDefault(); join(); }}>
					<div class="field">
						<label for="peep-name">Seu nome</label>
						<input id="peep-name" name="name" bind:this={nameInput} bind:value={name} maxlength="24"
							autocomplete="nickname" placeholder="Como você quer aparecer" />
					</div>
					<div class="field">
						<label for="peep-room-code">Código da sala</label>
						<div class="code-field">
							<input id="peep-room-code" name="code" bind:this={codeInput} class="code" value={code}
								oninput={(event) => (code = event.currentTarget.value = maskCode(event.currentTarget.value))}
								placeholder="XXX-XXX" autocomplete="off" spellcheck="false" aria-describedby="peep-code-help" />
							<button class="paste" type="button" aria-label="Colar código da sala" title="Colar código" onclick={paste} disabled={busy}><Icon name="clipboard" size={18} /></button>
						</div>
						<p id="peep-code-help" class="help">O código enviado por quem criou a sala.</p>
					</div>
					{#if error}<p class="error-message" role="alert">{error}</p>{/if}
					{#if notice}<p class="notice" role="status">{notice}</p>{/if}
					<button class="join-button" type="submit" disabled={busy}>{busy ? 'Conectando…' : 'Entrar na sala'}<Icon name="arrow-right" size={17} /></button>
					<div class="divider"><span>ou comece uma nova</span></div>
					<button class="create-button" type="button" disabled={busy} onclick={create}>Criar sala</button>
				</form>
				<div class="server-status">
					<Icon name="server" size={14} />
					<span title={configured ? 'Configuração salva neste dispositivo' : 'Configure o servidor antes de entrar'}>{configured ? serverLabel : 'Servidor não configurado'}</span>
					<button type="button" disabled={busy} onclick={() => { showSetup = true; error = ''; notice = ''; }}>Alterar</button>
				</div>
				{#if isTauri && version}
					<div class="app-version" aria-live="polite">
						{#if available && updateState !== 'error'}
							<span class="update-ready">Versão {available} disponível</span>
							<button type="button" class="update-button" disabled={updateState === 'installing'} onclick={applyUpdate}>{updateState === 'installing' ? 'Atualizando…' : 'Atualizar'}</button>
						{:else}
							<span>v{version}{updateState === 'checking' ? ' · verificando…' : updateState === 'current' ? ' · atualizado' : updateState === 'error' ? ` · ${updateError}` : ''}</span>
							<button type="button" class="check-button" aria-label="Verificar atualizações" title="Verificar atualizações" disabled={updateState === 'checking'} onclick={refreshUpdate}><Icon name="refresh" size={13} /></button>
						{/if}
					</div>
				{/if}
			{/if}
		</div>
	</section>
	{#if !isTauri}<a class="back-home" href="/"><Icon name="arrow-left" size={14} /> Sobre o Peep</a>{/if}
</main>

<style>
	.join-page { min-height: 100svh; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 40px 24px; background: var(--bg); }
	.mini { width: min(100%, 400px); border: 1px solid var(--border); border-radius: 16px; background: var(--card); box-shadow: 0 16px 64px #00000018; overflow: hidden; }
	.app-version { display: flex; align-items: center; gap: 7px; margin-top: 10px; color: var(--muted); font-size: 10px; }
	.app-version span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.app-version button { margin-left: auto; border: none; background: transparent; color: var(--muted); padding: 3px; font-size: 10px; display: grid; place-items: center; }
	.app-version button:hover:enabled { color: var(--fg); }
	.update-ready { color: var(--ok); }
	.app-version .update-button { color: var(--accent); font-weight: 600; }
	.desktop .brand > * { pointer-events: none; }
	header { display: flex; align-items: center; justify-content: space-between; padding: 22px 26px 17px; border-bottom: 1px solid var(--border); }
	.brand { display: flex; align-items: center; gap: 9px; color: var(--fg); font-size: 24px; font-weight: 650; letter-spacing: -.8px; }
	.brand img { border-radius: 9px; }
	.brand-dot { color: var(--ok); }
	.settings-button { display: grid; place-items: center; width: 34px; height: 34px; padding: 0; background: transparent; color: var(--muted); border: 1px solid transparent; border-radius: 8px; }
	.settings-button:hover, .settings-button[aria-pressed='true'] { color: var(--fg); border-color: var(--border); background: var(--bg); }
	.content { padding: 25px 26px 19px; }
	.view-intro { margin-bottom: 23px; }
	h1 { margin: 0 0 7px; font-size: 24px; line-height: 1.2; letter-spacing: -.8px; font-weight: 600; text-wrap: balance; }
	.view-intro p { margin: 0; font-size: 12px; color: var(--muted); }
	form { display: grid; gap: 19px; }
	.field { display: grid; gap: 8px; }
	label { color: var(--fg); font-size: 12px; font-weight: 500; }
	input { min-width: 0; height: 43px; padding: 10px 12px; border-radius: 8px; font-size: 13px; }
	input::placeholder { color: var(--muted); opacity: .8; }
	input:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
	.code-field { position: relative; }
	.code { padding: 12px 49px 12px 15px; height: 52px; font-family: ui-monospace, 'Cascadia Code', monospace; font-size: 20px; font-weight: 500; letter-spacing: .18em; }
	.paste { display: grid; place-items: center; position: absolute; right: 5px; top: 7px; width: 37px; height: 37px; padding: 0; border: none; background: transparent; color: var(--muted); border-radius: 6px; }
	.paste:hover { background: var(--card); color: var(--fg); }
	.help { margin: 0; font-size: 11px; color: var(--muted); }
	.join-button, .create-button { display: flex; align-items: center; justify-content: center; gap: 12px; width: 100%; min-height: 42px; border-radius: 8px; font-size: 13px; font-weight: 550; }
	.join-button { background: var(--accent); border-color: var(--accent); color: var(--accent-fg); }
	.join-button:hover:enabled { filter: brightness(1.1); }
	.create-button { background: transparent; color: var(--fg); }
	.create-button:hover:enabled { background: var(--bg); border-color: var(--muted); }
	.divider { display: flex; gap: 10px; align-items: center; font-size: 10px; color: var(--muted); }
	.divider::before, .divider::after { content: ''; height: 1px; background: var(--border); flex: 1; }
	.error-message, .notice { margin: 0; font-size: 12px; line-height: 1.5; }
	.error-message { color: var(--danger); }
	.notice { color: var(--ok); }
	.server-status { display: flex; align-items: center; gap: 7px; margin-top: 22px; padding-top: 16px; border-top: 1px solid var(--border); color: var(--muted); font-size: 10px; }
	.server-status :global(svg) { flex-shrink: 0; }
	.server-status span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.server-status button { margin-left: auto; border: none; background: transparent; padding: 3px; font-size: 10px; color: var(--muted); }
	.server-status button:hover { color: var(--fg); }
	.back-home { display: flex; align-items: center; gap: 7px; margin-top: 20px; color: var(--muted); text-decoration: none; font-size: 11px; }
	.back-home:hover { color: var(--fg); }
	button:focus-visible, a:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
	button { touch-action: manipulation; }
	.desktop { justify-content: start; padding: 0; }
	.desktop .mini { width: 100%; border: none; border-radius: 0; box-shadow: none; }
	@media (max-width: 360px) {
		.join-page { justify-content: start; padding: 0; }
		.mini { border: none; border-radius: 0; box-shadow: none; }
		header { padding: 15px 18px 12px; }
		.content { padding: 17px 18px 14px; }
		.view-intro { display: none; }
		form { gap: 12px; }
		.server-status { margin-top: 14px; padding-top: 12px; }
	}
</style>
