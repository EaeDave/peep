<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { settings } from '#lib/config.ts';
	import Icon from '#lib/Icon.svelte';

	type Person = { identity: string; name: string; joinedAt: string; mic: boolean; camera: boolean; screen: boolean };
	type Room = { code: string; createdAt: string; people: Person[] };

	// Admin session lives only in this tab (sessionStorage); closing it logs out.
	const KEY = 'peep.adminToken';
	let token = $state('');
	let user = $state('');
	let password = $state('');
	let apiUrl = $state('');
	let rooms = $state<Room[] | null>(null);
	let error = $state('');
	let busy = $state(false);
	let pending = $state(''); // action awaiting confirmation: "close:CODE" / "kick:CODE:identity"
	let now = $state(Date.now());
	let timer: ReturnType<typeof setInterval>;

	async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
		let res: Response;
		try {
			res = await fetch(apiUrl + path, {
				method,
				headers: { ...(token ? { Authorization: 'Bearer ' + token } : {}), ...(body ? { 'Content-Type': 'application/json' } : {}) },
				body: body ? JSON.stringify(body) : undefined
			});
		} catch {
			throw new Error('Sem conexão com a API.');
		}
		if (res.status === 401 && token) {
			logout();
			throw new Error('Sessão expirada. Entre de novo.');
		}
		if (res.status === 401) throw new Error('Usuário ou senha incorretos.');
		if (res.status === 429) throw new Error('Muitas tentativas. Aguarde um minuto.');
		if (res.status === 404 && path === '/admin/login') throw new Error('Painel desativado neste servidor (ADMIN_USER/ADMIN_PASSWORD).');
		if (!res.ok) throw new Error(`Erro do servidor (${res.status}).`);
		return res.status === 204 ? (undefined as T) : res.json();
	}

	async function refresh() {
		if (!token) return;
		try {
			rooms = (await call<{ rooms: Room[] }>('GET', '/admin/rooms')).rooms;
			error = '';
		} catch (e) {
			error = (e as Error).message;
		}
	}

	async function login(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			apiUrl = apiUrl.trim().replace(/\/+$/, '');
			const r = await call<{ token: string }>('POST', '/admin/login', { user, password });
			token = r.token;
			password = '';
			sessionStorage.setItem(KEY, JSON.stringify({ token, apiUrl }));
			await refresh();
		} catch (err) {
			error = (err as Error).message;
		} finally {
			busy = false;
		}
	}

	function logout() {
		token = '';
		rooms = null;
		pending = '';
		sessionStorage.removeItem(KEY);
	}

	async function act(kind: 'close' | 'kick', code: string, identity = '') {
		const id = `${kind}:${code}:${identity}`;
		if (pending !== id) return void (pending = id); // first click arms, second confirms
		pending = '';
		busy = true;
		try {
			if (kind === 'close') await call('POST', `/admin/rooms/${encodeURIComponent(code)}/close`);
			else await call('POST', `/admin/rooms/${encodeURIComponent(code)}/kick`, { identity });
			await refresh();
		} catch (e) {
			error = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	function since(iso: string) {
		const s = Math.max(0, Math.floor((now - Date.parse(iso)) / 1000));
		if (s < 60) return `${s} s`;
		const m = Math.floor(s / 60);
		return m < 60 ? `${m} min` : `${Math.floor(m / 60)} h ${String(m % 60).padStart(2, '0')} min`;
	}

	const total = $derived(rooms?.reduce((n, r) => n + r.people.length, 0) ?? 0);

	onMount(() => {
		apiUrl = settings.apiUrl;
		try {
			const saved = JSON.parse(sessionStorage.getItem(KEY) ?? 'null');
			if (saved?.token) ({ token, apiUrl } = saved);
		} catch {
			/* no session */
		}
		void refresh();
		timer = setInterval(() => {
			now = Date.now();
			if (!busy) void refresh();
		}, 5000);
	});
	onDestroy(() => clearInterval(timer));
</script>

<svelte:head><title>Admin - Peep</title><meta name="robots" content="noindex" /></svelte:head>

<main class="admin">
	<header>
		<div class="brand"><img src="/favicon.svg" alt="" width="28" height="28" /><span>peep<span class="dot">.</span></span><small>admin</small></div>
		{#if token}
			<div class="tools">
				<span class="summary" aria-live="polite">{rooms ? `${rooms.length} ${rooms.length === 1 ? 'sala' : 'salas'} · ${total} ${total === 1 ? 'pessoa' : 'pessoas'}` : 'Carregando...'}</span>
				<button class="icon" aria-label="Atualizar" title="Atualizar" onclick={refresh} disabled={busy}><Icon name="refresh" size={16} /></button>
				<button onclick={logout}><Icon name="exit" size={16} /> Sair</button>
			</div>
		{/if}
	</header>

	{#if error}<p class="error" role="alert">{error}</p>{/if}

	{#if !token}
		<form class="login" onsubmit={login}>
			<h1>Painel administrativo</h1>
			<p>Credenciais definidas em <code>ADMIN_USER</code> e <code>ADMIN_PASSWORD</code> no servidor.</p>
			<label>URL da API <input bind:value={apiUrl} type="url" required autocomplete="off" spellcheck="false" /></label>
			<label>Usuário <input bind:value={user} required autocomplete="username" /></label>
			<label>Senha <input bind:value={password} type="password" required autocomplete="current-password" /></label>
			<button class="primary" type="submit" disabled={busy}>{busy ? 'Entrando...' : 'Entrar'}</button>
		</form>
	{:else if rooms && rooms.length === 0}
		<section class="empty"><Icon name="users" size={28} /><h2>Nenhuma sala ativa agora.</h2><p>A lista atualiza sozinha a cada 5 s.</p></section>
	{:else if rooms}
		<section class="rooms">
			{#each rooms as r (r.code)}
				<article class="room">
					<div class="room-head">
						<div>
							<strong class="code">{r.code}</strong>
							<span class="meta">aberta há {since(r.createdAt)} · {r.people.length} {r.people.length === 1 ? 'pessoa' : 'pessoas'}</span>
						</div>
						<button class="danger" class:armed={pending === `close:${r.code}:`} disabled={busy} onclick={() => act('close', r.code)}>
							{pending === `close:${r.code}:` ? 'Confirmar encerramento' : 'Encerrar sala'}
						</button>
					</div>
					{#if r.people.length}
						<ul>
							{#each r.people as p (p.identity)}
								<li>
									<span class="avatar" aria-hidden="true">{(p.name || p.identity).trim().slice(0, 2).toLocaleUpperCase()}</span>
									<span class="who"><span class="name">{p.name || p.identity}</span><small>há {since(p.joinedAt)} · {p.identity}</small></span>
									<span class="media">
										<span class:on={p.mic} title={p.mic ? 'Microfone ligado' : 'Microfone desligado'}><Icon name={p.mic ? 'mic' : 'mic-off'} size={15} /></span>
										<span class:on={p.camera} title={p.camera ? 'Câmera ligada' : 'Câmera desligada'}><Icon name="camera" size={15} /></span>
										<span class:on={p.screen} title={p.screen ? 'Compartilhando tela' : 'Sem tela'}><Icon name="screen" size={15} /></span>
									</span>
									<button class="danger small" class:armed={pending === `kick:${r.code}:${p.identity}`} disabled={busy} onclick={() => act('kick', r.code, p.identity)}>
										{pending === `kick:${r.code}:${p.identity}` ? 'Confirmar' : 'Remover'}
									</button>
								</li>
							{/each}
						</ul>
					{:else}
						<p class="meta pad">Sala vazia; o LiveKit a encerra sozinho em alguns minutos.</p>
					{/if}
				</article>
			{/each}
		</section>
	{/if}
</main>

<style>
	.admin { max-width: 880px; margin: 0 auto; padding: 24px 20px 48px; }
	header { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; padding-bottom: 18px; margin-bottom: 20px; border-bottom: 1px solid var(--border); }
	.brand { display: flex; align-items: center; gap: 9px; font-size: 22px; font-weight: 650; letter-spacing: -.6px; }
	.brand .dot { color: var(--accent); }
	.brand small { margin-left: 4px; padding: 2px 8px; border: 1px solid var(--border); border-radius: 999px; color: var(--muted); font-size: 11px; font-weight: 500; letter-spacing: 0; }
	.tools { display: flex; align-items: center; gap: 8px; }
	.summary { color: var(--muted); font-size: 13px; margin-right: 6px; }
	.tools button { display: inline-flex; align-items: center; gap: 6px; padding: 8px 12px; font-size: 13px; }
	.tools .icon { padding: 8px; }
	.error { margin: 0 0 16px; padding: 10px 14px; border: 1px solid var(--danger); border-radius: 10px; }
	.login { display: grid; gap: 14px; max-width: 360px; margin: 40px auto; padding: 24px; border: 1px solid var(--border); border-radius: 16px; background: var(--card); }
	.login h1 { margin: 0; font-size: 20px; }
	.login p { margin: 0; color: var(--muted); font-size: 12px; line-height: 1.6; }
	.login label { color: var(--fg); font-size: 12px; font-weight: 500; }
	.login input { height: 42px; font-size: 13px; }
	.empty { display: grid; justify-items: center; gap: 6px; padding: 64px 20px; color: var(--muted); text-align: center; }
	.empty h2 { margin: 8px 0 0; color: var(--fg); font-size: 17px; }
	.empty p { margin: 0; font-size: 13px; }
	.rooms { display: grid; gap: 14px; }
	.room { border: 1px solid var(--border); border-radius: 14px; background: var(--card); overflow: hidden; }
	.room-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 14px 16px; }
	.code { display: block; font: 600 18px/1.2 ui-monospace, monospace; letter-spacing: .12em; }
	.meta { color: var(--muted); font-size: 12px; }
	.pad { margin: 0; padding: 0 16px 14px; }
	ul { margin: 0; padding: 0; list-style: none; }
	li { display: flex; align-items: center; gap: 12px; padding: 10px 16px; border-top: 1px solid var(--border); }
	.avatar { display: grid; place-items: center; width: 34px; height: 34px; flex-shrink: 0; border-radius: 10px; background: color-mix(in srgb, var(--accent) 16%, var(--card)); font-size: 12px; font-weight: 600; }
	.who { flex: 1; min-width: 0; display: grid; }
	.who .name, .who small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.who .name { font-size: 14px; }
	.who small { color: var(--muted); font-size: 11px; }
	.media { display: flex; gap: 8px; color: var(--muted); opacity: .45; }
	.media span { display: inline-flex; opacity: .55; }
	.media span.on { opacity: 1; color: var(--ok); }
	.media:has(.on) { opacity: 1; }
	button.danger { color: var(--danger); border-color: color-mix(in srgb, var(--danger) 40%, var(--border)); font-size: 13px; white-space: nowrap; }
	button.danger.armed { color: #fff; background: var(--danger); border-color: var(--danger); }
	button.small { padding: 6px 10px; font-size: 12px; }
	@media (max-width: 560px) {
		.room-head { flex-direction: column; align-items: stretch; }
		.who small { display: none; }
	}
</style>
