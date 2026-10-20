<script lang="ts">
	import Icon from '#lib/Icon.svelte';
	import { normalizeServerUrl, settings } from '#lib/config.ts';

	let { onsaved, oncancel }: { onsaved: () => void; oncancel: () => void } = $props();
	let apiUrl = $state(settings.apiUrl);
	let accessKey = $state(settings.secret);
	let reveal = $state(false);
	let apiError = $state('');
	let keyError = $state('');
	let storageError = $state('');
	let apiInput: HTMLInputElement;
	let keyInput: HTMLInputElement;

	function save() {
		apiError = '';
		keyError = '';
		storageError = '';
		let normalized: string;
		try {
			normalized = normalizeServerUrl(apiUrl);
		} catch (e) {
			apiError = (e as Error).message;
			apiInput.focus();
			return;
		}
		const key = accessKey.trim();
		if (!key) {
			keyError = 'Informe a chave fornecida por quem hospeda o servidor.';
			keyInput.focus();
			return;
		}
		settings.apiUrl = normalized;
		settings.secret = key;
		if (settings.apiUrl !== normalized || settings.secret !== key) {
			storageError = 'Não foi possível salvar neste dispositivo. Verifique se o armazenamento local está permitido.';
			return;
		}
		onsaved();
	}
</script>

<form class="server-settings" novalidate onsubmit={(event) => { event.preventDefault(); save(); }}>
	<div class="intro">
		<h2>Configurar servidor</h2>
		<p>Escolha onde suas salas vão acontecer.</p>
	</div>

	<div class="field">
		<label for="peep-api-url">URL da API</label>
		<input id="peep-api-url" name="apiUrl" type="url" bind:this={apiInput} bind:value={apiUrl}
			autocomplete="url" spellcheck="false" placeholder="https://api.exemplo.com"
			aria-invalid={!!apiError} aria-describedby={apiError ? 'peep-api-error' : 'peep-api-help'} />
		{#if apiError}<p id="peep-api-error" class="field-error" role="alert">{apiError}</p>
		{:else}<p id="peep-api-help" class="help">Endereço da API de salas, não do servidor TURN.</p>{/if}
	</div>

	<div class="field">
		<label for="peep-server-key">Chave de acesso ao servidor</label>
		<div class="key-input">
			<input id="peep-server-key" name="serverAccessKey" type={reveal ? 'text' : 'password'}
				bind:this={keyInput} bind:value={accessKey} autocomplete="off" spellcheck="false"
				placeholder="Cole a chave de acesso" aria-invalid={!!keyError}
				aria-describedby={keyError ? 'peep-key-error' : 'peep-key-help'} />
			<button type="button" class="reveal" onclick={() => (reveal = !reveal)}
				aria-label={reveal ? 'Ocultar chave de acesso' : 'Mostrar chave de acesso'} aria-pressed={reveal}>
				<Icon name={reveal ? 'eye-off' : 'eye'} size={17} />
			</button>
		</div>
		{#if keyError}<p id="peep-key-error" class="field-error" role="alert">{keyError}</p>
		{:else}<p id="peep-key-help" class="help">Fornecida por quem hospeda o Peep; não é o código da sala.</p>{/if}
	</div>

	<div class="privacy"><Icon name="key" size={15} /><span>A chave fica neste dispositivo. Nunca a compartilhe em salas públicas.</span></div>
	{#if storageError}<p class="field-error" role="alert">{storageError}</p>{/if}
	<div class="actions">
		<button type="button" class="cancel" onclick={oncancel}>Cancelar</button>
		<button type="submit" class="save">Salvar configurações</button>
	</div>
</form>

<style>
	.server-settings { display: grid; gap: 20px; color: var(--fg); }
	.intro h2 { margin: 0 0 6px; font-size: 23px; font-weight: 600; letter-spacing: -.7px; text-wrap: balance; }
	.intro p { margin: 0; color: var(--muted); font-size: 13px; }
	.field { display: grid; gap: 8px; }
	label { display: block; color: var(--fg); font-size: 12px; font-weight: 500; }
	input { min-width: 0; height: 43px; padding: 10px 12px; border-radius: 8px; background: var(--bg); font-size: 13px; }
	input[aria-invalid='true'] { border-color: var(--danger); }
	input:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
	.help, .field-error { margin: 0; font-size: 11px; line-height: 1.55; }
	.help { color: var(--muted); }
	.field-error { color: var(--danger); }
	.key-input { position: relative; }
	.key-input input { padding-right: 42px; }
	.reveal { position: absolute; top: 3px; right: 3px; display: grid; place-items: center; height: 37px; width: 37px; padding: 0; border: 0; border-radius: 6px; color: var(--muted); background: transparent; }
	.reveal:hover { color: var(--fg); background: var(--card); }
	.privacy { display: flex; align-items: start; gap: 9px; color: var(--muted); font-size: 11px; line-height: 1.5; }
	.privacy :global(svg) { flex-shrink: 0; margin-top: 1px; }
	.actions { display: flex; justify-content: flex-end; gap: 10px; padding-top: 3px; }
	.actions button { min-height: 40px; border-radius: 8px; padding: 9px 13px; font-size: 12px; font-weight: 500; }
	.cancel { background: transparent; color: var(--muted); }
	.cancel:hover { color: var(--fg); background: var(--bg); }
	.save { background: var(--accent); border-color: var(--accent); color: var(--accent-fg); }
	.save:hover { filter: brightness(1.1); }
	button:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
	button { touch-action: manipulation; }
	@media (max-width: 360px) { .server-settings { gap: 15px; } .intro h2 { font-size: 20px; } .privacy { display: none; } }
</style>
