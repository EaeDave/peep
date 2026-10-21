import {
	AudioPresets,
	ConnectionQuality,
	type Participant,
	RemoteVideoTrack,
	Room,
	RoomEvent,
	ScreenSharePresets,
	Track,
	VideoPresets
} from 'livekit-client';
import { hasNativeScreenAudio, type NativeAudio, startNativeAudio } from './screenAudio.ts';

export type QualityMode = 'jogo' | 'texto' | 'eco';

export const QUALITY: Record<
	QualityMode,
	{
		label: string;
		resolution: { width: number; height: number; frameRate: number };
		contentHint: 'motion' | 'detail';
		maxBitrate: number;
		degradation: RTCDegradationPreference;
		lowLayer: (typeof ScreenSharePresets)[keyof typeof ScreenSharePresets];
	}
> = {
	jogo: {
		label: 'Jogo · 1080p60',
		resolution: { width: 1920, height: 1080, frameRate: 60 },
		contentHint: 'motion',
		maxBitrate: 6_000_000,
		degradation: 'maintain-framerate',
		lowLayer: ScreenSharePresets.h720fps30
	},
	texto: {
		label: 'Texto/código · 1440p30',
		resolution: { width: 2560, height: 1440, frameRate: 30 },
		contentHint: 'detail',
		maxBitrate: 5_000_000,
		degradation: 'maintain-resolution',
		lowLayer: ScreenSharePresets.h720fps15
	},
	eco: {
		label: 'Econômico · 720p30',
		resolution: { width: 1280, height: 720, frameRate: 30 },
		contentHint: 'motion',
		maxBitrate: 1_500_000,
		degradation: 'balanced',
		lowLayer: ScreenSharePresets.h360fps15
	}
};

export interface Tile {
	id: string;
	name: string;
	local: boolean;
	screen: boolean;
	track: Track;
}

export interface Person {
	identity: string;
	name: string;
	local: boolean;
	screen: boolean;
	camera: boolean;
	mic: boolean;
	screenAudio: boolean;
	quality: ConnectionQuality;
}

export class Session {
	readonly room = new Room({
		adaptiveStream: true,
		dynacast: true,
		videoCaptureDefaults: { resolution: VideoPresets.h720.resolution }
	});

	status = $state<'connecting' | 'connected' | 'reconnecting' | 'disconnected'>('connecting');
	tiles = $state<Tile[]>([]);
	people = $state<Person[]>([]);
	focusId = $state<string | null>(null);
	screenOn = $state(false);
	cameraOn = $state(false);
	micOn = $state(false);
	audioBlocked = $state(false);
	volumes = $state<Record<string, number>>({});

	constructor() {
		const r = this.room;
		const refresh = () => this.#refresh();
		r.on(RoomEvent.ParticipantConnected, refresh)
			.on(RoomEvent.ParticipantDisconnected, refresh)
			.on(RoomEvent.TrackPublished, refresh)
			.on(RoomEvent.TrackUnpublished, refresh)
			.on(RoomEvent.TrackSubscribed, (t) => {
				// Mic + screen-share audio. RemoteParticipant.setVolume persists per source.
				if (t.kind === Track.Kind.Audio) document.body.append(t.attach());
				refresh();
			})
			.on(RoomEvent.TrackUnsubscribed, (t) => {
				t.detach().forEach((el) => el.remove());
				refresh();
			})
			.on(RoomEvent.TrackMuted, refresh)
			.on(RoomEvent.TrackUnmuted, refresh)
			.on(RoomEvent.LocalTrackPublished, refresh)
			.on(RoomEvent.LocalTrackUnpublished, (pub) => {
				// Also covers the browser's own "Stop sharing" button.
				if (pub.source === Track.Source.ScreenShare) this.#stopNativeAudio();
				refresh();
			})
			.on(RoomEvent.ConnectionQualityChanged, refresh)
			.on(RoomEvent.Reconnecting, () => (this.status = 'reconnecting'))
			.on(RoomEvent.Reconnected, () => (this.status = 'connected'))
			.on(RoomEvent.Disconnected, () => {
				this.#stopNativeAudio();
				this.status = 'disconnected';
				refresh();
			})
			.on(RoomEvent.AudioPlaybackStatusChanged, () => {
				this.audioBlocked = !r.canPlaybackAudio;
			});
	}

	async connect(url: string, token: string) {
		this.status = 'connecting';
		await this.room.connect(url, token);
		this.status = 'connected';
		this.audioBlocked = !this.room.canPlaybackAudio;
		this.#refresh();
	}

	/** VW-13: volume of a participant's shared-screen audio (0..1). */
	setScreenVolume(identity: string, v: number) {
		this.volumes[identity] = v;
		this.room.remoteParticipants.get(identity)?.setVolume(v, Track.Source.ScreenShareAudio);
	}

	#refresh() {
		const lp = this.room.localParticipant;
		const all: Participant[] = [lp, ...this.room.remoteParticipants.values()];
		const tiles: Tile[] = [];
		const people: Person[] = [];
		for (const p of all) {
			const local = p === lp;
			const name = p.name || p.identity;
			const pub = (s: Track.Source) => p.getTrackPublication(s);
			const live = (s: Track.Source) => {
				const x = pub(s);
				return !!x && !x.isMuted && (local || x.isSubscribed || !!x.track);
			};
			for (const source of [Track.Source.ScreenShare, Track.Source.Camera]) {
				const track = pub(source)?.track;
				if (track && !pub(source)!.isMuted) {
					tiles.push({
						id: `${p.identity}:${source}`,
						name,
						local,
						screen: source === Track.Source.ScreenShare,
						track
					});
				}
			}
			people.push({
				identity: p.identity,
				name,
				local,
				screen: !!pub(Track.Source.ScreenShare),
				camera: live(Track.Source.Camera),
				mic: live(Track.Source.Microphone),
				screenAudio: !!pub(Track.Source.ScreenShareAudio),
				quality: p.connectionQuality
			});
		}

		// VW-3: a newly appearing remote screen share takes focus unless one is already focused.
		const before = new Set(this.tiles.map((t) => t.id));
		const newScreen = tiles.find((t) => t.screen && !t.local && !before.has(t.id));
		const focused = tiles.find((t) => t.id === this.focusId);
		if (newScreen && !focused?.screen) this.focusId = newScreen.id;
		else if (!focused) {
			const fallback =
				tiles.find((t) => t.screen && !t.local) ?? tiles.find((t) => !t.local) ?? tiles[0];
			this.focusId = fallback?.id ?? null;
		}

		this.tiles = tiles;
		this.people = people;
		this.screenOn = lp.isScreenShareEnabled;
		this.cameraOn = lp.isCameraEnabled;
		this.micOn = lp.isMicrophoneEnabled;
	}

	/** Returns a warning when the screen is shared but its audio could not be. */
	async setScreen(on: boolean, mode: QualityMode = 'jogo'): Promise<string | undefined> {
		const q = QUALITY[mode];
		try {
			await this.room.localParticipant.setScreenShareEnabled(
				on,
				{
					// Native capture replaces it where available (window audio, no echo).
					audio: !hasNativeScreenAudio, // VW-6: "Compartilhar áudio do sistema" checkbox in the picker
					systemAudio: 'include',
					selfBrowserSurface: 'exclude',
					surfaceSwitching: 'include',
					suppressLocalAudioPlayback: true,
					resolution: q.resolution,
					contentHint: q.contentHint
				},
				{
					videoCodec: 'vp9',
					backupCodec: { codec: 'vp8' },
					simulcast: true,
					// Declared up front so livekit-client treats VP9 screen share as rid
					// simulcast everywhere. Left implicit, it reports SVC layers to the
					// server but sends q/h simulcast: the server rejects h ("duplicate
					// layer") and viewers only get the 720p30 low layer.
					scalabilityMode: 'L1T3',
					screenShareEncoding: {
						maxBitrate: q.maxBitrate,
						maxFramerate: q.resolution.frameRate,
						priority: 'high'
					},
					screenShareSimulcastLayers: [q.lowLayer],
					degradationPreference: q.degradation
				}
			);
		} catch (e) {
			// User closed the picker: not an error worth showing.
			if ((e as Error).name !== 'NotAllowedError') throw e;
		}
		this.#refresh();
		if (!on) this.#stopNativeAudio();
		else if (hasNativeScreenAudio && this.screenOn && !this.#native) return this.#startNativeAudio();
	}

	#native: NativeAudio | null = null;

	async #startNativeAudio(): Promise<string | undefined> {
		const video = this.room.localParticipant.getTrackPublication(Track.Source.ScreenShare)?.track
			?.mediaStreamTrack;
		if (!video) return;
		try {
			const native = await startNativeAudio(video);
			this.#native = native;
			native.track.contentHint = 'music';
			await this.room.localParticipant.publishTrack(native.track, {
				source: Track.Source.ScreenShareAudio,
				name: 'screen_share_audio',
				audioPreset: AudioPresets.musicHighQualityStereo,
				forceStereo: true,
				dtx: false,
				red: false
			});
		} catch (e) {
			this.#stopNativeAudio();
			return `Tela compartilhada sem áudio: ${(e as Error).message}`;
		}
	}

	#stopNativeAudio() {
		const native = this.#native;
		if (!native) return;
		this.#native = null;
		void this.room.localParticipant.unpublishTrack(native.track).catch(() => {});
		native.stop();
	}

	async setCamera(on: boolean) {
		await this.room.localParticipant.setCameraEnabled(
			on,
			{ resolution: VideoPresets.h720.resolution },
			{ simulcast: true, videoEncoding: VideoPresets.h720.encoding }
		);
		this.#refresh();
	}

	async setMic(on: boolean) {
		await this.room.localParticipant.setMicrophoneEnabled(on);
		this.#refresh();
	}

	// --- Quality sampling: sent to the API for server-side diagnostics. ---
	#prev = new Map<string, { bytes: number; ts: number }>();

	/** kbps since the previous sample of the same RTP stream (bytes*8/ms = kbps). */
	#kbps(id: string, bytes: number, ts: number) {
		const p = this.#prev.get(id);
		this.#prev.set(id, { bytes, ts });
		return p && ts > p.ts ? Math.round(((bytes - p.bytes) * 8) / (ts - p.ts)) : undefined;
	}

	/** Selected ICE path (direct/relay, UDP/TCP), RTT and send bandwidth estimate. */
	#path(stats: RTCStatsReport): IcePath | undefined {
		const all: WebRtcStat[] = [...stats.values()];
		const pairId = all.find((s) => s.type === 'transport')?.selectedCandidatePairId;
		const pair: WebRtcStat | undefined = pairId
			? stats.get(pairId)
			: all.find((s) => s.type === 'candidate-pair' && s.nominated);
		if (!pair?.localCandidateId) return undefined;
		const local: WebRtcStat | undefined = stats.get(pair.localCandidateId);
		return {
			type: local?.candidateType,
			proto: local?.relayProtocol ?? local?.protocol,
			rttMs: pair.currentRoundTripTime === undefined ? undefined : Math.round(pair.currentRoundTripTime * 1000),
			availOutKbps: pair.availableOutgoingBitrate === undefined ? undefined : Math.round(pair.availableOutgoingBitrate / 1000)
		};
	}

	/** One compact sample of every sent and received video stream; null when idle. */
	async sampleStats(): Promise<QualitySample | null> {
		const out: QualitySample['out'] = [];
		const inbound: QualitySample['in'] = [];
		let path: IcePath | undefined;
		const codec = (stats: RTCStatsReport, s: WebRtcStat): string | undefined =>
			s.codecId ? (stats.get(s.codecId) as WebRtcStat | undefined)?.mimeType : undefined;
		for (const pub of this.room.localParticipant.videoTrackPublications.values()) {
			const sender = pub.videoTrack?.sender;
			if (!sender) continue;
			const stats = await sender.getStats();
			path ??= this.#path(stats);
			const layers: QualitySample['out'][number]['layers'] = [];
			for (const s of stats.values() as Iterable<WebRtcStat>) {
				if (s.type !== 'outbound-rtp') continue;
				layers.push({
					rid: s.rid,
					codec: codec(stats, s),
					enc: s.encoderImplementation,
					w: s.frameWidth,
					h: s.frameHeight,
					fps: s.framesPerSecond,
					kbps: this.#kbps(s.id, s.bytesSent ?? 0, s.timestamp),
					limit: s.qualityLimitationReason,
					nack: s.nackCount,
					pli: s.pliCount
				});
			}
			out.push({ src: pub.source, layers });
		}
		for (const p of this.room.remoteParticipants.values()) {
			for (const pub of p.videoTrackPublications.values()) {
				const track = pub.videoTrack;
				const receiver = track instanceof RemoteVideoTrack ? track.receiver : undefined;
				if (!receiver) continue;
				const stats = await receiver.getStats();
				path ??= this.#path(stats);
				for (const s of stats.values() as Iterable<WebRtcStat>) {
					if (s.type !== 'inbound-rtp') continue;
					inbound.push({
						from: p.name || p.identity,
						src: pub.source,
						codec: codec(stats, s),
						dec: s.decoderImplementation,
						w: s.frameWidth,
						h: s.frameHeight,
						fps: s.framesPerSecond,
						kbps: this.#kbps(s.id, s.bytesReceived ?? 0, s.timestamp),
						lost: s.packetsLost,
						recv: s.packetsReceived,
						jitterMs: s.jitter === undefined ? undefined : Math.round(s.jitter * 1000),
						freezes: s.freezeCount,
						dropped: s.framesDropped
					});
				}
			}
		}
		return out.length || inbound.length ? { path, out, in: inbound } : null;
	}
}

/** Fields read from RTCStatsReport entries (all optional: they vary by stat type). */
interface WebRtcStat {
	id: string;
	type: string;
	timestamp: number;
	codecId?: string;
	mimeType?: string;
	rid?: string;
	encoderImplementation?: string;
	decoderImplementation?: string;
	frameWidth?: number;
	frameHeight?: number;
	framesPerSecond?: number;
	bytesSent?: number;
	bytesReceived?: number;
	qualityLimitationReason?: string;
	nackCount?: number;
	pliCount?: number;
	packetsLost?: number;
	packetsReceived?: number;
	jitter?: number;
	freezeCount?: number;
	framesDropped?: number;
	selectedCandidatePairId?: string;
	localCandidateId?: string;
	nominated?: boolean;
	candidateType?: string;
	protocol?: string;
	relayProtocol?: string;
	currentRoundTripTime?: number;
	availableOutgoingBitrate?: number;
}

export interface IcePath {
	type?: string;
	proto?: string;
	rttMs?: number;
	availOutKbps?: number;
}

/** Periodic quality sample posted to the room-api for diagnostics. */
export interface QualitySample {
	path?: IcePath;
	out: {
		src: Track.Source;
		layers: {
			rid?: string;
			codec?: string;
			enc?: string;
			w?: number;
			h?: number;
			fps?: number;
			kbps?: number;
			limit?: string;
			nack?: number;
			pli?: number;
		}[];
	}[];
	in: {
		from: string;
		src: Track.Source;
		codec?: string;
		dec?: string;
		w?: number;
		h?: number;
		fps?: number;
		kbps?: number;
		lost?: number;
		recv?: number;
		jitterMs?: number;
		freezes?: number;
		dropped?: number;
	}[];
}
