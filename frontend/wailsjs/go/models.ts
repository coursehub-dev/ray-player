export namespace appstate {
	
	export class PlayerState {
	    status: string;
	    currentTrackId: string;
	    currentPath: string;
	    currentTitle: string;
	    currentArtist: string;
	    currentSub: string;
	    durationMs: number;
	    durationLabel: string;
	    positionMs: number;
	    positionLabel: string;
	    queueId: string;
	    queueIndex: number;
	    queueLength: number;
	    rayId: string;
	    raySeedTrackId: string;
	    updatedAt: number;
	    lastError?: string;
	    playing: boolean;
	    volume: number;
	    muted: boolean;
	    lastNonZeroVolume: number;
	    currentRayId: string;
	    queue: rays.QueueItem[];
	
	    static createFrom(source: any = {}) {
	        return new PlayerState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.currentTrackId = source["currentTrackId"];
	        this.currentPath = source["currentPath"];
	        this.currentTitle = source["currentTitle"];
	        this.currentArtist = source["currentArtist"];
	        this.currentSub = source["currentSub"];
	        this.durationMs = source["durationMs"];
	        this.durationLabel = source["durationLabel"];
	        this.positionMs = source["positionMs"];
	        this.positionLabel = source["positionLabel"];
	        this.queueId = source["queueId"];
	        this.queueIndex = source["queueIndex"];
	        this.queueLength = source["queueLength"];
	        this.rayId = source["rayId"];
	        this.raySeedTrackId = source["raySeedTrackId"];
	        this.updatedAt = source["updatedAt"];
	        this.lastError = source["lastError"];
	        this.playing = source["playing"];
	        this.volume = source["volume"];
	        this.muted = source["muted"];
	        this.lastNonZeroVolume = source["lastNonZeroVolume"];
	        this.currentRayId = source["currentRayId"];
	        this.queue = this.convertValues(source["queue"], rays.QueueItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RayBuildState {
	    status: string;
	    seedTrackId: string;
	    requestId: number;
	    startedAt: number;
	    finishedAt: number;
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new RayBuildState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.seedTrackId = source["seedTrackId"];
	        this.requestId = source["requestId"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.lastError = source["lastError"];
	    }
	}

}

export namespace deps {
	
	export class Check {
	    id: string;
	    title: string;
	    status: string;
	    message: string;
	    path?: string;
	    repairable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Check(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.path = source["path"];
	        this.repairable = source["repairable"];
	    }
	}
	export class SettingsPatch {
	    onnxRuntimePath?: string;
	    miniLMModelDir?: string;
	    ffmpegPath?: string;
	    ffprobePath?: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingsPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.onnxRuntimePath = source["onnxRuntimePath"];
	        this.miniLMModelDir = source["miniLMModelDir"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffprobePath = source["ffprobePath"];
	    }
	}
	export class RepairResult {
	    check: Check;
	    patch: SettingsPatch;
	
	    static createFrom(source: any = {}) {
	        return new RepairResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.check = this.convertValues(source["check"], Check);
	        this.patch = this.convertValues(source["patch"], SettingsPatch);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace emoflow {
	
	export class Palette {
	    accent: string;
	    accentSoft: string;
	    accentHot: string;
	    background: string;
	    surface: string;
	    glow: string;
	    glowSoft: string;
	    ring: string;
	    progress: string;
	    icon: string;
	    accentOn: string;
	
	    static createFrom(source: any = {}) {
	        return new Palette(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accent = source["accent"];
	        this.accentSoft = source["accentSoft"];
	        this.accentHot = source["accentHot"];
	        this.background = source["background"];
	        this.surface = source["surface"];
	        this.glow = source["glow"];
	        this.glowSoft = source["glowSoft"];
	        this.ring = source["ring"];
	        this.progress = source["progress"];
	        this.icon = source["icon"];
	        this.accentOn = source["accentOn"];
	    }
	}
	export class Vector {
	    energy: number;
	    valence: number;
	    brightness: number;
	    darkness: number;
	    calmness: number;
	    aggression: number;
	    movement: number;
	    tempoBpm: number;
	    tempoConfidence: number;
	    rhythmicPulse: number;
	    drive: number;
	    melancholy: number;
	    intensity: number;
	    pulse: number;
	    clubPressure: number;
	    mechanicalPressure: number;
	    atmosphere: number;
	    melodicness: number;
	    softness: number;
	    heaviness: number;
	    dreaminess: number;
	    acousticness: number;
	    electronicness: number;
	    instrumentalness: number;
	    vocalness: number;
	    timbreBrightness: number;
	    tonality: number;
	    approachability: number;
	    engagement: number;
	
	    static createFrom(source: any = {}) {
	        return new Vector(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.energy = source["energy"];
	        this.valence = source["valence"];
	        this.brightness = source["brightness"];
	        this.darkness = source["darkness"];
	        this.calmness = source["calmness"];
	        this.aggression = source["aggression"];
	        this.movement = source["movement"];
	        this.tempoBpm = source["tempoBpm"];
	        this.tempoConfidence = source["tempoConfidence"];
	        this.rhythmicPulse = source["rhythmicPulse"];
	        this.drive = source["drive"];
	        this.melancholy = source["melancholy"];
	        this.intensity = source["intensity"];
	        this.pulse = source["pulse"];
	        this.clubPressure = source["clubPressure"];
	        this.mechanicalPressure = source["mechanicalPressure"];
	        this.atmosphere = source["atmosphere"];
	        this.melodicness = source["melodicness"];
	        this.softness = source["softness"];
	        this.heaviness = source["heaviness"];
	        this.dreaminess = source["dreaminess"];
	        this.acousticness = source["acousticness"];
	        this.electronicness = source["electronicness"];
	        this.instrumentalness = source["instrumentalness"];
	        this.vocalness = source["vocalness"];
	        this.timbreBrightness = source["timbreBrightness"];
	        this.tonality = source["tonality"];
	        this.approachability = source["approachability"];
	        this.engagement = source["engagement"];
	    }
	}
	export class TrackState {
	    trackId: string;
	    title: string;
	    artist: string;
	    vector: Vector;
	    palette: Palette;
	    intensity: number;
	    heat: number;
	    cool: number;
	    tension: number;
	    reason: string;
	    dominant: string;
	    secondary: string;
	    direction: string;
	
	    static createFrom(source: any = {}) {
	        return new TrackState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trackId = source["trackId"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.vector = this.convertValues(source["vector"], Vector);
	        this.palette = this.convertValues(source["palette"], Palette);
	        this.intensity = source["intensity"];
	        this.heat = source["heat"];
	        this.cool = source["cool"];
	        this.tension = source["tension"];
	        this.reason = source["reason"];
	        this.dominant = source["dominant"];
	        this.secondary = source["secondary"];
	        this.direction = source["direction"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TransitionVisualState {
	    moodDistance: number;
	    energyDelta: number;
	    aggroDelta: number;
	    direction: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new TransitionVisualState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.moodDistance = source["moodDistance"];
	        this.energyDelta = source["energyDelta"];
	        this.aggroDelta = source["aggroDelta"];
	        this.direction = source["direction"];
	        this.reason = source["reason"];
	    }
	}
	export class UISettings {
	    enabled: boolean;
	    intensity: number;
	    animateDuringTrack: boolean;
	    respectReducedMotion: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.intensity = source["intensity"];
	        this.animateDuringTrack = source["animateDuringTrack"];
	        this.respectReducedMotion = source["respectReducedMotion"];
	    }
	}
	export class UIState {
	    trackId: string;
	    current: TrackState;
	    previous?: TrackState;
	    next?: TrackState;
	    vector: Vector;
	    direction: string;
	    intensity: number;
	    heat: number;
	    cool: number;
	    tension: number;
	    palette: Palette;
	    reason: string;
	    transition: TransitionVisualState;
	
	    static createFrom(source: any = {}) {
	        return new UIState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trackId = source["trackId"];
	        this.current = this.convertValues(source["current"], TrackState);
	        this.previous = this.convertValues(source["previous"], TrackState);
	        this.next = this.convertValues(source["next"], TrackState);
	        this.vector = this.convertValues(source["vector"], Vector);
	        this.direction = source["direction"];
	        this.intensity = source["intensity"];
	        this.heat = source["heat"];
	        this.cool = source["cool"];
	        this.tension = source["tension"];
	        this.palette = this.convertValues(source["palette"], Palette);
	        this.reason = source["reason"];
	        this.transition = this.convertValues(source["transition"], TransitionVisualState);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace events {
	
	export class HistoryItem {
	    track: library.Track;
	    positionMs: number;
	    progress: number;
	    progressLabel: string;
	    playedAtLabel: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.track = this.convertValues(source["track"], library.Track);
	        this.positionMs = source["positionMs"];
	        this.progress = source["progress"];
	        this.progressLabel = source["progressLabel"];
	        this.playedAtLabel = source["playedAtLabel"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace externalmedia {
	
	export class JobDTO {
	    id: string;
	    libraryType: string;
	    itemId: string;
	    url: string;
	    sourceSite: string;
	    externalId: string;
	    status: string;
	    progress: number;
	    title: string;
	    uploader: string;
	    duration: number;
	    thumbnailUrl: string;
	    outputPath: string;
	    error: string;
	    attempts: number;
	    maxAttempts: number;
	
	    static createFrom(source: any = {}) {
	        return new JobDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.libraryType = source["libraryType"];
	        this.itemId = source["itemId"];
	        this.url = source["url"];
	        this.sourceSite = source["sourceSite"];
	        this.externalId = source["externalId"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.title = source["title"];
	        this.uploader = source["uploader"];
	        this.duration = source["duration"];
	        this.thumbnailUrl = source["thumbnailUrl"];
	        this.outputPath = source["outputPath"];
	        this.error = source["error"];
	        this.attempts = source["attempts"];
	        this.maxAttempts = source["maxAttempts"];
	    }
	}
	export class Settings {
	    ytDlpPath: string;
	    ffmpegPath: string;
	    ytDlpDownloadDir: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ytDlpPath = source["ytDlpPath"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ytDlpDownloadDir = source["ytDlpDownloadDir"];
	    }
	}
	export class ToolCheckResult {
	    ok: boolean;
	    version: string;
	    output: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.version = source["version"];
	        this.output = source["output"];
	        this.error = source["error"];
	    }
	}

}

export namespace library {
	
	export class FileError {
	    id: string;
	    trackId: string;
	    path: string;
	    libraryType: string;
	    stage: string;
	    kind: string;
	    message: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new FileError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.trackId = source["trackId"];
	        this.path = source["path"];
	        this.libraryType = source["libraryType"];
	        this.stage = source["stage"];
	        this.kind = source["kind"];
	        this.message = source["message"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class LibraryRoot {
	    id: string;
	    path: string;
	    libraryType: string;
	    enabled: boolean;
	    recursive: boolean;
	    lastScanStartedAt: number;
	    lastScanFinishedAt: number;
	    lastScanError: string;
	
	    static createFrom(source: any = {}) {
	        return new LibraryRoot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.libraryType = source["libraryType"];
	        this.enabled = source["enabled"];
	        this.recursive = source["recursive"];
	        this.lastScanStartedAt = source["lastScanStartedAt"];
	        this.lastScanFinishedAt = source["lastScanFinishedAt"];
	        this.lastScanError = source["lastScanError"];
	    }
	}
	export class ImportSummary {
	    sessionId: string;
	    inputCount: number;
	    scanned: number;
	    audioFound: number;
	    added: number;
	    updated: number;
	    unchanged: number;
	    skipped: number;
	    errors: number;
	    alreadyPresent: number;
	    roots?: LibraryRoot[];
	    fileErrors?: FileError[];
	
	    static createFrom(source: any = {}) {
	        return new ImportSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.inputCount = source["inputCount"];
	        this.scanned = source["scanned"];
	        this.audioFound = source["audioFound"];
	        this.added = source["added"];
	        this.updated = source["updated"];
	        this.unchanged = source["unchanged"];
	        this.skipped = source["skipped"];
	        this.errors = source["errors"];
	        this.alreadyPresent = source["alreadyPresent"];
	        this.roots = this.convertValues(source["roots"], LibraryRoot);
	        this.fileErrors = this.convertValues(source["fileErrors"], FileError);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LibraryStats {
	    tracks: number;
	    roots: number;
	    errors: number;
	
	    static createFrom(source: any = {}) {
	        return new LibraryStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tracks = source["tracks"];
	        this.roots = source["roots"];
	        this.errors = source["errors"];
	    }
	}
	export class Track {
	    id: string;
	    path: string;
	    title: string;
	    artist: string;
	    album: string;
	    importedAt: number;
	    genre: string;
	    genrePrimary: string;
	    genreDetail: string;
	    genreTags: onnx.GenreTag[];
	    genreLabel: string;
	    durationMs: number;
	    durationLabel: string;
	    folder: string;
	    fileName: string;
	    tempo: number;
	    bpmPerceived: number;
	    tempoConfidence: number;
	    tempoStability: number;
	    bpmHalf: number;
	    bpmDouble: number;
	    tempoSource: string;
	    tempoModelVersion: string;
	    tempoAnalyzedAt: number;
	    tempoError: string;
	    energy: number;
	    danceability: number;
	    valence: number;
	    acousticness: number;
	    electronicness: number;
	    instrumentalness: number;
	    vocalness: number;
	    happy: number;
	    sad: number;
	    relaxed: number;
	    party: number;
	    aggressive: number;
	    timbreBrightness: number;
	    tonality: number;
	    approachability: number;
	    engagement: number;
	    melodicness: number;
	    softness: number;
	    heaviness: number;
	    dreaminess: number;
	    emotionality: number;
	    loudness: number;
	    spectralCentroid: number;
	    zeroCrossingRate: number;
	    rms: number;
	    spectralFlatness: number;
	    spectralRolloff85: number;
	    spectralFlux: number;
	    onsetRate: number;
	    dynamicRange: number;
	    lowBandRatio: number;
	    midBandRatio: number;
	    highBandRatio: number;
	    clusterId: number;
	    playCount: number;
	    skipCount: number;
	    completeCount: number;
	    metadataSource: string;
	    analyzedLevel: number;
	    analysisVersion: number;
	    analyzedAt: number;
	    analysisError: string;
	    essentiaModelVersion: string;
	    normalizedPath: string;
	    libraryRootId: string;
	    importStatus: string;
	    analysisStatus: string;
	    fileMissing: boolean;
	    fileSize: number;
	    fileMtime: number;
	    fileInode: string;
	    quickHash: string;
	    lastSeenAt: number;
	    lastError: string;
	    playbackErrorCount: number;
	    lastPlaybackError: string;
	    lastPlaybackErrorAt: number;
	    sourceType: string;
	    sourceUrl: string;
	    sourceSite: string;
	    externalId: string;
	    downloadStatus: string;
	    downloadProgress: number;
	    downloadError: string;
	    downloadAttempts: number;
	    downloadedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Track(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.importedAt = source["importedAt"];
	        this.genre = source["genre"];
	        this.genrePrimary = source["genrePrimary"];
	        this.genreDetail = source["genreDetail"];
	        this.genreTags = this.convertValues(source["genreTags"], onnx.GenreTag);
	        this.genreLabel = source["genreLabel"];
	        this.durationMs = source["durationMs"];
	        this.durationLabel = source["durationLabel"];
	        this.folder = source["folder"];
	        this.fileName = source["fileName"];
	        this.tempo = source["tempo"];
	        this.bpmPerceived = source["bpmPerceived"];
	        this.tempoConfidence = source["tempoConfidence"];
	        this.tempoStability = source["tempoStability"];
	        this.bpmHalf = source["bpmHalf"];
	        this.bpmDouble = source["bpmDouble"];
	        this.tempoSource = source["tempoSource"];
	        this.tempoModelVersion = source["tempoModelVersion"];
	        this.tempoAnalyzedAt = source["tempoAnalyzedAt"];
	        this.tempoError = source["tempoError"];
	        this.energy = source["energy"];
	        this.danceability = source["danceability"];
	        this.valence = source["valence"];
	        this.acousticness = source["acousticness"];
	        this.electronicness = source["electronicness"];
	        this.instrumentalness = source["instrumentalness"];
	        this.vocalness = source["vocalness"];
	        this.happy = source["happy"];
	        this.sad = source["sad"];
	        this.relaxed = source["relaxed"];
	        this.party = source["party"];
	        this.aggressive = source["aggressive"];
	        this.timbreBrightness = source["timbreBrightness"];
	        this.tonality = source["tonality"];
	        this.approachability = source["approachability"];
	        this.engagement = source["engagement"];
	        this.melodicness = source["melodicness"];
	        this.softness = source["softness"];
	        this.heaviness = source["heaviness"];
	        this.dreaminess = source["dreaminess"];
	        this.emotionality = source["emotionality"];
	        this.loudness = source["loudness"];
	        this.spectralCentroid = source["spectralCentroid"];
	        this.zeroCrossingRate = source["zeroCrossingRate"];
	        this.rms = source["rms"];
	        this.spectralFlatness = source["spectralFlatness"];
	        this.spectralRolloff85 = source["spectralRolloff85"];
	        this.spectralFlux = source["spectralFlux"];
	        this.onsetRate = source["onsetRate"];
	        this.dynamicRange = source["dynamicRange"];
	        this.lowBandRatio = source["lowBandRatio"];
	        this.midBandRatio = source["midBandRatio"];
	        this.highBandRatio = source["highBandRatio"];
	        this.clusterId = source["clusterId"];
	        this.playCount = source["playCount"];
	        this.skipCount = source["skipCount"];
	        this.completeCount = source["completeCount"];
	        this.metadataSource = source["metadataSource"];
	        this.analyzedLevel = source["analyzedLevel"];
	        this.analysisVersion = source["analysisVersion"];
	        this.analyzedAt = source["analyzedAt"];
	        this.analysisError = source["analysisError"];
	        this.essentiaModelVersion = source["essentiaModelVersion"];
	        this.normalizedPath = source["normalizedPath"];
	        this.libraryRootId = source["libraryRootId"];
	        this.importStatus = source["importStatus"];
	        this.analysisStatus = source["analysisStatus"];
	        this.fileMissing = source["fileMissing"];
	        this.fileSize = source["fileSize"];
	        this.fileMtime = source["fileMtime"];
	        this.fileInode = source["fileInode"];
	        this.quickHash = source["quickHash"];
	        this.lastSeenAt = source["lastSeenAt"];
	        this.lastError = source["lastError"];
	        this.playbackErrorCount = source["playbackErrorCount"];
	        this.lastPlaybackError = source["lastPlaybackError"];
	        this.lastPlaybackErrorAt = source["lastPlaybackErrorAt"];
	        this.sourceType = source["sourceType"];
	        this.sourceUrl = source["sourceUrl"];
	        this.sourceSite = source["sourceSite"];
	        this.externalId = source["externalId"];
	        this.downloadStatus = source["downloadStatus"];
	        this.downloadProgress = source["downloadProgress"];
	        this.downloadError = source["downloadError"];
	        this.downloadAttempts = source["downloadAttempts"];
	        this.downloadedAt = source["downloadedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class BootstrapPayload {
	    library: library.Track[];
	    podcasts: podcast.Item[];
	    podcastRay: podcast.Ray;
	    podcastPlayback: podcast.Playback;
	    podcastHistory: podcast.HistoryItem[];
	    podcastRays: podcast.RayHistoryItem[];
	    current: appstate.PlayerState;
	    history: events.HistoryItem[];
	    rays: rays.RaySummary[];
	    queue: rays.QueueItem[];
	    musicRay: rays.Ray;
	    libraryStat: library.LibraryStats;
	    roots?: library.LibraryRoot[];
	    importErrors?: library.FileError[];
	    emoFlow: emoflow.UIState;
	    emoFlowUiSettings: emoflow.UISettings;
	    rayBuild: appstate.RayBuildState;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.library = this.convertValues(source["library"], library.Track);
	        this.podcasts = this.convertValues(source["podcasts"], podcast.Item);
	        this.podcastRay = this.convertValues(source["podcastRay"], podcast.Ray);
	        this.podcastPlayback = this.convertValues(source["podcastPlayback"], podcast.Playback);
	        this.podcastHistory = this.convertValues(source["podcastHistory"], podcast.HistoryItem);
	        this.podcastRays = this.convertValues(source["podcastRays"], podcast.RayHistoryItem);
	        this.current = this.convertValues(source["current"], appstate.PlayerState);
	        this.history = this.convertValues(source["history"], events.HistoryItem);
	        this.rays = this.convertValues(source["rays"], rays.RaySummary);
	        this.queue = this.convertValues(source["queue"], rays.QueueItem);
	        this.musicRay = this.convertValues(source["musicRay"], rays.Ray);
	        this.libraryStat = this.convertValues(source["libraryStat"], library.LibraryStats);
	        this.roots = this.convertValues(source["roots"], library.LibraryRoot);
	        this.importErrors = this.convertValues(source["importErrors"], library.FileError);
	        this.emoFlow = this.convertValues(source["emoFlow"], emoflow.UIState);
	        this.emoFlowUiSettings = this.convertValues(source["emoFlowUiSettings"], emoflow.UISettings);
	        this.rayBuild = this.convertValues(source["rayBuild"], appstate.RayBuildState);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DebugReindexResult {
	    started: boolean;
	    busy: boolean;
	    total: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DebugReindexResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.started = source["started"];
	        this.busy = source["busy"];
	        this.total = source["total"];
	        this.message = source["message"];
	    }
	}
	export class ModelCheckResult {
	    name: string;
	    modelPath: string;
	    metaPath: string;
	    present: boolean;
	    loaded: boolean;
	    message: string;
	    inputName: string;
	    outputName: string;
	    inputShape: string[];
	    outputShape: string[];
	
	    static createFrom(source: any = {}) {
	        return new ModelCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.modelPath = source["modelPath"];
	        this.metaPath = source["metaPath"];
	        this.present = source["present"];
	        this.loaded = source["loaded"];
	        this.message = source["message"];
	        this.inputName = source["inputName"];
	        this.outputName = source["outputName"];
	        this.inputShape = source["inputShape"];
	        this.outputShape = source["outputShape"];
	    }
	}
	export class EssentiaTestResult {
	    ok: boolean;
	    runtimePath: string;
	    modelDir: string;
	    base: ModelCheckResult;
	    genre: ModelCheckResult;
	    heads: ModelCheckResult[];
	    message: string;
	    latencyMs: number;
	
	    static createFrom(source: any = {}) {
	        return new EssentiaTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.runtimePath = source["runtimePath"];
	        this.modelDir = source["modelDir"];
	        this.base = this.convertValues(source["base"], ModelCheckResult);
	        this.genre = this.convertValues(source["genre"], ModelCheckResult);
	        this.heads = this.convertValues(source["heads"], ModelCheckResult);
	        this.message = source["message"];
	        this.latencyMs = source["latencyMs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MiniLMTestResult {
	    ok: boolean;
	    runtimePath: string;
	    modelDir: string;
	    modelPath: string;
	    tokenizerPath: string;
	    message: string;
	    latencyMs: number;
	    embeddingDim: number;
	
	    static createFrom(source: any = {}) {
	        return new MiniLMTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.runtimePath = source["runtimePath"];
	        this.modelDir = source["modelDir"];
	        this.modelPath = source["modelPath"];
	        this.tokenizerPath = source["tokenizerPath"];
	        this.message = source["message"];
	        this.latencyMs = source["latencyMs"];
	        this.embeddingDim = source["embeddingDim"];
	    }
	}
	
	export class RuntimeTestResult {
	    ok: boolean;
	    runtimePath: string;
	    message: string;
	    latencyMs: number;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.runtimePath = source["runtimePath"];
	        this.message = source["message"];
	        this.latencyMs = source["latencyMs"];
	    }
	}
	export class SettingsPayload {
	    onnxRuntimePath: string;
	    miniLMModelDir: string;
	    essentiaModelDir: string;
	    ffmpegPath: string;
	    ffprobePath: string;
	    storagePath: string;
	    repeatRay: boolean;
	    extendRay: boolean;
	    emoFlowUi: emoflow.UISettings;
	    normalizePodcastVolume: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.onnxRuntimePath = source["onnxRuntimePath"];
	        this.miniLMModelDir = source["miniLMModelDir"];
	        this.essentiaModelDir = source["essentiaModelDir"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffprobePath = source["ffprobePath"];
	        this.storagePath = source["storagePath"];
	        this.repeatRay = source["repeatRay"];
	        this.extendRay = source["extendRay"];
	        this.emoFlowUi = this.convertValues(source["emoFlowUi"], emoflow.UISettings);
	        this.normalizePodcastVolume = source["normalizePodcastVolume"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace onnx {
	
	export class GenreTag {
	    label: string;
	    detail?: string;
	    score: number;
	    rank: number;
	    support?: number;
	
	    static createFrom(source: any = {}) {
	        return new GenreTag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.detail = source["detail"];
	        this.score = source["score"];
	        this.rank = source["rank"];
	        this.support = source["support"];
	    }
	}

}

export namespace podcast {
	
	export class Item {
	    id: string;
	    path: string;
	    title: string;
	    author: string;
	    series: string;
	    folder: string;
	    duration: number;
	    fileSize: number;
	    transcriptPath: string;
	    transcriptStatus: string;
	    semanticStatus: string;
	    summary: string;
	    lastPosition: number;
	    completedRatio: number;
	    isCompleted: boolean;
	    playCount: number;
	    skipCount: number;
	    lastPlayedAt: number;
	    lastError: string;
	    importedAt: number;
	    modifiedAt: number;
	    sourceType: string;
	    sourceUrl: string;
	    sourceSite: string;
	    externalId: string;
	    downloadStatus: string;
	    downloadProgress: number;
	    downloadError: string;
	    downloadAttempts: number;
	    downloadedAt: number;
	    resumePosition: number;
	    durationLabel: string;
	    progressPercentage: number;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.series = source["series"];
	        this.folder = source["folder"];
	        this.duration = source["duration"];
	        this.fileSize = source["fileSize"];
	        this.transcriptPath = source["transcriptPath"];
	        this.transcriptStatus = source["transcriptStatus"];
	        this.semanticStatus = source["semanticStatus"];
	        this.summary = source["summary"];
	        this.lastPosition = source["lastPosition"];
	        this.completedRatio = source["completedRatio"];
	        this.isCompleted = source["isCompleted"];
	        this.playCount = source["playCount"];
	        this.skipCount = source["skipCount"];
	        this.lastPlayedAt = source["lastPlayedAt"];
	        this.lastError = source["lastError"];
	        this.importedAt = source["importedAt"];
	        this.modifiedAt = source["modifiedAt"];
	        this.sourceType = source["sourceType"];
	        this.sourceUrl = source["sourceUrl"];
	        this.sourceSite = source["sourceSite"];
	        this.externalId = source["externalId"];
	        this.downloadStatus = source["downloadStatus"];
	        this.downloadProgress = source["downloadProgress"];
	        this.downloadError = source["downloadError"];
	        this.downloadAttempts = source["downloadAttempts"];
	        this.downloadedAt = source["downloadedAt"];
	        this.resumePosition = source["resumePosition"];
	        this.durationLabel = source["durationLabel"];
	        this.progressPercentage = source["progressPercentage"];
	    }
	}
	export class HistoryItem {
	    id: string;
	    item: Item;
	    rayId: string;
	    startedAt: number;
	    endedAt: number;
	    startPosition: number;
	    endPosition: number;
	    listenedSeconds: number;
	    completedRatio: number;
	    source: string;
	    endReason: string;
	    playedAtLabel: string;
	    listenedLabel: string;
	    positionLabel: string;
	    progressPercent: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.item = this.convertValues(source["item"], Item);
	        this.rayId = source["rayId"];
	        this.startedAt = source["startedAt"];
	        this.endedAt = source["endedAt"];
	        this.startPosition = source["startPosition"];
	        this.endPosition = source["endPosition"];
	        this.listenedSeconds = source["listenedSeconds"];
	        this.completedRatio = source["completedRatio"];
	        this.source = source["source"];
	        this.endReason = source["endReason"];
	        this.playedAtLabel = source["playedAtLabel"];
	        this.listenedLabel = source["listenedLabel"];
	        this.positionLabel = source["positionLabel"];
	        this.progressPercent = source["progressPercent"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ImportResult {
	    inputCount: number;
	    audioFound: number;
	    addedOrUpdated: number;
	    skipped: number;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inputCount = source["inputCount"];
	        this.audioFound = source["audioFound"];
	        this.addedOrUpdated = source["addedOrUpdated"];
	        this.skipped = source["skipped"];
	        this.errors = source["errors"];
	    }
	}
	
	export class Playback {
	    itemId: string;
	    rayId: string;
	    queueIndex: number;
	    queueLength: number;
	    resumeMs: number;
	    durationMs: number;
	    title: string;
	    author: string;
	
	    static createFrom(source: any = {}) {
	        return new Playback(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.rayId = source["rayId"];
	        this.queueIndex = source["queueIndex"];
	        this.queueLength = source["queueLength"];
	        this.resumeMs = source["resumeMs"];
	        this.durationMs = source["durationMs"];
	        this.title = source["title"];
	        this.author = source["author"];
	    }
	}
	export class RayItem {
	    item: Item;
	    position: number;
	    originalPosition: number;
	    reason: string;
	    score: number;
	    semanticScore: number;
	    folderScore: number;
	    noveltyScore: number;
	    resumeScore: number;
	    addedBy: string;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RayItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.item = this.convertValues(source["item"], Item);
	        this.position = source["position"];
	        this.originalPosition = source["originalPosition"];
	        this.reason = source["reason"];
	        this.score = source["score"];
	        this.semanticScore = source["semanticScore"];
	        this.folderScore = source["folderScore"];
	        this.noveltyScore = source["noveltyScore"];
	        this.resumeScore = source["resumeScore"];
	        this.addedBy = source["addedBy"];
	        this.current = source["current"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RayFilters {
	    excludeCompleted: boolean;
	    excludeHardSkipped: boolean;
	    minSemanticSimilarity: number;
	    maxItems: number;
	
	    static createFrom(source: any = {}) {
	        return new RayFilters(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.excludeCompleted = source["excludeCompleted"];
	        this.excludeHardSkipped = source["excludeHardSkipped"];
	        this.minSemanticSimilarity = source["minSemanticSimilarity"];
	        this.maxItems = source["maxItems"];
	    }
	}
	export class RayWeights {
	    semanticSimilarity: number;
	    folderAffinity: number;
	    seriesAffinity: number;
	    resumeValue: number;
	    novelty: number;
	    freshness: number;
	    userTaste: number;
	    skipPenalty: number;
	    recentPenalty: number;
	    topicBridge: number;
	
	    static createFrom(source: any = {}) {
	        return new RayWeights(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.semanticSimilarity = source["semanticSimilarity"];
	        this.folderAffinity = source["folderAffinity"];
	        this.seriesAffinity = source["seriesAffinity"];
	        this.resumeValue = source["resumeValue"];
	        this.novelty = source["novelty"];
	        this.freshness = source["freshness"];
	        this.userTaste = source["userTaste"];
	        this.skipPenalty = source["skipPenalty"];
	        this.recentPenalty = source["recentPenalty"];
	        this.topicBridge = source["topicBridge"];
	    }
	}
	export class Scope {
	    seedFolder: string;
	    includeSubfolders: boolean;
	    preferSameFolder: boolean;
	    allowOutside: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Scope(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seedFolder = source["seedFolder"];
	        this.includeSubfolders = source["includeSubfolders"];
	        this.preferSameFolder = source["preferSameFolder"];
	        this.allowOutside = source["allowOutside"];
	    }
	}
	export class RayConfig {
	    contentMode: string;
	    sortMode: string;
	    seedItemId: string;
	    scope: Scope;
	    weights: RayWeights;
	    filters: RayFilters;
	    createdWithVersion: number;
	
	    static createFrom(source: any = {}) {
	        return new RayConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contentMode = source["contentMode"];
	        this.sortMode = source["sortMode"];
	        this.seedItemId = source["seedItemId"];
	        this.scope = this.convertValues(source["scope"], Scope);
	        this.weights = this.convertValues(source["weights"], RayWeights);
	        this.filters = this.convertValues(source["filters"], RayFilters);
	        this.createdWithVersion = source["createdWithVersion"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Ray {
	    id: string;
	    seedItemId: string;
	    title: string;
	    mode: string;
	    contentMode: string;
	    sortMode: string;
	    config: RayConfig;
	    isManualOrder: boolean;
	    manualUpdatedAt: number;
	    parentRayId: string;
	    revision: number;
	    createdAt: number;
	    folderScope: string;
	    currentIndex: number;
	    items: RayItem[];
	
	    static createFrom(source: any = {}) {
	        return new Ray(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.seedItemId = source["seedItemId"];
	        this.title = source["title"];
	        this.mode = source["mode"];
	        this.contentMode = source["contentMode"];
	        this.sortMode = source["sortMode"];
	        this.config = this.convertValues(source["config"], RayConfig);
	        this.isManualOrder = source["isManualOrder"];
	        this.manualUpdatedAt = source["manualUpdatedAt"];
	        this.parentRayId = source["parentRayId"];
	        this.revision = source["revision"];
	        this.createdAt = source["createdAt"];
	        this.folderScope = source["folderScope"];
	        this.currentIndex = source["currentIndex"];
	        this.items = this.convertValues(source["items"], RayItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class RayHistoryItem {
	    id: string;
	    seedItemId: string;
	    seed: Item;
	    title: string;
	    contentMode: string;
	    sortMode: string;
	    isManualOrder: boolean;
	    parentRayId: string;
	    revision: number;
	    folderScope: string;
	    itemCount: number;
	    createdAt: number;
	    createdAtLabel: string;
	
	    static createFrom(source: any = {}) {
	        return new RayHistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.seedItemId = source["seedItemId"];
	        this.seed = this.convertValues(source["seed"], Item);
	        this.title = source["title"];
	        this.contentMode = source["contentMode"];
	        this.sortMode = source["sortMode"];
	        this.isManualOrder = source["isManualOrder"];
	        this.parentRayId = source["parentRayId"];
	        this.revision = source["revision"];
	        this.folderScope = source["folderScope"];
	        this.itemCount = source["itemCount"];
	        this.createdAt = source["createdAt"];
	        this.createdAtLabel = source["createdAtLabel"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

export namespace rays {
	
	export class EmotionBasisInsight {
	    label?: string;
	    prevLabel?: string;
	    distance?: number;
	    smoothness?: number;
	    hardJump?: number;
	    bridgeScore?: number;
	    rawDistance?: number;
	    textureConfidence?: number;
	    edgeDrive?: number;
	    dirtyElectro?: number;
	    joy?: number;
	    melancholy?: number;
	    serenity?: number;
	    combat?: number;
	    pressure?: number;
	    roughness?: number;
	    swagger?: number;
	    sprintClean?: number;
	
	    static createFrom(source: any = {}) {
	        return new EmotionBasisInsight(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.prevLabel = source["prevLabel"];
	        this.distance = source["distance"];
	        this.smoothness = source["smoothness"];
	        this.hardJump = source["hardJump"];
	        this.bridgeScore = source["bridgeScore"];
	        this.rawDistance = source["rawDistance"];
	        this.textureConfidence = source["textureConfidence"];
	        this.edgeDrive = source["edgeDrive"];
	        this.dirtyElectro = source["dirtyElectro"];
	        this.joy = source["joy"];
	        this.melancholy = source["melancholy"];
	        this.serenity = source["serenity"];
	        this.combat = source["combat"];
	        this.pressure = source["pressure"];
	        this.roughness = source["roughness"];
	        this.swagger = source["swagger"];
	        this.sprintClean = source["sprintClean"];
	    }
	}
	export class QueueInsight {
	    similarity: number;
	    moodSmoothness: number;
	    moodDistance: number;
	    energyDelta: number;
	    jumpPenalty: number;
	    novelty: number;
	    tempoCompatibility: number;
	    tempoUnknown?: boolean;
	    textureContinuity: number;
	    vocalContinuity: number;
	    sessionFit: number;
	    targetMoodFit: number;
	    mode: string;
	    bucket?: string;
	    strategy?: string;
	    score?: number;
	    transition?: string;
	    energyDirection?: string;
	    discovery: boolean;
	    bridge: boolean;
	    confidence?: number;
	    fallback?: string;
	    warning?: string;
	    lowTrustFeatures?: string[];
	    emotion?: EmotionBasisInsight;
	
	    static createFrom(source: any = {}) {
	        return new QueueInsight(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.similarity = source["similarity"];
	        this.moodSmoothness = source["moodSmoothness"];
	        this.moodDistance = source["moodDistance"];
	        this.energyDelta = source["energyDelta"];
	        this.jumpPenalty = source["jumpPenalty"];
	        this.novelty = source["novelty"];
	        this.tempoCompatibility = source["tempoCompatibility"];
	        this.tempoUnknown = source["tempoUnknown"];
	        this.textureContinuity = source["textureContinuity"];
	        this.vocalContinuity = source["vocalContinuity"];
	        this.sessionFit = source["sessionFit"];
	        this.targetMoodFit = source["targetMoodFit"];
	        this.mode = source["mode"];
	        this.bucket = source["bucket"];
	        this.strategy = source["strategy"];
	        this.score = source["score"];
	        this.transition = source["transition"];
	        this.energyDirection = source["energyDirection"];
	        this.discovery = source["discovery"];
	        this.bridge = source["bridge"];
	        this.confidence = source["confidence"];
	        this.fallback = source["fallback"];
	        this.warning = source["warning"];
	        this.lowTrustFeatures = source["lowTrustFeatures"];
	        this.emotion = this.convertValues(source["emotion"], EmotionBasisInsight);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QueueItem {
	    trackId: string;
	    title: string;
	    subtitle: string;
	    durationLabel: string;
	    isCurrent: boolean;
	    reason: string;
	    insight: QueueInsight;
	    artist: string;
	    album?: string;
	    genrePrimary: string;
	    genreLabel: string;
	    genreDetail: string;
	    genreTags: onnx.GenreTag[];
	    durationMs: number;
	    position: number;
	    originalPosition: number;
	    rayRole: string;
	    rayReason: string;
	    track: library.Track;
	
	    static createFrom(source: any = {}) {
	        return new QueueItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trackId = source["trackId"];
	        this.title = source["title"];
	        this.subtitle = source["subtitle"];
	        this.durationLabel = source["durationLabel"];
	        this.isCurrent = source["isCurrent"];
	        this.reason = source["reason"];
	        this.insight = this.convertValues(source["insight"], QueueInsight);
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.genrePrimary = source["genrePrimary"];
	        this.genreLabel = source["genreLabel"];
	        this.genreDetail = source["genreDetail"];
	        this.genreTags = this.convertValues(source["genreTags"], onnx.GenreTag);
	        this.durationMs = source["durationMs"];
	        this.position = source["position"];
	        this.originalPosition = source["originalPosition"];
	        this.rayRole = source["rayRole"];
	        this.rayReason = source["rayReason"];
	        this.track = this.convertValues(source["track"], library.Track);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Ray {
	    id: string;
	    name: string;
	    seedTrackId: string;
	    currentTrackId: string;
	    queue: QueueItem[];
	    resumeLabel: string;
	    positionMs: number;
	    contentMode: string;
	    sortMode: string;
	    isManualOrder: boolean;
	    manualUpdatedAt: number;
	    parentRayId: string;
	    revision: number;
	
	    static createFrom(source: any = {}) {
	        return new Ray(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.seedTrackId = source["seedTrackId"];
	        this.currentTrackId = source["currentTrackId"];
	        this.queue = this.convertValues(source["queue"], QueueItem);
	        this.resumeLabel = source["resumeLabel"];
	        this.positionMs = source["positionMs"];
	        this.contentMode = source["contentMode"];
	        this.sortMode = source["sortMode"];
	        this.isManualOrder = source["isManualOrder"];
	        this.manualUpdatedAt = source["manualUpdatedAt"];
	        this.parentRayId = source["parentRayId"];
	        this.revision = source["revision"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RaySummary {
	    id: string;
	    name: string;
	    trackCount: number;
	    currentTrackId: string;
	    currentTrackName: string;
	    resumeLabel: string;
	    positionMs: number;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RaySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.trackCount = source["trackCount"];
	        this.currentTrackId = source["currentTrackId"];
	        this.currentTrackName = source["currentTrackName"];
	        this.resumeLabel = source["resumeLabel"];
	        this.positionMs = source["positionMs"];
	        this.active = source["active"];
	    }
	}

}

export namespace recommend {
	
	export class RayAuditSummary {
	    totalTracks: number;
	    coreCount: number;
	    bridgeCount: number;
	    adjacentCount: number;
	    discoveryCount: number;
	    avgConfidence: number;
	    avgNovelty: number;
	    topStrategies: Record<string, number>;
	    topTransitions: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new RayAuditSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalTracks = source["totalTracks"];
	        this.coreCount = source["coreCount"];
	        this.bridgeCount = source["bridgeCount"];
	        this.adjacentCount = source["adjacentCount"];
	        this.discoveryCount = source["discoveryCount"];
	        this.avgConfidence = source["avgConfidence"];
	        this.avgNovelty = source["avgNovelty"];
	        this.topStrategies = source["topStrategies"];
	        this.topTransitions = source["topTransitions"];
	    }
	}
	export class RayAuditRow {
	    position: number;
	    trackId: string;
	    title: string;
	    reason: string;
	    bucket: string;
	    strategy: string;
	    score: number;
	    insight: rays.QueueInsight;
	    emotionLabel?: string;
	    emotionFamily?: string;
	    emotionDistance?: number;
	    hardJumpRisk?: number;
	    bridgeScore?: number;
	    familyPenalty?: number;
	
	    static createFrom(source: any = {}) {
	        return new RayAuditRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.position = source["position"];
	        this.trackId = source["trackId"];
	        this.title = source["title"];
	        this.reason = source["reason"];
	        this.bucket = source["bucket"];
	        this.strategy = source["strategy"];
	        this.score = source["score"];
	        this.insight = this.convertValues(source["insight"], rays.QueueInsight);
	        this.emotionLabel = source["emotionLabel"];
	        this.emotionFamily = source["emotionFamily"];
	        this.emotionDistance = source["emotionDistance"];
	        this.hardJumpRisk = source["hardJumpRisk"];
	        this.bridgeScore = source["bridgeScore"];
	        this.familyPenalty = source["familyPenalty"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RayAuditResult {
	    seedTrackId: string;
	    mode: string;
	    rows: RayAuditRow[];
	    summary: RayAuditSummary;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new RayAuditResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seedTrackId = source["seedTrackId"];
	        this.mode = source["mode"];
	        this.rows = this.convertValues(source["rows"], RayAuditRow);
	        this.summary = this.convertValues(source["summary"], RayAuditSummary);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace search {
	
	export class Result {
	    track: library.Track;
	    score: number;
	    explanation: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.track = this.convertValues(source["track"], library.Track);
	        this.score = source["score"];
	        this.explanation = source["explanation"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

