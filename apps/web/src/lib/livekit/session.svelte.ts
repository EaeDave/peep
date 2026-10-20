import {
	ConnectionQuality,
	type Participant,
	Room,
	RoomEvent,
	ScreenSharePresets,
	Track,
	VideoPresets
} from 'livekit-client';

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
			.on(RoomEvent.LocalTrackUnpublished, refresh)
			.on(RoomEvent.ConnectionQualityChanged, refresh)
			.on(RoomEvent.Reconnecting, () => (this.status = 'reconnecting'))
			.on(RoomEvent.Reconnected, () => (this.status = 'connected'))
			.on(RoomEvent.Disconnected, () => {
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

	async setScreen(on: boolean, mode: QualityMode = 'jogo') {
		const q = QUALITY[mode];
		try {
			await this.room.localParticipant.setScreenShareEnabled(
				on,
				{
					audio: true, // VW-6: "Compartilhar áudio do sistema" checkbox in the picker
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
}
