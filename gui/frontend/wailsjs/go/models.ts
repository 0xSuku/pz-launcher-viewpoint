export namespace main {
	
	export class SettingsPayload {
	    chunkBudgetMs: number;
	    maxHeapMB: number;
	    maxChunkGridWidth: number;
	    safeTowOff: boolean;
	    ramGB: number;
	    suggestHeapMB: number;
	    defaultHeapMB: number;
	    configured: boolean;
	    hasGameJSON: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chunkBudgetMs = source["chunkBudgetMs"];
	        this.maxHeapMB = source["maxHeapMB"];
	        this.maxChunkGridWidth = source["maxChunkGridWidth"];
	        this.safeTowOff = source["safeTowOff"];
	        this.ramGB = source["ramGB"];
	        this.suggestHeapMB = source["suggestHeapMB"];
	        this.defaultHeapMB = source["defaultHeapMB"];
	        this.configured = source["configured"];
	        this.hasGameJSON = source["hasGameJSON"];
	    }
	}
	export class StatePayload {
	    ready: boolean;
	    offline: boolean;
	    version: string;
	    gameDir: string;
	    error: string;
	    lastNote: string;
	    patchNotes: manifest.PatchNote[];
	
	    static createFrom(source: any = {}) {
	        return new StatePayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ready = source["ready"];
	        this.offline = source["offline"];
	        this.version = source["version"];
	        this.gameDir = source["gameDir"];
	        this.error = source["error"];
	        this.lastNote = source["lastNote"];
	        this.patchNotes = this.convertValues(source["patchNotes"], manifest.PatchNote);
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
	export class UninstallResult {
	    status: viewpoint.Status;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new UninstallResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = this.convertValues(source["status"], viewpoint.Status);
	        this.message = source["message"];
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

export namespace manifest {
	
	export class PatchNote {
	    id?: string;
	    title: string;
	    desc?: string;
	    date?: string;
	
	    static createFrom(source: any = {}) {
	        return new PatchNote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.desc = source["desc"];
	        this.date = source["date"];
	    }
	}

}

export namespace viewpoint {
	
	export class Status {
	    supported: boolean;
	    enabled: boolean;
	    autoUpdate: boolean;
	    gameFound: boolean;
	    zombieBuddySubscribed: boolean;
	    viewpointSubscribed: boolean;
	    agent: string;
	    installedVersion: string;
	    availableVersion: string;
	    installedExternally: boolean;
	    modsActive: boolean;
	    zombieBuddyPresent: boolean;
	    steamOptionsAgent: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.enabled = source["enabled"];
	        this.autoUpdate = source["autoUpdate"];
	        this.gameFound = source["gameFound"];
	        this.zombieBuddySubscribed = source["zombieBuddySubscribed"];
	        this.viewpointSubscribed = source["viewpointSubscribed"];
	        this.agent = source["agent"];
	        this.installedVersion = source["installedVersion"];
	        this.availableVersion = source["availableVersion"];
	        this.installedExternally = source["installedExternally"];
	        this.modsActive = source["modsActive"];
	        this.zombieBuddyPresent = source["zombieBuddyPresent"];
	        this.steamOptionsAgent = source["steamOptionsAgent"];
	    }
	}

}

