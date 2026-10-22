<script lang="ts">
	type Name = 'settings' | 'clipboard' | 'arrow-right' | 'arrow-left' | 'server' | 'key' | 'eye' | 'eye-off' | 'check' | 'screen' | 'camera' | 'mic' | 'mic-off' | 'copy' | 'users' | 'fullscreen' | 'exit' | 'chevron-down' | 'close' | 'refresh' | 'volume';
	let { name, size = 18 }: { name: Name; size?: number } = $props();
	const paths: Record<Name, string> = {
		settings: 'M9 3h6l1 3 3 1 2 5-2 5-3 1-1 3H9l-1-3-3-1-2-5 2-5 3-1 1-3Z M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z',
		refresh: 'M20 12a8 8 0 1 1-2.3-5.7 M20 4v5h-5',
		volume: 'M4 9h4l5-4v14l-5-4H4Z M16 9a4 4 0 0 1 0 6 M18.5 6.5a8 8 0 0 1 0 11',
		clipboard: 'M9 5H5v16h14V5h-4 M9 3h6v4H9Z',
		'arrow-right': 'M4 12h16 M14 6l6 6-6 6',
		'arrow-left': 'M20 12H4 M10 6l-6 6 6 6',
		server: 'M3 3h18v7H3Z M3 14h18v7H3Z M7 6.5h.01 M7 17.5h.01 M11 6.5h6 M11 17.5h6',
		key: 'M14 9a5 5 0 1 1-10 0 5 5 0 0 1 10 0Z M12.5 12.5 21 21 M17 17l3-3 M19 19l3-3',
		eye: 'M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12Z M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z',
		'eye-off': 'm3 3 18 18 M10.6 5.1 12 5c6 0 10 7 10 7a23 23 0 0 1-3 3.8 M6.2 6.2A23 23 0 0 0 2 12s4 7 10 7a11 11 0 0 0 5-1.3 M9.9 9.9a3 3 0 0 0 4.2 4.2',
		check: 'm5 12 4 4L19 6',
		screen: 'M3 4h18v13H3Z M12 17v4 M8 21h8',
		camera: 'M3 6h12v12H3Z M15 10l6-4v12l-6-4',
		mic: 'M9 5a3 3 0 0 1 6 0v7a3 3 0 0 1-6 0V5Z M5 10v2a7 7 0 0 0 14 0v-2 M12 19v3 M8 22h8',
		'mic-off': 'm3 3 18 18 M9 9v3a3 3 0 0 0 5.1 2.1 M9 5a3 3 0 0 1 6 0v6 M5 10v2a7 7 0 0 0 12 4.9 M19 10v2 M12 19v3 M8 22h8',
		copy: 'M9 9h12v12H9Z M5 15H3V3h12v2',
		users: 'M15 7a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z M5 21v-3a7 7 0 0 1 14 0v3 M18 4a3 3 0 0 1 0 6 M20 14a5 5 0 0 1 3 4v3 M6 4a3 3 0 0 0 0 6 M4 14a5 5 0 0 0-3 4v3',
		fullscreen: 'M8 3H3v5 M16 3h5v5 M21 16v5h-5 M3 16v5h5',
		exit: 'M9 3H3v18h6 M9 12h12 M16 7l5 5-5 5',
		'chevron-down': 'm6 9 6 6 6-6',
		close: 'm6 6 12 12 M6 18 18 6'
	};
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false" style="flex-shrink: 0; vertical-align: middle">
	<path d={paths[name]} />
</svg>
