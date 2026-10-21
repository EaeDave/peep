import {
	GtcrnWorkletNode,
	loadGtcrn,
	loadRnnoise,
	RnnoiseWorkletNode
} from '@sapphi-red/web-noise-suppressor';
import gtcrnWasm from '@sapphi-red/web-noise-suppressor/gtcrn.wasm?url';
import gtcrnWorklet from '@sapphi-red/web-noise-suppressor/gtcrnWorklet.js?url';
import rnnoiseWasm from '@sapphi-red/web-noise-suppressor/rnnoise.wasm?url';
import rnnoiseSimdWasm from '@sapphi-red/web-noise-suppressor/rnnoise_simd.wasm?url';
import rnnoiseWorklet from '@sapphi-red/web-noise-suppressor/rnnoiseWorklet.js?url';
import type { AudioProcessorOptions, Track, TrackProcessor } from 'livekit-client';

/**
 * Microphone noise suppression, applied after the browser's echo canceller
 * (LiveKit processor), so echo cancellation keeps working:
 * - strong: GTCRN (ICASSP 2024), best quality per CPU; 16 kHz model internally.
 * - light: RNNoise, full-band 48 kHz, most natural on mild noise.
 * WASM/worklets are served from this site (no third-party CDN).
 */
export type NoiseMode = 'strong' | 'light' | 'off';
export type FilterMode = Exclude<NoiseMode, 'off'>;

type DenoiseNode = AudioWorkletNode & { destroy(): void };

const ENGINES: Record<
	FilterMode,
	{ worklet: string; load: () => Promise<ArrayBuffer>; node: (ctx: AudioContext, wasm: ArrayBuffer) => DenoiseNode }
> = {
	strong: {
		worklet: gtcrnWorklet,
		load: () => loadGtcrn({ url: gtcrnWasm }),
		node: (ctx, wasmBinary) => new GtcrnWorkletNode(ctx, { maxChannels: 1, wasmBinary })
	},
	light: {
		worklet: rnnoiseWorklet,
		load: () => loadRnnoise({ url: rnnoiseWasm, simdUrl: rnnoiseSimdWasm }),
		node: (ctx, wasmBinary) => new RnnoiseWorkletNode(ctx, { maxChannels: 1, wasmBinary })
	}
};

const wasmCache = new Map<FilterMode, Promise<ArrayBuffer>>();

function wasmFor(mode: FilterMode): Promise<ArrayBuffer> {
	let wasm = wasmCache.get(mode);
	if (!wasm) {
		wasm = ENGINES[mode].load();
		wasm.catch(() => wasmCache.delete(mode)); // retry on the next attempt
		wasmCache.set(mode, wasm);
	}
	return wasm;
}

/** Warms the model download; errors surface later, when the filter starts. */
export const preloadNoise = (mode: FilterMode) => void wasmFor(mode).catch(() => {});

/** Capture constraints per mode: the model replaces the browser's own suppressor. */
export const captureFor = (mode: NoiseMode) => {
	const browser = mode === 'off';
	return { noiseSuppression: browser, voiceIsolation: browser };
};

/** Filters to try, best first; `off` always last. */
export const fallbackChain = (mode: NoiseMode): NoiseMode[] =>
	mode === 'strong' ? ['strong', 'light', 'off'] : mode === 'light' ? ['light', 'off'] : ['off'];

export function noiseFilter(mode: FilterMode): TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> {
	let ctx: AudioContext | undefined;
	let source: MediaStreamAudioSourceNode | undefined;
	let node: DenoiseNode | undefined;

	const teardown = async () => {
		source?.disconnect();
		node?.destroy();
		const closing = ctx?.close();
		ctx = source = node = undefined;
		await closing?.catch(() => {});
	};

	const processor: TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> = {
		name: `peep-noise-${mode}`,
		processedTrack: undefined,

		async init({ track }) {
			await teardown();
			const engine = ENGINES[mode];
			const wasm = await wasmFor(mode);
			// Own 48 kHz context: both models need 48 kHz (LiveKit's follows the device rate).
			ctx = new AudioContext({ sampleRate: 48_000 });
			await ctx.audioWorklet.addModule(engine.worklet);
			node = engine.node(ctx, wasm);
			// Downmix stereo mics to mono before the (mono) model.
			node.channelCount = 1;
			node.channelCountMode = 'explicit';
			source = ctx.createMediaStreamSource(new MediaStream([track]));
			const dest = ctx.createMediaStreamDestination();
			source.connect(node).connect(dest);
			await ctx.resume();
			if (ctx.state !== 'running') {
				await teardown();
				throw new Error('o navegador bloqueou o processamento de áudio');
			}
			processor.processedTrack = dest.stream.getAudioTracks()[0];
		},

		// Called by LiveKit after a microphone switch or track restart.
		async restart(opts) {
			await processor.init(opts);
		},

		async destroy() {
			await teardown();
			processor.processedTrack = undefined;
		}
	};
	return processor;
}
