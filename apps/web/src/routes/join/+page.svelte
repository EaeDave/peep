<script lang="ts">
	import { onMount } from 'svelte';
	import { createRoom, getRoom } from '#lib/api.ts';
	import { absorbUrlParams, isFullCode, maskCode, settings } from '#lib/config.ts';
	import { openViewer } from '#lib/platform.ts';

	let name = $state('');
	let code = $state('');
	let secret = $state('');
	let apiUrl = $state('');
	let showSetup = $state(false);
	let busy = $state(false);
	let error = $state('');

	onMount(() => {
		absorbUrlParams();
		name = settings.name;
		secret = settings.secret;
		apiUrl = settings.apiUrl;
		showSetup = !settings.configured;
	});

	function save() {
		settings.name = name;
		settings.secret = secret;
		settings.apiUrl = apiUrl;
	}

	async function run(fn: () => Promise<void>) {
		error = '';
		if (!name.trim()) return void (error = 'Informe seu nome');
		if (!secret.trim() || !apiUrl.trim()) return void ((showSetup = true), (error = 'Configure o servidor'));
		save();
		busy = true;
		try {
			await fn();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	const join = () =>
		run(async () => {
			if (!isFullCode(code)) throw new Error('Código incompleto');
			await getRoom(code);
			await openViewer(code);
		});

	const create = () =>
		run(async () => {
			const room = await createRoom(name.trim());
			await navigator.clipboard.writeText(room.code).catch(() => {});
			code = room.code;
			await openViewer(room.code);
		});

	async function paste() {
		const t = await navigator.clipboard.readText().catch(() => '');
		if (t) code = maskCode(t);
	}
</script>

<main>
	<header>
		<img src="/favicon.svg" alt="" width="28" height="28" />
		<h1>Peep</h1>
		<button class="ghost" title="Servidor" onclick={() => (showSetup = !showSetup)}>⚙</button>
	</header>

	<form
		onsubmit={(e) => {
			e.preventDefault();
			join();
		}}
	>
		<label>
			Nome
			<input bind:value={name} maxlength="24" autocomplete="nickname" placeholder="Seu nome" />
		</label>

		<label>
			Código da sala
			<div class="row">
				<input
					class="code"
					value={code}
					oninput={(e) => (code = e.currentTarget.value = maskCode(e.currentTarget.value))}
					placeholder="XXX-XXX"
					autocomplete="off"
					spellcheck="false"
				/>
				<button type="button" title="Colar" onclick={paste}>📋</button>
			</div>
		</label>

		{#if showSetup}
			<label>
				Segredo de convite
				<input bind:value={secret} type="password" autocomplete="off" />
			</label>
			<label>
				URL da API
				<input bind:value={apiUrl} placeholder="https://peep-api.exemplo.com" />
			</label>
		{/if}

		{#if error}<p class="error">{error}</p>{/if}

		<button class="primary" type="submit" disabled={busy || !isFullCode(code)}>Entrar</button>
		<button type="button" disabled={busy} onclick={create}>Criar sala</button>
	</form>
</main>

<style>
	main {
		max-width: 340px;
		margin: 0 auto;
		padding: 20px;
		display: grid;
		gap: 16px;
	}
	header {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	h1 {
		font-size: 18px;
		margin: 0;
		flex: 1;
	}
	form {
		display: grid;
		gap: 12px;
	}
	.row {
		display: flex;
		gap: 8px;
	}
	.code {
		font:
			600 18px/1 ui-monospace,
			monospace;
		letter-spacing: 0.15em;
		text-align: center;
	}
	.ghost {
		border: none;
		background: none;
		color: var(--muted);
		font-size: 18px;
		padding: 4px 8px;
	}
	p {
		margin: 0;
	}
</style>
