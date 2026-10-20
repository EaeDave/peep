<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from '#lib/Icon.svelte';

	const repo = 'https://github.com/EaeDave/peep';
	let platform = $state<'linux' | 'windows'>('linux');
	const apiUrl = import.meta.env.VITE_API_URL ?? '';
	let origin = $state('');
	let copied = $state(false);
	let copyError = $state('');
	const script = $derived(platform === 'linux' ? '/install.sh' : '/install.ps1');
	const command = $derived(platform === 'linux'
		? `curl -fsSL ${origin}/install.sh | bash -s -- --web-url '${origin}'${apiUrl ? ` --api-url '${apiUrl.replaceAll("'", "'\\''")}'` : ''}`
		: `irm ${origin}/install.ps1 | iex`);

	onMount(() => {
		origin = window.location.origin;
		if (/Windows/i.test(navigator.userAgent)) platform = 'windows';
	});

	async function copyCommand() {
		try {
			await navigator.clipboard.writeText(command);
			copied = true;
			copyError = '';
			setTimeout(() => (copied = false), 2200);
		} catch {
			copyError = 'Selecione o comando para copiar manualmente.';
		}
	}
</script>

<svelte:head>
	<title>Peep — sua tela, sua galera</title>
	<meta name="description" content="Compartilhe tela e câmera com seus amigos. Salas temporárias, código aberto e um servidor que é seu. Windows e Linux com Omarchy." />
</svelte:head>

<div class="landing">
	<div class="glow" aria-hidden="true"></div>
	<header>
		<a class="brand" href="/" aria-label="Peep, início"><img src="/favicon.svg" width="32" height="32" alt="" /> peep<span class="brand-dot">.</span></a>
		<nav aria-label="Navegação principal">
			<a class="browser-link" href="/join">Abrir no navegador</a>
			<a class="github" href={repo} target="_blank" rel="noreferrer" aria-label="Código do Peep no GitHub" title="Ver código no GitHub">
				<svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor" aria-hidden="true"><path d="M12 .9a11.1 11.1 0 0 0-3.5 21.6c.6.1.8-.2.8-.5v-2.1c-3.2.7-3.9-1.4-3.9-1.4-.5-1.3-1.3-1.6-1.3-1.6-1.1-.8.1-.8.1-.8 1.1.1 1.7 1.1 1.7 1.1 1 .1.7 2.5 3.4 1.7.1-.7.4-1.2.7-1.5-2.6-.3-5.3-1.3-5.3-5.5 0-1.2.4-2.2 1.1-3-.1-.3-.5-1.5.1-3 0 0 .9-.3 3.1 1.1a10.8 10.8 0 0 1 5.7 0c2.2-1.4 3.1-1.1 3.1-1.1.6 1.5.2 2.7.1 3 .7.8 1.1 1.8 1.1 3 0 4.3-2.7 5.2-5.3 5.5.4.4.8 1.1.8 2.1v3c0 .3.2.6.8.5A11.1 11.1 0 0 0 12 .9Z" /></svg>
			</a>
		</nav>
	</header>

	<main>
		<section class="intro" aria-labelledby="headline">
			<div class="eyebrow"><span></span> Feito para compartilhar, não para complicar</div>
			<h1 id="headline">Sua tela.<br />Sua galera.<br /><em>Só isso.</em></h1>
			<p class="lead">Uma partida, uma ideia, aquele bug impossível.<br class="desktop-break" /> Abra uma sala e mostre. Sem conta, sem feed, sem distração.</p>
			<ul class="facts"><li>Tela e câmera</li><li>Até 1080p60</li><li>Código aberto</li></ul>

			<section class="install" aria-label="Instalar Peep">
				<div class="install-top">
					<div class="platforms" aria-label="Sistema operacional">
						<button class:selected={platform === 'linux'} aria-pressed={platform === 'linux'} onclick={() => { platform = 'linux'; copied = false; copyError = ''; }}>Linux <span>Omarchy</span></button>
						<button class:selected={platform === 'windows'} aria-pressed={platform === 'windows'} onclick={() => { platform = 'windows'; copied = false; copyError = ''; }}>Windows</button>
					</div>
					<a href={script} target="_blank" rel="noreferrer">Ver script</a>
				</div>
				<div class="terminal">
					<span class="prompt" aria-hidden="true">{platform === 'linux' ? '$' : 'PS'}</span>
					<code>{command}</code>
					<button class="copy" class:done={copied} onclick={copyCommand} aria-label="Copiar comando de instalação"><Icon name={copied ? 'check' : 'copy'} size={14} />{copied ? 'Copiado' : 'Copiar'}</button>
				</div>
				<p class="install-note" aria-live="polite">{copyError || (platform === 'linux' ? 'Plugin Quickshell + Chromium. No seu Omarchy, na sua barra.' : 'App nativo na bandeja. Instalador baixado da release do GitHub.')}</p>
			</section>
			<p class="alternative">Prefere não usar o terminal? <a href={`${repo}/releases/latest`} target="_blank" rel="noreferrer">Baixe a última release</a></p>
		</section>

		<aside class="preview" aria-label="Ilustração de uma sala Peep">
			<div class="room-window">
				<div class="window-top"><div class="window-dots"><b></b><b></b><b></b></div><span>uma sala, muitas ideias</span></div>
				<div class="screen-art">
					<div class="orbit orbit-one"></div><div class="orbit orbit-two"></div>
					<div class="art-logo"><img src="/favicon.svg" alt="" width="90" height="90" /></div>
					<div class="screen-caption"><span class="live-dot"></span> é melhor mostrar do que explicar.</div>
				</div>
				<div class="room-bottom"><span class="avatar">v</span><span>você está entre amigos</span><span class="room-code">KJ7-29F</span></div>
			</div>
			<div class="floating-note"><span class="note-icon" aria-hidden="true"><Icon name="check" size={18} /></span><div><strong>Entrou. Compartilhou.</strong><span>A sala some quando todo mundo sai.</span></div></div>
			<div class="preview-caption"><span>1. Crie uma sala</span><span>2. Mande o código</span><span>3. Dê o play</span></div>
		</aside>
	</main>

	<footer><p>Pequeno por escolha. <span>Livre por princípio.</span></p><div><a href={repo} target="_blank" rel="noreferrer">Código aberto, licença MIT</a><span class="footer-dot">/</span><a href={`${repo}/tree/main/infra`} target="_blank" rel="noreferrer">Hospede o seu</a></div></footer>
</div>

<style>
	.landing { --ink: #f4f1fb; --soft: #a19bad; --line: #ffffff12; --mint: #b7f3d0; position: relative; isolation: isolate; min-height: 100svh; overflow: hidden; background: #100e17; color: var(--ink); padding: 0 max(28px, calc((100vw - 1180px) / 2)); font-family: Inter, ui-sans-serif, system-ui, sans-serif; }
	.landing::before { content: ''; position: absolute; inset: 0; z-index: -1; opacity: .18; background-image: radial-gradient(#aa99cc 0.7px, transparent .7px); background-size: 28px 28px; mask-image: linear-gradient(black, transparent 85%); }
	.glow { position: absolute; top: -440px; right: -240px; width: 1000px; height: 1000px; background: radial-gradient(ellipse, #7547cc25, transparent 65%); z-index: -1; pointer-events: none; }
	a { color: inherit; text-decoration: none; }
	a:hover { color: var(--mint); }
	a:focus-visible, button:focus-visible { outline: 2px solid var(--mint); outline-offset: 5px; }
	header { height: 100px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--line); }
	.brand { display: flex; align-items: center; gap: 11px; font-size: 28px; letter-spacing: -1px; font-weight: 750; }
	.brand-dot { color: var(--mint); margin-left: -10px; }
	nav { display: flex; align-items: center; gap: 26px; }
	.browser-link { font-size: 13px; color: #e2dbec; padding: 10px 16px; border: 1px solid #a98ada45; border-radius: 10px; background: #a38abb14; transition: background .15s, border-color .15s; }
	.browser-link:hover { background: #a38abb2b; border-color: #a98ada80; color: #fff; }
	.github { display: grid; place-items: center; width: 40px; height: 40px; border: 1px solid #ffffff16; border-radius: 12px; }
	main { display: grid; grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr); align-items: center; gap: 70px; padding: 88px 0 82px; }
	.intro { min-width: 0; }
	.eyebrow { display: flex; align-items: center; gap: 9px; color: #bfb0d4; font-size: 12px; font-weight: 500; }
	.eyebrow span { width: 6px; height: 6px; border-radius: 50%; background: var(--mint); box-shadow: 0 0 14px #b7f3d044; }
	h1 { font-size: clamp(56px, 6.2vw, 82px); line-height: .99; letter-spacing: -5px; font-weight: 750; margin: 27px 0; }
	h1 em { color: var(--mint); font-style: normal; }
	.lead { font-size: 16px; line-height: 1.75; color: var(--soft); margin: 0; letter-spacing: -.2px; }
	.facts { display: flex; flex-wrap: wrap; gap: 8px; margin: 24px 0 30px; padding: 0; list-style: none; font-size: 12px; color: #c2b9cf; }
	.facts li { padding: 5px 10px; border: 1px solid #ffffff14; border-radius: 999px; background: #ffffff06; }
	.install { border: 1px solid #a98ada35; border-radius: 12px; overflow: hidden; background: #191521b3; }
	.install-top { display: flex; align-items: center; justify-content: space-between; padding: 9px 12px 0; gap: 8px; }
	.platforms { display: flex; gap: 5px; }
	.platforms button { background: transparent; border: none; color: #8e849f; padding: 7px 10px; font-size: 12px; border-radius: 7px; }
	.platforms button.selected { color: #e9e0f5; background: #a38abb24; }
	.platforms button span { font-size: 11px; color: #a89aba; margin-left: 4px; }
	.install-top a { font-size: 12px; color: #b3a9c2; white-space: nowrap; padding: 0 4px; }
	.terminal { display: flex; align-items: center; gap: 10px; padding: 18px 16px 12px; }
	.prompt { color: #a692c1; font-family: ui-monospace, monospace; font-size: 12px; }
	code { min-width: 0; flex: 1; font-size: 12.5px; color: #ddd4ec; white-space: nowrap; overflow-x: auto; padding: 8px 0; scrollbar-width: none; mask-image: linear-gradient(90deg, black calc(100% - 40px), transparent); outline: none; }
	code::-webkit-scrollbar { display: none; }
	.terminal { margin: 12px 12px 10px; padding: 6px 6px 6px 14px; background: #0d0b13; border: 1px solid #ffffff0d; border-radius: 9px; }
	.terminal:focus-within { border-color: #a98ada70; }
	.copy { display: inline-flex; align-items: center; gap: 6px; color: #e9e0f5; border: 1px solid #a98ada40; background: #a38abb1f; font-size: 12px; font-weight: 500; padding: 8px 12px; border-radius: 7px; transition: background .15s, border-color .15s; }
	.copy:hover { background: #a38abb33; border-color: #a98ada70; }
	.copy.done { color: var(--mint); border-color: #b7f3d040; background: #b7f3d012; }
	.platforms button, .copy, .install-top a { cursor: pointer; }
	.platforms button:focus-visible, .copy:focus-visible, .install a:focus-visible { outline: 2px solid #a98ada; outline-offset: 2px; }
	.install-note { font-size: 12px; color: #9a8fab; margin: 0; padding: 0 16px 14px; }
	.alternative { margin: 16px 0 0; font-size: 13px; color: #968aa6; }
	.alternative a { color: #d3c8e0; margin-left: 4px; text-decoration: underline; text-decoration-color: #a98ada60; text-underline-offset: 3px; }
	.preview { position: relative; padding: 25px 0 60px; min-width: 0; }
	.room-window { border-radius: 13px; border: 1px solid #ad91d530; background: #17131f; box-shadow: 0 25px 90px #0005, 0 0 100px #8b59d10a; transform: rotate(-3deg); overflow: hidden; }
	.window-top { display: flex; align-items: center; gap: 12px; height: 40px; padding: 0 14px; color: #9a8cab; font-size: 12px; }
	.window-dots { display: flex; gap: 4px; }
	.window-dots b { width: 5px; height: 5px; border-radius: 50%; background: #594765; }
	.window-dots b:first-child { background: #b196c8; }
	.screen-art { position: relative; isolation: isolate; overflow: hidden; aspect-ratio: 1.5; margin: 0 8px; border-radius: 7px; background: radial-gradient(ellipse at 50% 15%, #a37be140, transparent 70%), linear-gradient(140deg, #362345, #201b35 55%, #183332); display: grid; place-items: center; }
	.art-logo { position: relative; z-index: 1; transform: rotate(10deg); filter: drop-shadow(0 12px 30px #100b2c); }
	.art-logo img { width: 84px; height: 84px; border-radius: 24px; }
	.orbit { position: absolute; width: 320px; height: 125px; border: 1px solid #c9afea29; border-radius: 50%; transform: rotate(-35deg); }
	.orbit-two { transform: rotate(35deg); width: 250px; height: 200px; border-color: #b7f3d019; }
	.screen-caption { position: absolute; bottom: 18px; color: #d6c7df; display: flex; align-items: center; gap: 7px; font-size: 13px; letter-spacing: .1px; }
	.live-dot { width: 5px; height: 5px; border-radius: 50%; background: var(--mint); }
	.room-bottom { display: flex; align-items: center; gap: 8px; height: 49px; padding: 0 14px; font-size: 12px; color: #a99bb9; }
	.avatar { display: grid; place-items: center; border-radius: 50%; width: 24px; height: 24px; background: #b7f3d017; color: var(--mint); font-size: 11px; }
	.room-code { margin-left: auto; font-family: ui-monospace, monospace; font-size: 12px; letter-spacing: 1px; border: 1px solid #ffffff0e; padding: 5px 8px; border-radius: 5px; }
	.floating-note { position: absolute; right: -18px; top: -4px; transform: rotate(3deg); display: flex; align-items: center; gap: 12px; padding: 14px 18px; background: #211c2d; border: 1px solid #a890ce35; box-shadow: 0 10px 30px #0004; border-radius: 10px; }
	.note-icon { display: grid; place-items: center; width: 32px; height: 32px; border-radius: 8px; background: #b7f3d015; color: var(--mint); }
	.floating-note div { display: grid; gap: 4px; }
	.floating-note strong { font-size: 13px; font-weight: 600; }
	.floating-note div span { font-size: 12px; color: #b0a2c0; }
	.preview-caption { display: flex; justify-content: space-between; gap: 8px; margin-top: 48px; font-size: 13px; color: #9a8cab; }
	footer { display: flex; justify-content: space-between; align-items: center; gap: 15px; padding: 25px 0; border-top: 1px solid var(--line); color: #a79ab7; font-size: 11px; }
	footer p { margin: 0; }
	footer p span { color: #8a7c99; }
	footer div { display: flex; gap: 14px; font-size: 12px; }
	.footer-dot { color: #53435f; }
	@media (min-width: 1450px) { main { padding-top: 110px; padding-bottom: 110px; } }
	@media (max-width: 1000px) { main { gap: 35px; } h1 { font-size: 66px; letter-spacing: -4px; } .floating-note { right: -5px; } .preview-caption { font-size: 12px; } }
	@media (max-width: 760px) { .landing { padding: 0 24px; } header { height: 80px; } main { grid-template-columns: 1fr; padding: 55px 0 35px; gap: 35px; } .intro { max-width: 530px; } h1 { font-size: clamp(58px, 12vw, 76px); } .preview { max-width: 430px; width: 90%; margin: 0 auto; padding: 72px 0 20px; } .floating-note { top: 0; } .preview-caption { margin-top: 40px; } footer { align-items: start; flex-direction: column; font-size: 12px; } }
	@media (max-width: 390px) { .landing { padding: 0 18px; } nav { gap: 10px; } .browser-link { font-size: 12px; padding: 8px 12px; } .eyebrow { font-size: 11px; } .lead { font-size: 15px; } .terminal { padding-left: 12px; gap: 7px; } code { font-size: 11px; } .desktop-break { display: none; } .floating-note { padding: 10px 14px; } }
</style>
