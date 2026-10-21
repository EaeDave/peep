import { Channel, invoke } from '@tauri-apps/api/core';
import { isTauri } from '#lib/platform.ts';

/**
 * Screen-share audio that browsers can't capture (window audio, Linux system
 * audio), always without the call's own playback (no echo):
 * - Windows app: WASAPI process loopback in Rust, PCM streamed over IPC.
 * - Omarchy: peep-audio.sh feeds the "Peep" PipeWire virtual source; the
 *   launcher marks the room URL with ?audio=pipewire.
 */
export interface NativeAudio {
	track: MediaStreamTrack;
	stop(): void;
}

const pipewire =
	typeof location !== 'undefined' && new URLSearchParams(location.search).get('audio') === 'pipewire';
const windowsApp = isTauri && typeof navigator !== 'undefined' && navigator.userAgent.includes('Windows');

/** When true, getDisplayMedia audio is skipped: the native source replaces it. */
export const hasNativeScreenAudio = pipewire || windowsApp;

export function startNativeAudio(video: MediaStreamTrack): Promise<NativeAudio> {
	return windowsApp ? wasapiAudio(video) : pipewireAudio();
}

// --- Omarchy: PipeWire virtual source --------------------------------------

const SOURCE_LABEL = 'Peep';

async function pipewireAudio(): Promise<NativeAudio> {
	const find = async () =>
		(await navigator.mediaDevices.enumerateDevices()).find(
			(d) => d.kind === 'audioinput' && d.label.includes(SOURCE_LABEL)
		);
	let device = await find();
	if (!device) {
		// Device labels stay hidden until the profile has microphone permission.
		const probe = await navigator.mediaDevices.getUserMedia({ audio: true });
		probe.getTracks().forEach((t) => t.stop());
		device = await find();
	}
	if (!device) throw new Error('Fonte de áudio do Peep não encontrada no PipeWire.');
	const stream = await navigator.mediaDevices.getUserMedia({
		audio: {
			deviceId: { exact: device.deviceId },
			channelCount: 2,
			echoCancellation: false,
			noiseSuppression: false,
			autoGainControl: false
		}
	});
	const track = stream.getAudioTracks()[0];
	return { track, stop: () => track.stop() };
}

// --- Windows app: WASAPI process loopback ----------------------------------

// Plays interleaved f32 stereo chunks; drops backlog beyond ~100 ms to bound latency.
const WORKLET = `
class PeepPcm extends AudioWorkletProcessor {
	constructor() {
		super();
		this.queue = [];
		this.offset = 0;
		this.port.onmessage = (e) => {
			this.queue.push(new Float32Array(e.data));
			if (this.queue.length > 30) this.queue.splice(0, this.queue.length - 10);
		};
	}
	process(_, [out]) {
		const [left, right] = out;
		for (let i = 0; i < left.length; i++) {
			const chunk = this.queue[0];
			if (!chunk) { left[i] = right[i] = 0; continue; }
			left[i] = chunk[this.offset];
			right[i] = chunk[this.offset + 1];
			this.offset += 2;
			if (this.offset >= chunk.length) { this.queue.shift(); this.offset = 0; }
		}
		return true;
	}
}
registerProcessor('peep-pcm', PeepPcm);
`;

async function wasapiAudio(video: MediaStreamTrack): Promise<NativeAudio> {
	// Chromium window captures carry the native handle: "window:<HWND>:<n>".
	const hwnd = /^window:(\d+):/.exec(video.getSettings().deviceId ?? '')?.[1];
	const ctx = new AudioContext({ sampleRate: 48_000 });
	const url = URL.createObjectURL(new Blob([WORKLET], { type: 'text/javascript' }));
	try {
		await ctx.audioWorklet.addModule(url);
	} finally {
		URL.revokeObjectURL(url);
	}
	const node = new AudioWorkletNode(ctx, 'peep-pcm', { outputChannelCount: [2] });
	const dest = ctx.createMediaStreamDestination();
	node.connect(dest);
	const channel = new Channel<ArrayBuffer>();
	channel.onmessage = (buf) => node.port.postMessage(buf, [buf]);
	try {
		await invoke('start_screen_audio', { hwnd: hwnd ? Number(hwnd) : null, channel });
	} catch (e) {
		await ctx.close();
		throw new Error(String(e));
	}
	await ctx.resume();
	const track = dest.stream.getAudioTracks()[0];
	return {
		track,
		stop: () => {
			void invoke('stop_screen_audio');
			track.stop();
			void ctx.close();
		}
	};
}
