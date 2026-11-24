export namespace aruba {
	
	export class ApplyResult {
	    ok: boolean;
	    transcript: string[];
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.transcript = source["transcript"];
	        this.error = source["error"];
	    }
	}
	export class Banks {
	    primary: string;
	    secondary: string;
	    running: string;
	    default: string;
	    known: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Banks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.primary = source["primary"];
	        this.secondary = source["secondary"];
	        this.running = source["running"];
	        this.default = source["default"];
	        this.known = source["known"];
	    }
	}
	export class CablePair {
	    pair: string;
	    status: string;
	    ok: boolean;
	    impedance?: string;
	    distance?: string;
	    mdiMode?: string;
	
	    static createFrom(source: any = {}) {
	        return new CablePair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pair = source["pair"];
	        this.status = source["status"];
	        this.ok = source["ok"];
	        this.impedance = source["impedance"];
	        this.distance = source["distance"];
	        this.mdiMode = source["mdiMode"];
	    }
	}
	export class CableDiagResult {
	    port: string;
	    supported: boolean;
	    health: string;
	    summary: string;
	    pairs: CablePair[];
	    raw: string;
	    // Go type: time
	    testedAt: any;
	    commands: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CableDiagResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.supported = source["supported"];
	        this.health = source["health"];
	        this.summary = source["summary"];
	        this.pairs = this.convertValues(source["pairs"], CablePair);
	        this.raw = source["raw"];
	        this.testedAt = this.convertValues(source["testedAt"], null);
	        this.commands = source["commands"];
	        this.error = source["error"];
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
	
	export class CloudAlert {
	    severity: string;
	    type: string;
	    text: string;
	    // Go type: time
	    raised: any;
	
	    static createFrom(source: any = {}) {
	        return new CloudAlert(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.type = source["type"];
	        this.text = source["text"];
	        this.raised = this.convertValues(source["raised"], null);
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
	export class CloudInfo {
	    site: string;
	    health: string;
	    upToDate?: boolean;
	    firmware: string;
	    clients: number;
	    poeUsedW: number;
	    poeBudgetW: number;
	    alerts: CloudAlert[];
	
	    static createFrom(source: any = {}) {
	        return new CloudInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site = source["site"];
	        this.health = source["health"];
	        this.upToDate = source["upToDate"];
	        this.firmware = source["firmware"];
	        this.clients = source["clients"];
	        this.poeUsedW = source["poeUsedW"];
	        this.poeBudgetW = source["poeBudgetW"];
	        this.alerts = this.convertValues(source["alerts"], CloudAlert);
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
	export class DanteProfileChange {
	    vlanIds: number[];
	    ports: string[];
	    globalTrust: boolean;
	    portTrust?: boolean;
	    dscpMap: boolean;
	    strictSchedule: boolean;
	    igmpSnooping: boolean;
	    igmpQuerier: boolean;
	    disableEee: boolean;
	    jumboMtu?: boolean;
	    save: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DanteProfileChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vlanIds = source["vlanIds"];
	        this.ports = source["ports"];
	        this.globalTrust = source["globalTrust"];
	        this.portTrust = source["portTrust"];
	        this.dscpMap = source["dscpMap"];
	        this.strictSchedule = source["strictSchedule"];
	        this.igmpSnooping = source["igmpSnooping"];
	        this.igmpQuerier = source["igmpQuerier"];
	        this.disableEee = source["disableEee"];
	        this.jumboMtu = source["jumboMtu"];
	        this.save = source["save"];
	    }
	}
	export class EnvItem {
	    kind: string;
	    name: string;
	    status: string;
	    ok: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EnvItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.ok = source["ok"];
	    }
	}
	export class Facts {
	    hostname: string;
	    model: string;
	    serial: string;
	    baseMac: string;
	    software: string;
	    uptime: string;
	    cpu: number;
	    cpu1m?: number;
	    cpu5m?: number;
	    cpuInstant?: number;
	    memory: number;
	    location: string;
	    contact: string;
	    configSaved?: boolean;
	    bootRom?: string;
	    powerUsedW?: number;
	    powerMaxW?: number;
	
	    static createFrom(source: any = {}) {
	        return new Facts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostname = source["hostname"];
	        this.model = source["model"];
	        this.serial = source["serial"];
	        this.baseMac = source["baseMac"];
	        this.software = source["software"];
	        this.uptime = source["uptime"];
	        this.cpu = source["cpu"];
	        this.cpu1m = source["cpu1m"];
	        this.cpu5m = source["cpu5m"];
	        this.cpuInstant = source["cpuInstant"];
	        this.memory = source["memory"];
	        this.location = source["location"];
	        this.contact = source["contact"];
	        this.configSaved = source["configSaved"];
	        this.bootRom = source["bootRom"];
	        this.powerUsedW = source["powerUsedW"];
	        this.powerMaxW = source["powerMaxW"];
	    }
	}
	export class LogLine {
	    severity: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new LogLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.text = source["text"];
	    }
	}
	export class MACEntry {
	    mac: string;
	    port: string;
	    vlan: string;
	    vendor?: string;
	
	    static createFrom(source: any = {}) {
	        return new MACEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mac = source["mac"];
	        this.port = source["port"];
	        this.vlan = source["vlan"];
	        this.vendor = source["vendor"];
	    }
	}
	export class Neighbor {
	    protocol: string;
	    localPort: string;
	    sysName: string;
	    chassisId: string;
	    portId: string;
	    portDescr: string;
	    mgmtIp: string;
	    description: string;
	    capabilities: string;
	
	    static createFrom(source: any = {}) {
	        return new Neighbor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.localPort = source["localPort"];
	        this.sysName = source["sysName"];
	        this.chassisId = source["chassisId"];
	        this.portId = source["portId"];
	        this.portDescr = source["portDescr"];
	        this.mgmtIp = source["mgmtIp"];
	        this.description = source["description"];
	        this.capabilities = source["capabilities"];
	    }
	}
	export class Note {
	    severity: string;
	    category: string;
	    title: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Note(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.category = source["category"];
	        this.title = source["title"];
	        this.detail = source["detail"];
	    }
	}
	export class Optic {
	    port: string;
	    type: string;
	    model?: string;
	    serial?: string;
	    wavelength?: string;
	    dom: boolean;
	    tempC?: number;
	    voltage?: number;
	    biasMa?: number;
	    rx?: number[];
	    tx?: number[];
	    rxLowAlarm?: number;
	    rxLowWarn?: number;
	    rxHighWarn?: number;
	    rxHighAlarm?: number;
	    txLowAlarm?: number;
	    txLowWarn?: number;
	    alarms?: string[];
	    recent?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Optic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.type = source["type"];
	        this.model = source["model"];
	        this.serial = source["serial"];
	        this.wavelength = source["wavelength"];
	        this.dom = source["dom"];
	        this.tempC = source["tempC"];
	        this.voltage = source["voltage"];
	        this.biasMa = source["biasMa"];
	        this.rx = source["rx"];
	        this.tx = source["tx"];
	        this.rxLowAlarm = source["rxLowAlarm"];
	        this.rxLowWarn = source["rxLowWarn"];
	        this.rxHighWarn = source["rxHighWarn"];
	        this.rxHighAlarm = source["rxHighAlarm"];
	        this.txLowAlarm = source["txLowAlarm"];
	        this.txLowWarn = source["txLowWarn"];
	        this.alarms = source["alarms"];
	        this.recent = source["recent"];
	    }
	}
	export class Port {
	    name: string;
	    enabled: boolean;
	    up: boolean;
	    speed: string;
	    mode: string;
	    vlan: string;
	    description: string;
	    type: string;
	    trunk?: string;
	    cage?: string;
	    qosTrust?: string;
	    eeeOff?: boolean;
	    eeeOn?: boolean;
	    poeMw?: number;
	    poeOff?: boolean;
	    loop?: boolean;
	    uplink?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Port(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.up = source["up"];
	        this.speed = source["speed"];
	        this.mode = source["mode"];
	        this.vlan = source["vlan"];
	        this.description = source["description"];
	        this.type = source["type"];
	        this.trunk = source["trunk"];
	        this.cage = source["cage"];
	        this.qosTrust = source["qosTrust"];
	        this.eeeOff = source["eeeOff"];
	        this.eeeOn = source["eeeOn"];
	        this.poeMw = source["poeMw"];
	        this.poeOff = source["poeOff"];
	        this.loop = source["loop"];
	        this.uplink = source["uplink"];
	    }
	}
	export class PortClient {
	    port: string;
	    mac: string;
	    user?: string;
	    ip?: string;
	    method?: string;
	    status?: string;
	    role?: string;
	    roleStatus?: string;
	    vlan?: string;
	    tagged?: string;
	    session?: string;
	
	    static createFrom(source: any = {}) {
	        return new PortClient(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.mac = source["mac"];
	        this.user = source["user"];
	        this.ip = source["ip"];
	        this.method = source["method"];
	        this.status = source["status"];
	        this.role = source["role"];
	        this.roleStatus = source["roleStatus"];
	        this.vlan = source["vlan"];
	        this.tagged = source["tagged"];
	        this.session = source["session"];
	    }
	}
	export class PortAuth {
	    port: string;
	    dot1x: boolean;
	    macAuth: boolean;
	    clientLimit?: number;
	    reauthPeriod?: number;
	    unauthVlan?: number;
	    roles?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new PortAuth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.dot1x = source["dot1x"];
	        this.macAuth = source["macAuth"];
	        this.clientLimit = source["clientLimit"];
	        this.reauthPeriod = source["reauthPeriod"];
	        this.unauthVlan = source["unauthVlan"];
	        this.roles = source["roles"];
	    }
	}
	export class PortRole {
	    name: string;
	    description?: string;
	    vlanMode?: string;
	    vlan?: number;
	    vlanName?: string;
	    tagged?: number[];
	
	    static createFrom(source: any = {}) {
	        return new PortRole(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.vlanMode = source["vlanMode"];
	        this.vlan = source["vlan"];
	        this.vlanName = source["vlanName"];
	        this.tagged = source["tagged"];
	    }
	}
	export class RadiusServer {
	    host: string;
	    vrf?: string;
	    hasKey: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RadiusServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.vrf = source["vrf"];
	        this.hasKey = source["hasKey"];
	    }
	}
	export class PortAccess {
	    servers: RadiusServer[];
	    roles: PortRole[];
	    dot1xOn: boolean;
	    macAuthOn: boolean;
	    ports: PortAuth[];
	    clients?: PortClient[];
	    clientsRead: boolean;
	    // Go type: time
	    clientsAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PortAccess(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.servers = this.convertValues(source["servers"], RadiusServer);
	        this.roles = this.convertValues(source["roles"], PortRole);
	        this.dot1xOn = source["dot1xOn"];
	        this.macAuthOn = source["macAuthOn"];
	        this.ports = this.convertValues(source["ports"], PortAuth);
	        this.clients = this.convertValues(source["clients"], PortClient);
	        this.clientsRead = source["clientsRead"];
	        this.clientsAt = this.convertValues(source["clientsAt"], null);
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
	export class RadiusServerChange {
	    host: string;
	    key: string;
	    vrf: string;
	
	    static createFrom(source: any = {}) {
	        return new RadiusServerChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.key = source["key"];
	        this.vrf = source["vrf"];
	    }
	}
	export class PortAccessChange {
	    addServer?: RadiusServerChange;
	    removeServer: string;
	    ports: string[];
	    dot1x?: boolean;
	    macAuth?: boolean;
	    clientLimit?: number;
	    reauthPeriod?: number;
	    unauthVlan?: number;
	    roles: Record<string, string>;
	    setRole?: PortRole;
	    deleteRole: string;
	    globalDot1x?: boolean;
	    globalMacAuth?: boolean;
	    save: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortAccessChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.addServer = this.convertValues(source["addServer"], RadiusServerChange);
	        this.removeServer = source["removeServer"];
	        this.ports = source["ports"];
	        this.dot1x = source["dot1x"];
	        this.macAuth = source["macAuth"];
	        this.clientLimit = source["clientLimit"];
	        this.reauthPeriod = source["reauthPeriod"];
	        this.unauthVlan = source["unauthVlan"];
	        this.roles = source["roles"];
	        this.setRole = this.convertValues(source["setRole"], PortRole);
	        this.deleteRole = source["deleteRole"];
	        this.globalDot1x = source["globalDot1x"];
	        this.globalMacAuth = source["globalMacAuth"];
	        this.save = source["save"];
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
	export class PortAdminChange {
	    ports: string[];
	    action: string;
	    save: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortAdminChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ports = source["ports"];
	        this.action = source["action"];
	        this.save = source["save"];
	    }
	}
	
	
	export class PortNameChange {
	    names: Record<string, string>;
	    lldp?: string[];
	    save: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortNameChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.names = source["names"];
	        this.lldp = source["lldp"];
	        this.save = source["save"];
	    }
	}
	export class PortNameSuggestion {
	    port: string;
	    current: string;
	    name: string;
	    neighbor: string;
	    replaces?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortNameSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.current = source["current"];
	        this.name = source["name"];
	        this.neighbor = source["neighbor"];
	        this.replaces = source["replaces"];
	    }
	}
	export class PortPoEChange {
	    ports: string[];
	    action: string;
	    save: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortPoEChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ports = source["ports"];
	        this.action = source["action"];
	        this.save = source["save"];
	    }
	}
	
	export class PortVLANChange {
	    ports: string[];
	    mode: string;
	    accessVlan: number;
	    taggedVlans: number[];
	    tagAll: boolean;
	    qosTrust?: string;
	    disableEee?: boolean;
	    save: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PortVLANChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ports = source["ports"];
	        this.mode = source["mode"];
	        this.accessVlan = source["accessVlan"];
	        this.taggedVlans = source["taggedVlans"];
	        this.tagAll = source["tagAll"];
	        this.qosTrust = source["qosTrust"];
	        this.disableEee = source["disableEee"];
	        this.save = source["save"];
	    }
	}
	export class QoSInfo {
	    known?: boolean;
	    globalTrust?: string;
	    dscp56Priority: number;
	    dscp46Priority: number;
	    scheduleProfile?: string;
	    strictTopQueues?: boolean;
	    globalEeeOff?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QoSInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.known = source["known"];
	        this.globalTrust = source["globalTrust"];
	        this.dscp56Priority = source["dscp56Priority"];
	        this.dscp46Priority = source["dscp46Priority"];
	        this.scheduleProfile = source["scheduleProfile"];
	        this.strictTopQueues = source["strictTopQueues"];
	        this.globalEeeOff = source["globalEeeOff"];
	    }
	}
	
	
	export class STPPort {
	    port: string;
	    role: string;
	    state: string;
	    cost: string;
	
	    static createFrom(source: any = {}) {
	        return new STPPort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.role = source["role"];
	        this.state = source["state"];
	        this.cost = source["cost"];
	    }
	}
	export class STP {
	    enabled: boolean;
	    known: boolean;
	    mode: string;
	    bridgePriority: string;
	    bridgeMac: string;
	    rootPriority: string;
	    rootMac: string;
	    rootPort: string;
	    isRoot: boolean;
	    topologyChanges: number;
	    lastChange: string;
	    ports: STPPort[];
	
	    static createFrom(source: any = {}) {
	        return new STP(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.known = source["known"];
	        this.mode = source["mode"];
	        this.bridgePriority = source["bridgePriority"];
	        this.bridgeMac = source["bridgeMac"];
	        this.rootPriority = source["rootPriority"];
	        this.rootMac = source["rootMac"];
	        this.rootPort = source["rootPort"];
	        this.isRoot = source["isRoot"];
	        this.topologyChanges = source["topologyChanges"];
	        this.lastChange = source["lastChange"];
	        this.ports = this.convertValues(source["ports"], STPPort);
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
	
	export class Security {
	    sshEnabled: boolean;
	    sshVersion: string;
	    sshPort: string;
	    telnetEnabled: boolean;
	    httpPlain: boolean;
	    httpsEnabled: boolean;
	    snmpCommunities: string[];
	    snmpDefault: boolean;
	    ntpServers: string[];
	    logServers: string[];
	    aaa: string[];
	    managerPassword: boolean;
	    timezone: string;
	    sshCiphers?: string[];
	    sshMacs?: string[];
	    sshKex?: string[];
	    telnetPort: string;
	    httpPort: string;
	
	    static createFrom(source: any = {}) {
	        return new Security(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sshEnabled = source["sshEnabled"];
	        this.sshVersion = source["sshVersion"];
	        this.sshPort = source["sshPort"];
	        this.telnetEnabled = source["telnetEnabled"];
	        this.httpPlain = source["httpPlain"];
	        this.httpsEnabled = source["httpsEnabled"];
	        this.snmpCommunities = source["snmpCommunities"];
	        this.snmpDefault = source["snmpDefault"];
	        this.ntpServers = source["ntpServers"];
	        this.logServers = source["logServers"];
	        this.aaa = source["aaa"];
	        this.managerPassword = source["managerPassword"];
	        this.timezone = source["timezone"];
	        this.sshCiphers = source["sshCiphers"];
	        this.sshMacs = source["sshMacs"];
	        this.sshKex = source["sshKex"];
	        this.telnetPort = source["telnetPort"];
	        this.httpPort = source["httpPort"];
	    }
	}
	export class StackMember {
	    id: string;
	    mac: string;
	    model: string;
	    status: string;
	    ok: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StackMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.mac = source["mac"];
	        this.model = source["model"];
	        this.status = source["status"];
	        this.ok = source["ok"];
	    }
	}
	export class Stack {
	    topology: string;
	    status: string;
	    members: StackMember[];
	
	    static createFrom(source: any = {}) {
	        return new Stack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.topology = source["topology"];
	        this.status = source["status"];
	        this.members = this.convertValues(source["members"], StackMember);
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
	export class VLAN {
	    id: number;
	    name: string;
	    status: string;
	    tagged: string[];
	    untagged: string[];
	    ip: string;
	    igmpSnooping?: boolean;
	    igmpQuerier?: boolean;
	    igmpKnown?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VLAN(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.tagged = source["tagged"];
	        this.untagged = source["untagged"];
	        this.ip = source["ip"];
	        this.igmpSnooping = source["igmpSnooping"];
	        this.igmpQuerier = source["igmpQuerier"];
	        this.igmpKnown = source["igmpKnown"];
	    }
	}
	export class Snapshot {
	    family: string;
	    facts: Facts;
	    stp: STP;
	    security: Security;
	    vlans: VLAN[];
	    ports: Port[];
	    neighbors: Neighbor[];
	    env: EnvItem[];
	    logs: LogLine[];
	    raw: Record<string, string>;
	    runningConfig: string;
	    // Go type: time
	    collectedAt: any;
	    duration: string;
	    errors: string[];
	    banks: Banks;
	    cloud?: CloudInfo;
	    notes?: Note[];
	    optics?: Optic[];
	    portAccess?: PortAccess;
	    web?: boolean;
	    macs?: MACEntry[];
	    stack?: Stack;
	    qos?: QoSInfo;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.family = source["family"];
	        this.facts = this.convertValues(source["facts"], Facts);
	        this.stp = this.convertValues(source["stp"], STP);
	        this.security = this.convertValues(source["security"], Security);
	        this.vlans = this.convertValues(source["vlans"], VLAN);
	        this.ports = this.convertValues(source["ports"], Port);
	        this.neighbors = this.convertValues(source["neighbors"], Neighbor);
	        this.env = this.convertValues(source["env"], EnvItem);
	        this.logs = this.convertValues(source["logs"], LogLine);
	        this.raw = source["raw"];
	        this.runningConfig = source["runningConfig"];
	        this.collectedAt = this.convertValues(source["collectedAt"], null);
	        this.duration = source["duration"];
	        this.errors = source["errors"];
	        this.banks = this.convertValues(source["banks"], Banks);
	        this.cloud = this.convertValues(source["cloud"], CloudInfo);
	        this.notes = this.convertValues(source["notes"], Note);
	        this.optics = this.convertValues(source["optics"], Optic);
	        this.portAccess = this.convertValues(source["portAccess"], PortAccess);
	        this.web = source["web"];
	        this.macs = this.convertValues(source["macs"], MACEntry);
	        this.stack = this.convertValues(source["stack"], Stack);
	        this.qos = this.convertValues(source["qos"], QoSInfo);
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
	
	
	export class TemplatePort {
	    name: string;
	    description?: string;
	    mode?: string;
	    nativeVlan?: number;
	    taggedVlans?: number[];
	    qosTrust?: string;
	    eeeOff?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TemplatePort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.mode = source["mode"];
	        this.nativeVlan = source["nativeVlan"];
	        this.taggedVlans = source["taggedVlans"];
	        this.qosTrust = source["qosTrust"];
	        this.eeeOff = source["eeeOff"];
	    }
	}
	export class TemplateVLAN {
	    id: number;
	    name: string;
	    igmpSnooping?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TemplateVLAN(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.igmpSnooping = source["igmpSnooping"];
	    }
	}
	export class SwitchTemplate {
	    id: string;
	    name: string;
	    notes?: string;
	    sourceModel?: string;
	    sourceFamily?: string;
	    // Go type: time
	    createdAt: any;
	    vlans: TemplateVLAN[];
	    ports?: TemplatePort[];
	    danteQos?: boolean;
	    jumboMtu?: boolean;
	    stpEnabled?: boolean;
	    timezone?: string;
	    ntpServers?: string[];
	
	    static createFrom(source: any = {}) {
	        return new SwitchTemplate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.notes = source["notes"];
	        this.sourceModel = source["sourceModel"];
	        this.sourceFamily = source["sourceFamily"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.vlans = this.convertValues(source["vlans"], TemplateVLAN);
	        this.ports = this.convertValues(source["ports"], TemplatePort);
	        this.danteQos = source["danteQos"];
	        this.jumboMtu = source["jumboMtu"];
	        this.stpEnabled = source["stpEnabled"];
	        this.timezone = source["timezone"];
	        this.ntpServers = source["ntpServers"];
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
	export class TemplateApplyRequest {
	    templateId: string;
	    newHostname?: string;
	    includeVlans: boolean;
	    includeIgmp: boolean;
	    igmpQuerier: boolean;
	    includeDanteQos: boolean;
	    jumboMtu: boolean;
	    includePortVlans: boolean;
	    includePortNames: boolean;
	    includeSystem: boolean;
	    skipPorts?: string[];
	    save: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TemplateApplyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.templateId = source["templateId"];
	        this.newHostname = source["newHostname"];
	        this.includeVlans = source["includeVlans"];
	        this.includeIgmp = source["includeIgmp"];
	        this.igmpQuerier = source["igmpQuerier"];
	        this.includeDanteQos = source["includeDanteQos"];
	        this.jumboMtu = source["jumboMtu"];
	        this.includePortVlans = source["includePortVlans"];
	        this.includePortNames = source["includePortNames"];
	        this.includeSystem = source["includeSystem"];
	        this.skipPorts = source["skipPorts"];
	        this.save = source["save"];
	    }
	}
	
	
	
	export class VLANChange {
	    action: string;
	    id: number;
	    name: string;
	    addTagged: string[];
	    addUntagged: string[];
	    removePorts: string[];
	    igmpSnooping?: boolean;
	    igmpQuerier?: boolean;
	    save: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VLANChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.addTagged = source["addTagged"];
	        this.addUntagged = source["addUntagged"];
	        this.removePorts = source["removePorts"];
	        this.igmpSnooping = source["igmpSnooping"];
	        this.igmpQuerier = source["igmpQuerier"];
	        this.save = source["save"];
	    }
	}

}

export namespace core {
	
	export class ClientEntry {
	    switchId: string;
	    switchName: string;
	    port: string;
	    portName?: string;
	    vlan: string;
	    mac: string;
	    vendor?: string;
	    uplink?: string;
	    edge: boolean;
	    others: number;
	    seen: number;
	
	    static createFrom(source: any = {}) {
	        return new ClientEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.switchId = source["switchId"];
	        this.switchName = source["switchName"];
	        this.port = source["port"];
	        this.portName = source["portName"];
	        this.vlan = source["vlan"];
	        this.mac = source["mac"];
	        this.vendor = source["vendor"];
	        this.uplink = source["uplink"];
	        this.edge = source["edge"];
	        this.others = source["others"];
	        this.seen = source["seen"];
	    }
	}
	export class CloudPick {
	    siteId: string;
	    siteName: string;
	    deviceId: string;
	    name: string;
	    ip: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudPick(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.siteId = source["siteId"];
	        this.siteName = source["siteName"];
	        this.deviceId = source["deviceId"];
	        this.name = source["name"];
	        this.ip = source["ip"];
	    }
	}
	export class CloudSwitch {
	    deviceId: string;
	    name: string;
	    model: string;
	    ip: string;
	    online: boolean;
	    ports: number;
	    addedAs?: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudSwitch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceId = source["deviceId"];
	        this.name = source["name"];
	        this.model = source["model"];
	        this.ip = source["ip"];
	        this.online = source["online"];
	        this.ports = source["ports"];
	        this.addedAs = source["addedAs"];
	    }
	}
	export class CloudSite {
	    id: string;
	    name: string;
	    error?: string;
	    switches: CloudSwitch[];
	
	    static createFrom(source: any = {}) {
	        return new CloudSite(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.error = source["error"];
	        this.switches = this.convertValues(source["switches"], CloudSwitch);
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
	
	export class ConfigMatchLine {
	    line: number;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigMatchLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.text = source["text"];
	    }
	}
	export class ConfigSearchHit {
	    switchId: string;
	    switchName: string;
	    host: string;
	    site?: string;
	    family?: string;
	    totalLines: number;
	    lines: ConfigMatchLine[];
	
	    static createFrom(source: any = {}) {
	        return new ConfigSearchHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.switchId = source["switchId"];
	        this.switchName = source["switchName"];
	        this.host = source["host"];
	        this.site = source["site"];
	        this.family = source["family"];
	        this.totalLines = source["totalLines"];
	        this.lines = this.convertValues(source["lines"], ConfigMatchLine);
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
	export class PortLED {
	    name: string;
	    up: boolean;
	    enabled: boolean;
	    blocking: boolean;
	    type: string;
	    speed: string;
	    description?: string;
	    trunk?: string;
	    cage?: string;
	
	    static createFrom(source: any = {}) {
	        return new PortLED(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.up = source["up"];
	        this.enabled = source["enabled"];
	        this.blocking = source["blocking"];
	        this.type = source["type"];
	        this.speed = source["speed"];
	        this.description = source["description"];
	        this.trunk = source["trunk"];
	        this.cage = source["cage"];
	    }
	}
	export class Summary {
	    family: string;
	    hostname: string;
	    model: string;
	    software: string;
	    serial: string;
	    uptime: string;
	    uptimeSec?: number;
	    cpu: number;
	    cpu1m?: number;
	    cpu5m?: number;
	    cpuInstant?: number;
	    memory: number;
	    vlans: number;
	    portsUp: number;
	    portsTotal: number;
	    neighbors: number;
	    stpRole: string;
	    bootRom?: string;
	    webScheme?: string;
	    ports: PortLED[];
	    // Go type: time
	    collectedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.family = source["family"];
	        this.hostname = source["hostname"];
	        this.model = source["model"];
	        this.software = source["software"];
	        this.serial = source["serial"];
	        this.uptime = source["uptime"];
	        this.uptimeSec = source["uptimeSec"];
	        this.cpu = source["cpu"];
	        this.cpu1m = source["cpu1m"];
	        this.cpu5m = source["cpu5m"];
	        this.cpuInstant = source["cpuInstant"];
	        this.memory = source["memory"];
	        this.vlans = source["vlans"];
	        this.portsUp = source["portsUp"];
	        this.portsTotal = source["portsTotal"];
	        this.neighbors = source["neighbors"];
	        this.stpRole = source["stpRole"];
	        this.bootRom = source["bootRom"];
	        this.webScheme = source["webScheme"];
	        this.ports = this.convertValues(source["ports"], PortLED);
	        this.collectedAt = this.convertValues(source["collectedAt"], null);
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
	export class Job {
	    id: string;
	    switchId: string;
	    switchName: string;
	    image: string;
	    from: string;
	    to: string;
	    reboot: boolean;
	    auto: boolean;
	    state: string;
	    step: string;
	    progress: number;
	    error: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    endedAt: any;
	    log: string[];
	    batch?: string;
	    order?: number;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.switchId = source["switchId"];
	        this.switchName = source["switchName"];
	        this.image = source["image"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.reboot = source["reboot"];
	        this.auto = source["auto"];
	        this.state = source["state"];
	        this.step = source["step"];
	        this.progress = source["progress"];
	        this.error = source["error"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.endedAt = this.convertValues(source["endedAt"], null);
	        this.log = source["log"];
	        this.batch = source["batch"];
	        this.order = source["order"];
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
	export class FirmwareInfo {
	    running: string;
	    banks: aruba.Banks;
	    available: string;
	    image: string;
	    staged: boolean;
	    supported: boolean;
	    job?: Job;
	
	    static createFrom(source: any = {}) {
	        return new FirmwareInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.banks = this.convertValues(source["banks"], aruba.Banks);
	        this.available = source["available"];
	        this.image = source["image"];
	        this.staged = source["staged"];
	        this.supported = source["supported"];
	        this.job = this.convertValues(source["job"], Job);
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
	export class Status {
	    online: boolean;
	    checked: boolean;
	    latencyMs: number;
	    pingError: string;
	    // Go type: time
	    lastCheck: any;
	    // Go type: time
	    since: any;
	    collecting: boolean;
	    // Go type: time
	    lastCollect: any;
	    collectError: string;
	    authFailed: boolean;
	    hostKeyChanged: boolean;
	    topologyDelta: number;
	    usersAdded?: string[];
	    usersRemoved?: string[];
	    // Go type: time
	    usersChanged?: any;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.online = source["online"];
	        this.checked = source["checked"];
	        this.latencyMs = source["latencyMs"];
	        this.pingError = source["pingError"];
	        this.lastCheck = this.convertValues(source["lastCheck"], null);
	        this.since = this.convertValues(source["since"], null);
	        this.collecting = source["collecting"];
	        this.lastCollect = this.convertValues(source["lastCollect"], null);
	        this.collectError = source["collectError"];
	        this.authFailed = source["authFailed"];
	        this.hostKeyChanged = source["hostKeyChanged"];
	        this.topologyDelta = source["topologyDelta"];
	        this.usersAdded = source["usersAdded"];
	        this.usersRemoved = source["usersRemoved"];
	        this.usersChanged = this.convertValues(source["usersChanged"], null);
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
	export class SwitchView {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    family: string;
	    credentialId: string;
	    site: string;
	    tags?: string[];
	    notes: string;
	    hostKey: string;
	    certFp?: string;
	    webEncrypted?: boolean;
	    disabled: boolean;
	    // Go type: time
	    createdAt: any;
	    cloud?: store.CloudRef;
	    web?: boolean;
	    lldpNames?: Record<string, string>;
	    firmwareHold?: boolean;
	    unsaved?: boolean;
	    status: Status;
	    firmware: FirmwareInfo;
	    summary?: Summary;
	    issues: Issue[];
	    worst: string;
	
	    static createFrom(source: any = {}) {
	        return new SwitchView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.family = source["family"];
	        this.credentialId = source["credentialId"];
	        this.site = source["site"];
	        this.tags = source["tags"];
	        this.notes = source["notes"];
	        this.hostKey = source["hostKey"];
	        this.certFp = source["certFp"];
	        this.webEncrypted = source["webEncrypted"];
	        this.disabled = source["disabled"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.cloud = this.convertValues(source["cloud"], store.CloudRef);
	        this.web = source["web"];
	        this.lldpNames = source["lldpNames"];
	        this.firmwareHold = source["firmwareHold"];
	        this.unsaved = source["unsaved"];
	        this.status = this.convertValues(source["status"], Status);
	        this.firmware = this.convertValues(source["firmware"], FirmwareInfo);
	        this.summary = this.convertValues(source["summary"], Summary);
	        this.issues = this.convertValues(source["issues"], Issue);
	        this.worst = source["worst"];
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
	export class Issue {
	    switchId: string;
	    switchIds?: string[];
	    switchName: string;
	    severity: string;
	    category: string;
	    title: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.switchId = source["switchId"];
	        this.switchIds = source["switchIds"];
	        this.switchName = source["switchName"];
	        this.severity = source["severity"];
	        this.category = source["category"];
	        this.title = source["title"];
	        this.detail = source["detail"];
	    }
	}
	export class Dashboard {
	    total: number;
	    online: number;
	    offline: number;
	    unknown: number;
	    disabled: number;
	    critical: number;
	    warning: number;
	    info: number;
	    collecting: number;
	    issues: Issue[];
	    switches: SwitchView[];
	    families: Record<string, number>;
	    dataDir: string;
	    loading?: number;
	
	    static createFrom(source: any = {}) {
	        return new Dashboard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.online = source["online"];
	        this.offline = source["offline"];
	        this.unknown = source["unknown"];
	        this.disabled = source["disabled"];
	        this.critical = source["critical"];
	        this.warning = source["warning"];
	        this.info = source["info"];
	        this.collecting = source["collecting"];
	        this.issues = this.convertValues(source["issues"], Issue);
	        this.switches = this.convertValues(source["switches"], SwitchView);
	        this.families = source["families"];
	        this.dataDir = source["dataDir"];
	        this.loading = source["loading"];
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
	export class Detail {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    family: string;
	    credentialId: string;
	    site: string;
	    tags?: string[];
	    notes: string;
	    hostKey: string;
	    certFp?: string;
	    webEncrypted?: boolean;
	    disabled: boolean;
	    // Go type: time
	    createdAt: any;
	    cloud?: store.CloudRef;
	    web?: boolean;
	    lldpNames?: Record<string, string>;
	    firmwareHold?: boolean;
	    unsaved?: boolean;
	    status: Status;
	    firmware: FirmwareInfo;
	    summary?: Summary;
	    issues: Issue[];
	    worst: string;
	    snapshot?: aruba.Snapshot;
	    backups: store.Backup[];
	
	    static createFrom(source: any = {}) {
	        return new Detail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.family = source["family"];
	        this.credentialId = source["credentialId"];
	        this.site = source["site"];
	        this.tags = source["tags"];
	        this.notes = source["notes"];
	        this.hostKey = source["hostKey"];
	        this.certFp = source["certFp"];
	        this.webEncrypted = source["webEncrypted"];
	        this.disabled = source["disabled"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.cloud = this.convertValues(source["cloud"], store.CloudRef);
	        this.web = source["web"];
	        this.lldpNames = source["lldpNames"];
	        this.firmwareHold = source["firmwareHold"];
	        this.unsaved = source["unsaved"];
	        this.status = this.convertValues(source["status"], Status);
	        this.firmware = this.convertValues(source["firmware"], FirmwareInfo);
	        this.summary = this.convertValues(source["summary"], Summary);
	        this.issues = this.convertValues(source["issues"], Issue);
	        this.worst = source["worst"];
	        this.snapshot = this.convertValues(source["snapshot"], aruba.Snapshot);
	        this.backups = this.convertValues(source["backups"], store.Backup);
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
	export class Edge {
	    id: string;
	    from: string;
	    to: string;
	    fromPort: string;
	    toPort: string;
	    protocols: string[];
	    blocking: boolean;
	    up: boolean;
	    fromVlans?: number[];
	    toVlans?: number[];
	
	    static createFrom(source: any = {}) {
	        return new Edge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.fromPort = source["fromPort"];
	        this.toPort = source["toPort"];
	        this.protocols = source["protocols"];
	        this.blocking = source["blocking"];
	        this.up = source["up"];
	        this.fromVlans = source["fromVlans"];
	        this.toVlans = source["toVlans"];
	    }
	}
	
	
	
	export class LinkTraffic {
	    rx: number;
	    tx: number;
	    util: number;
	    speed: number;
	    live: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LinkTraffic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rx = source["rx"];
	        this.tx = source["tx"];
	        this.util = source["util"];
	        this.speed = source["speed"];
	        this.live = source["live"];
	    }
	}
	export class MACHit {
	    switchId: string;
	    switchName: string;
	    port: string;
	    portName?: string;
	    vlan: string;
	    mac: string;
	    vendor?: string;
	    uplink?: string;
	    edge: boolean;
	    others: number;
	
	    static createFrom(source: any = {}) {
	        return new MACHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.switchId = source["switchId"];
	        this.switchName = source["switchName"];
	        this.port = source["port"];
	        this.portName = source["portName"];
	        this.vlan = source["vlan"];
	        this.mac = source["mac"];
	        this.vendor = source["vendor"];
	        this.uplink = source["uplink"];
	        this.edge = source["edge"];
	        this.others = source["others"];
	    }
	}
	export class NodeTraffic {
	    cpu: number;
	    cpu1m?: number;
	    cpu5m?: number;
	    cpuInstant?: number;
	    memory: number;
	    rx: number;
	    tx: number;
	    hasTraffic: boolean;
	    live: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NodeTraffic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpu = source["cpu"];
	        this.cpu1m = source["cpu1m"];
	        this.cpu5m = source["cpu5m"];
	        this.cpuInstant = source["cpuInstant"];
	        this.memory = source["memory"];
	        this.rx = source["rx"];
	        this.tx = source["tx"];
	        this.hasTraffic = source["hasTraffic"];
	        this.live = source["live"];
	    }
	}
	export class MapTelemetry {
	    edges: Record<string, LinkTraffic>;
	    nodes: Record<string, NodeTraffic>;
	
	    static createFrom(source: any = {}) {
	        return new MapTelemetry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.edges = this.convertValues(source["edges"], LinkTraffic, true);
	        this.nodes = this.convertValues(source["nodes"], NodeTraffic, true);
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
	export class MonPort {
	    port: string;
	    name?: string;
	    up: boolean;
	    rx: number;
	    tx: number;
	    util: number;
	    speed?: number;
	
	    static createFrom(source: any = {}) {
	        return new MonPort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.name = source["name"];
	        this.up = source["up"];
	        this.rx = source["rx"];
	        this.tx = source["tx"];
	        this.util = source["util"];
	        this.speed = source["speed"];
	    }
	}
	export class MonSample {
	    t: number;
	    cpu: number;
	    mem: number;
	    rx: number;
	    tx: number;
	
	    static createFrom(source: any = {}) {
	        return new MonSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.t = source["t"];
	        this.cpu = source["cpu"];
	        this.mem = source["mem"];
	        this.rx = source["rx"];
	        this.tx = source["tx"];
	    }
	}
	export class MonitorRow {
	    id: string;
	    name: string;
	    host: string;
	    model: string;
	    family: string;
	    online: boolean;
	    disabled: boolean;
	    supported: boolean;
	    cpu: number;
	    cpu1m?: number;
	    cpu5m?: number;
	    cpuInstant?: number;
	    memory: number;
	    rx: number;
	    tx: number;
	    hasTraffic: boolean;
	    portsUp: number;
	    portsTotal: number;
	    top: MonPort[];
	    history: MonSample[];
	    // Go type: time
	    at: any;
	    live: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new MonitorRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.model = source["model"];
	        this.family = source["family"];
	        this.online = source["online"];
	        this.disabled = source["disabled"];
	        this.supported = source["supported"];
	        this.cpu = source["cpu"];
	        this.cpu1m = source["cpu1m"];
	        this.cpu5m = source["cpu5m"];
	        this.cpuInstant = source["cpuInstant"];
	        this.memory = source["memory"];
	        this.rx = source["rx"];
	        this.tx = source["tx"];
	        this.hasTraffic = source["hasTraffic"];
	        this.portsUp = source["portsUp"];
	        this.portsTotal = source["portsTotal"];
	        this.top = this.convertValues(source["top"], MonPort);
	        this.history = this.convertValues(source["history"], MonSample);
	        this.at = this.convertValues(source["at"], null);
	        this.live = source["live"];
	        this.error = source["error"];
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
	export class Node {
	    id: string;
	    label: string;
	    kind: string;
	    managed: boolean;
	    host: string;
	    model: string;
	    mac?: string;
	    vendor?: string;
	    vlans?: number[];
	    online: boolean;
	    disabled: boolean;
	    severity: string;
	    issues: number;
	    stpRoot: boolean;
	    x?: number;
	    y?: number;
	
	    static createFrom(source: any = {}) {
	        return new Node(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.managed = source["managed"];
	        this.host = source["host"];
	        this.model = source["model"];
	        this.mac = source["mac"];
	        this.vendor = source["vendor"];
	        this.vlans = source["vlans"];
	        this.online = source["online"];
	        this.disabled = source["disabled"];
	        this.severity = source["severity"];
	        this.issues = source["issues"];
	        this.stpRoot = source["stpRoot"];
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	
	
	
	
	export class SwitchVLANs {
	    id: string;
	    name: string;
	    online: boolean;
	    vlans: aruba.VLAN[];
	    family: string;
	    editable: boolean;
	    readOnly?: string;
	
	    static createFrom(source: any = {}) {
	        return new SwitchVLANs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.online = source["online"];
	        this.vlans = this.convertValues(source["vlans"], aruba.VLAN);
	        this.family = source["family"];
	        this.editable = source["editable"];
	        this.readOnly = source["readOnly"];
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
	
	export class TopoVLAN {
	    id: number;
	    name: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new TopoVLAN(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.count = source["count"];
	    }
	}
	export class Topology {
	    nodes: Node[];
	    edges: Edge[];
	    vlans?: TopoVLAN[];
	
	    static createFrom(source: any = {}) {
	        return new Topology(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], Node);
	        this.edges = this.convertValues(source["edges"], Edge);
	        this.vlans = this.convertValues(source["vlans"], TopoVLAN);
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

export namespace firmware {
	
	export class Version {
	    branch: string;
	    nums: number[];
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new Version(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.nums = source["nums"];
	        this.text = source["text"];
	    }
	}
	export class Image {
	    file: string;
	    version: Version;
	    size: number;
	    sha256: string;
	    // Go type: time
	    addedAt: any;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new Image(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.version = this.convertValues(source["version"], Version);
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	        this.addedAt = this.convertValues(source["addedAt"], null);
	        this.source = source["source"];
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
	
	export class BulkResult {
	    added: number;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new BulkResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.errors = source["errors"];
	    }
	}
	export class DataFolder {
	    path: string;
	    defaultPath: string;
	    custom: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DataFolder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.defaultPath = source["defaultPath"];
	        this.custom = source["custom"];
	    }
	}
	export class LockInfo {
	    enabled: boolean;
	    locked: boolean;
	    starting: boolean;
	    lockAfter: number;
	
	    static createFrom(source: any = {}) {
	        return new LockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.locked = source["locked"];
	        this.starting = source["starting"];
	        this.lockAfter = source["lockAfter"];
	    }
	}
	export class Translation {
	    language: string;
	    code: string;
	    strings: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Translation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.code = source["code"];
	        this.strings = source["strings"];
	    }
	}
	export class UpdateInfo {
	    current: string;
	    latest: string;
	    url: string;
	    // Go type: time
	    published: any;
	    available: boolean;
	    // Go type: time
	    checkedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.url = source["url"];
	        this.published = this.convertValues(source["published"], null);
	        this.available = source["available"];
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
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

export namespace netbind {
	
	export class DiscoveredHost {
	    ip: string;
	    ports: number[];
	    familyGuess: string;
	    webOnly: boolean;
	    sshBanner?: string;
	    existingId?: string;
	    existingName?: string;
	
	    static createFrom(source: any = {}) {
	        return new DiscoveredHost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.ports = source["ports"];
	        this.familyGuess = source["familyGuess"];
	        this.webOnly = source["webOnly"];
	        this.sshBanner = source["sshBanner"];
	        this.existingId = source["existingId"];
	        this.existingName = source["existingName"];
	    }
	}
	export class NICInfo {
	    name: string;
	    ip: string;
	    cidr: string;
	    maskBits: number;
	    rangeHint: string;
	    mac: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new NICInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ip = source["ip"];
	        this.cidr = source["cidr"];
	        this.maskBits = source["maskBits"];
	        this.rangeHint = source["rangeHint"];
	        this.mac = source["mac"];
	        this.kind = source["kind"];
	    }
	}

}

export namespace sshopen {
	
	export class Client {
	    id: string;
	    name: string;
	    found: boolean;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new Client(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.found = source["found"];
	        this.command = source["command"];
	    }
	}

}

export namespace store {
	
	export class Backup {
	    name: string;
	    // Go type: time
	    at: any;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Backup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.at = this.convertValues(source["at"], null);
	        this.size = source["size"];
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
	export class BundleInfo {
	    // Go type: time
	    createdAt: any;
	    switches: number;
	    history: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BundleInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.switches = source["switches"];
	        this.history = source["history"];
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
	export class CloudRef {
	    siteId: string;
	    siteName: string;
	    deviceId: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.siteId = source["siteId"];
	        this.siteName = source["siteName"];
	        this.deviceId = source["deviceId"];
	    }
	}
	export class Credential {
	    id: string;
	    name: string;
	    username: string;
	    password?: string;
	    kind?: string;
	
	    static createFrom(source: any = {}) {
	        return new Credential(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.kind = source["kind"];
	    }
	}
	export class DirInfo {
	    path: string;
	    hasData: boolean;
	    switches: number;
	    writable: boolean;
	    same: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DirInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.hasData = source["hasData"];
	        this.switches = source["switches"];
	        this.writable = source["writable"];
	        this.same = source["same"];
	    }
	}
	export class ImportResult {
	    added: number;
	    skipped: string[];
	    credentials: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.skipped = source["skipped"];
	        this.credentials = source["credentials"];
	    }
	}
	export class Settings {
	    checkMethod: string;
	    pingInterval: number;
	    collectInterval: number;
	    parallel: number;
	    keepBackups: number;
	    cpuWarn: number;
	    memWarn: number;
	    ignoreSnmp: boolean;
	    hardwarePhotos: boolean;
	    firmwareAuto: string;
	    windowStart: string;
	    windowEnd: string;
	    lockAfter: number;
	    language: string;
	    sshClient: string;
	    sshCommand: string;
	    uiFont: string;
	    uiScale: number;
	    noUpdateCheck: boolean;
	    autoPortNames: boolean;
	    rebootPause: number;
	    bindIp?: string;
	    bindNic?: string;
	    skipStartupNic?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checkMethod = source["checkMethod"];
	        this.pingInterval = source["pingInterval"];
	        this.collectInterval = source["collectInterval"];
	        this.parallel = source["parallel"];
	        this.keepBackups = source["keepBackups"];
	        this.cpuWarn = source["cpuWarn"];
	        this.memWarn = source["memWarn"];
	        this.ignoreSnmp = source["ignoreSnmp"];
	        this.hardwarePhotos = source["hardwarePhotos"];
	        this.firmwareAuto = source["firmwareAuto"];
	        this.windowStart = source["windowStart"];
	        this.windowEnd = source["windowEnd"];
	        this.lockAfter = source["lockAfter"];
	        this.language = source["language"];
	        this.sshClient = source["sshClient"];
	        this.sshCommand = source["sshCommand"];
	        this.uiFont = source["uiFont"];
	        this.uiScale = source["uiScale"];
	        this.noUpdateCheck = source["noUpdateCheck"];
	        this.autoPortNames = source["autoPortNames"];
	        this.rebootPause = source["rebootPause"];
	        this.bindIp = source["bindIp"];
	        this.bindNic = source["bindNic"];
	        this.skipStartupNic = source["skipStartupNic"];
	    }
	}
	export class Switch {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    family: string;
	    credentialId: string;
	    site: string;
	    tags?: string[];
	    notes: string;
	    hostKey: string;
	    certFp?: string;
	    webEncrypted?: boolean;
	    disabled: boolean;
	    // Go type: time
	    createdAt: any;
	    cloud?: CloudRef;
	    web?: boolean;
	    lldpNames?: Record<string, string>;
	    firmwareHold?: boolean;
	    unsaved?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Switch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.family = source["family"];
	        this.credentialId = source["credentialId"];
	        this.site = source["site"];
	        this.tags = source["tags"];
	        this.notes = source["notes"];
	        this.hostKey = source["hostKey"];
	        this.certFp = source["certFp"];
	        this.webEncrypted = source["webEncrypted"];
	        this.disabled = source["disabled"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.cloud = this.convertValues(source["cloud"], CloudRef);
	        this.web = source["web"];
	        this.lldpNames = source["lldpNames"];
	        this.firmwareHold = source["firmwareHold"];
	        this.unsaved = source["unsaved"];
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

