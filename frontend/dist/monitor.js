"use strict";
/* Live Monitoring page for MySwitch: CPU, Memory, and interface traffic over SSH. */

const MON = {
  active: false,
  loaded: false,
  prefsLoaded: false,
  paused: false,
  interval: 10,
  filter: "all",
  sort: { by: "traffic", dir: -1 },
  q: "",
  rows: [],
  open: new Set(),
  keepTimer: null,
  pollTimer: null,
  fetching: false,
};

function monInitPrefs() {
  if (MON.prefsLoaded) return;
  MON.prefsLoaded = true;
  const iv = Number(swPref("ms-mon-iv", 10));
  if ([5, 10, 15, 30, 60].includes(iv)) MON.interval = iv;
  const f = swPref("ms-mon-filter", "all");
  if (["all", "online", "hot"].includes(f)) MON.filter = f;
  const s = swPref("ms-mon-sort", null);
  if (s && s.by) MON.sort = { by: s.by, dir: s.dir === 1 ? 1 : -1 };
}

function stopMonitor(keepLive) {
  MON.active = false;
  if (MON.keepTimer) { clearInterval(MON.keepTimer); MON.keepTimer = null; }
  if (MON.pollTimer) { clearInterval(MON.pollTimer); MON.pollTimer = null; }
  if (!keepLive && typeof api !== "undefined" && api) call("MonitorLive", 0).catch(() => {});
}

async function monHeartbeat() {
  if (!MON.active || MON.paused) return;
  try { await call("MonitorLive", MON.interval); } catch {}
}

async function monFetch(force) {
  if (!MON.active || (MON.paused && !force) || MON.fetching) return;
  MON.fetching = true;
  try {
    MON.rows = (await call("MonitorRows")) || [];
    MON.loaded = true;
    if (MON.active) drawMonitorData();
  } catch (e) {
    if (MON.active && !MON.loaded) toast("Monitor error: " + e.message, true);
  } finally {
    MON.fetching = false;
  }
}

function renderMonitor() {
  monInitPrefs();
  MON.active = true;
  if (![5, 10, 15, 30, 60].includes(MON.interval)) MON.interval = 10;
  if (MON.keepTimer) { clearInterval(MON.keepTimer); MON.keepTimer = null; }
  if (MON.pollTimer) { clearInterval(MON.pollTimer); MON.pollTimer = null; }

  const main = $("#main");
  main.innerHTML = `<div class="page mon-page">
    <div class="head">
      <h1>Monitor</h1>
      ${siteSub()}
      <span id="mon-live-pill"></span>
      <span class="grow"></span>
      <label class="mon-iv-wrap" title="How often switches are polled">
        <span class="faint">Interval</span>
        <select id="mon-iv" onchange="monSetInterval(this.value)">
          ${[5, 10, 15, 30, 60].map((s) => `<option value="${s}"${MON.interval === s ? " selected" : ""}>${s}s</option>`).join("")}
        </select>
      </label>
      <button class="btn" id="mon-pause-btn" onclick="monTogglePause()"></button>
      <button class="btn primary" id="mon-refresh-btn" onclick="monRefreshNow(this)">${ICON.refresh}Refresh</button>
    </div>
    <div class="kpi-grid" id="mon-kpis"></div>
    <div class="panel mon-panel">
      <div class="mon-toolbar">
        <div class="chips" id="mon-chips"></div>
        <div class="grow"></div>
        <div class="mon-sort-pills" id="mon-sorts"></div>
        <input type="search" class="mon-search" id="mon-q" value="${esc(MON.q)}" placeholder="Filter switch, IP, model…" oninput="MON.q=this.value;drawMonitorData()">
      </div>
      <div class="mon-scroll" id="mon-scroll">
        <div class="empty">Connecting to switches…</div>
      </div>
    </div>
  </div>`;

  monUpdateHeaderControls();
  if (!MON.paused) {
    monHeartbeat();
    MON.keepTimer = setInterval(monHeartbeat, 10000);
  }
  monFetch(true);
  MON.pollTimer = setInterval(() => monFetch(false), 2000);
}

function monUpdateHeaderControls() {
  const pill = $("#mon-live-pill");
  if (pill) {
    pill.innerHTML = MON.paused
      ? `<span class="mon-pill paused"><span class="mon-dot"></span>Paused</span>`
      : `<span class="mon-pill live"><span class="mon-dot"></span>Live · ${MON.interval}s</span>`;
  }
  const btn = $("#mon-pause-btn");
  if (btn) btn.innerHTML = MON.paused ? `${ICON.play}Resume` : `${ICON.pause}Pause`;
}

function monSetInterval(val) {
  MON.interval = Number(val) || 10;
  swSave("ms-mon-iv", MON.interval);
  monUpdateHeaderControls();
  if (!MON.paused) {
    monHeartbeat();
    monFetch(true);
  }
}

function monTogglePause() {
  MON.paused = !MON.paused;
  if (MON.paused) {
    if (MON.keepTimer) { clearInterval(MON.keepTimer); MON.keepTimer = null; }
    call("MonitorLive", 0).catch(() => {});
  } else {
    monHeartbeat();
    MON.keepTimer = setInterval(monHeartbeat, 10000);
    monFetch(true);
  }
  monUpdateHeaderControls();
  drawMonitorData();
}

async function monRefreshNow(btn) {
  btn?.classList.add("spin");
  try {
    if (!MON.paused) await monHeartbeat();
    await monFetch(true);
  } finally {
    setTimeout(() => btn?.classList.remove("spin"), 350);
  }
}

function monSetFilter(f) {
  MON.filter = f;
  swSave("ms-mon-filter", f);
  drawMonitorData();
}

function monSetSort(by) {
  if (MON.sort.by === by) MON.sort.dir = -MON.sort.dir;
  else MON.sort = { by, dir: by === "name" || by === "host" ? 1 : -1 };
  swSave("ms-mon-sort", MON.sort);
  drawMonitorData();
}

function monToggleRow(id) {
  if (MON.open.has(id)) MON.open.delete(id);
  else MON.open.add(id);
  drawMonitorData();
}

function fmtBps(bps) {
  const v = Number(bps) || 0;
  if (v <= 0) return "0 bps";
  if (v < 1000) return `${Math.round(v)} bps`;
  if (v < 1e6) return `${(v / 1e3).toFixed(v >= 1e5 ? 0 : 1)} Kbps`;
  if (v < 1e9) return `${(v / 1e6).toFixed(v >= 1e8 ? 0 : 1)} Mbps`;
  return `${(v / 1e9).toFixed(2)} Gbps`;
}

function fmtSpeed(mbps) {
  const m = Number(mbps) || 0;
  if (!m) return "—";
  return m >= 1000 ? `${+(m / 1000).toFixed(1)}G` : `${m}M`;
}

const cpuLevel = (c) => (c < 0 ? "faint" : c >= 85 ? "crit" : c >= 65 ? "warn" : "ok");
const memLevel = (m) => (m < 0 ? "faint" : m >= 90 ? "crit" : m >= 75 ? "warn" : "info");
const utilLevel = (u) => (u < 0 ? "faint" : u >= 80 ? "crit" : u >= 50 ? "warn" : "ok");
const isHot = (r) => (r.cpu >= 70) || (r.memory >= 80);

function monSpark(values, fixedMax, stroke, areaFill) {
  const pts = (values || []).filter((v) => Number.isFinite(v) && v >= 0);
  if (pts.length === 0) {
    return `<svg class="mon-spark empty-spark" viewBox="0 0 112 26" preserveAspectRatio="none"><line x1="0" y1="22" x2="112" y2="22" stroke="rgba(148,163,184,.18)" stroke-width="1" stroke-dasharray="2 3"/></svg>`;
  }
  const series = pts.length === 1 ? [pts[0], pts[0]] : pts;
  const max = Math.max(fixedMax || 0, ...series, 1);
  const w = 112, h = 26, pad = 2.5;
  const coords = series.map((v, i) => {
    const x = (i / (series.length - 1)) * w;
    const y = h - pad - ((Math.min(v, max) / max) * (h - pad * 2));
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  const line = coords.join(" ");
  const area = areaFill ? `<polygon points="0,${h} ${line} ${w},${h}" fill="${areaFill}"/>` : "";
  const last = coords[coords.length - 1].split(",");
  return `<svg class="mon-spark" viewBox="0 0 ${w} ${h}" preserveAspectRatio="none">${area}<polyline points="${line}" fill="none" stroke="${stroke}" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/><circle cx="${last[0]}" cy="${last[1]}" r="2" fill="${stroke}"/></svg>`;
}

function monTrafficSpark(hist) {
  const samples = (hist || []).filter((s) => Number.isFinite(s.rx) && Number.isFinite(s.tx));
  if (samples.length === 0) {
    return `<svg class="mon-spark empty-spark" viewBox="0 0 112 26" preserveAspectRatio="none"><line x1="0" y1="22" x2="112" y2="22" stroke="rgba(148,163,184,.18)" stroke-width="1" stroke-dasharray="2 3"/></svg>`;
  }
  const s = samples.length === 1 ? [samples[0], samples[0]] : samples;
  const max = Math.max(1000, ...s.map((p) => Math.max(p.rx || 0, p.tx || 0)));
  const w = 112, h = 26, pad = 2.5;
  const toLine = (key) => s.map((p, i) => {
    const x = (i / (s.length - 1)) * w;
    const y = h - pad - ((Math.max(0, p[key] || 0) / max) * (h - pad * 2));
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(" ");
  const rxLine = toLine("rx"), txLine = toLine("tx");
  return `<svg class="mon-spark" viewBox="0 0 ${w} ${h}" preserveAspectRatio="none">
    <polygon points="0,${h} ${rxLine} ${w},${h}" fill="rgba(56,189,248,.12)"/>
    <polyline points="${txLine}" fill="none" stroke="#10b981" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" opacity=".9"/>
    <polyline points="${rxLine}" fill="none" stroke="#38bdf8" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>`;
}

function drawMonitorData() {
  if (!MON.active) return;
  const all = siteOnly(MON.rows);

  // Compute KPI figures across visible site switches
  const enabled = all.filter((r) => !r.disabled);
  const supported = enabled.filter((r) => r.supported);
  const onlineSupported = supported.filter((r) => r.online);
  const liveRows = enabled.filter((r) => r.live);

  const cpuRows = enabled.filter((r) => r.cpu >= 0);
  const avgCpu = cpuRows.length ? Math.round(cpuRows.reduce((s, r) => s + r.cpu, 0) / cpuRows.length) : -1;
  const peakCpuRow = cpuRows.reduce((best, r) => (!best || r.cpu > best.cpu ? r : best), null);

  const memRows = enabled.filter((r) => r.memory >= 0);
  const avgMem = memRows.length ? Math.round(memRows.reduce((s, r) => s + r.memory, 0) / memRows.length) : -1;
  const peakMemRow = memRows.reduce((best, r) => (!best || r.memory > best.memory ? r : best), null);

  const trafficRows = enabled.filter((r) => r.hasTraffic);
  const totalRx = trafficRows.reduce((s, r) => s + (r.rx || 0), 0);
  const totalTx = trafficRows.reduce((s, r) => s + (r.tx || 0), 0);
  const peakTrafficRow = trafficRows.reduce((best, r) => (!best || (r.rx + r.tx) > (best.rx + best.tx) ? r : best), null);

  const cpuCardCls = peakCpuRow?.cpu >= 85 ? "crit-card" : peakCpuRow?.cpu >= 65 ? "warn-card" : "";
  const cpuIcoCls = peakCpuRow?.cpu >= 85 ? "bad" : peakCpuRow?.cpu >= 65 ? "warnc" : "ok";
  const memCardCls = peakMemRow?.memory >= 90 ? "crit-card" : peakMemRow?.memory >= 75 ? "warn-card" : "";
  const memIcoCls = peakMemRow?.memory >= 90 ? "bad" : peakMemRow?.memory >= 75 ? "warnc" : "info";

  const kpis = $("#mon-kpis");
  if (kpis) {
    kpis.innerHTML = `
      <div class="kpi-card ok-card" onclick="monSetFilter('online')">
        <div class="kpi-top"><span class="kpi-lbl">Live Telemetry</span><span class="kpi-ico ok">${ICON.activity}</span></div>
        <div class="kpi-val ok">${liveRows.length}<small> / ${onlineSupported.length}</small></div>
        <div class="kpi-sub">${supported.length} SSH-capable · every ${MON.interval}s</div>
      </div>
      <div class="kpi-card ${cpuCardCls}" onclick="monSetSort('cpu')">
        <div class="kpi-top"><span class="kpi-lbl">CPU Load (Avg)</span><span class="kpi-ico ${cpuIcoCls}">${ICON.chip}</span></div>
        <div class="kpi-val ${peakCpuRow ? cpuLevel(avgCpu) : "muted"}">${avgCpu >= 0 ? avgCpu + "%" : "—"}<small>${peakCpuRow ? ` · peak ${peakCpuRow.cpu}%` : ""}</small></div>
        <div class="kpi-sub">${peakCpuRow ? `Peak on ${esc(peakCpuRow.name)}` : "Waiting for CPU reading"}</div>
      </div>
      <div class="kpi-card ${memCardCls}" onclick="monSetSort('mem')">
        <div class="kpi-top"><span class="kpi-lbl">Memory Usage (Avg)</span><span class="kpi-ico ${memIcoCls}">${ICON.dash}</span></div>
        <div class="kpi-val ${peakMemRow ? "" : "muted"}">${avgMem >= 0 ? avgMem + "%" : "—"}<small>${peakMemRow ? ` · peak ${peakMemRow.memory}%` : ""}</small></div>
        <div class="kpi-sub">${peakMemRow ? `Peak on ${esc(peakMemRow.name)}` : "Waiting for memory reading"}</div>
      </div>
      <div class="kpi-card info-card" onclick="monSetSort('traffic')">
        <div class="kpi-top"><span class="kpi-lbl">Aggregate Traffic</span><span class="kpi-ico info">${ICON.up}</span></div>
        <div class="kpi-val info">${trafficRows.length ? fmtBps(totalRx + totalTx) : "—"}</div>
        <div class="kpi-sub">${trafficRows.length ? `↓ ${fmtBps(totalRx)} · ↑ ${fmtBps(totalTx)}${peakTrafficRow ? ` · top: ${esc(peakTrafficRow.name)}` : ""}` : "Needs 2 samples on AOS-CX"}</div>
      </div>`;
  }

  // Toolbar chips & sort indicators
  const onlineCnt = all.filter((r) => !r.disabled && r.online).length;
  const hotCnt = all.filter((r) => !r.disabled && isHot(r)).length;
  const chips = $("#mon-chips");
  if (chips) {
    const cbtn = (k, label, n) => `<button class="chip${MON.filter === k ? " on" : ""}" onclick="monSetFilter('${k}')">${label} ${n}</button>`;
    chips.innerHTML = cbtn("all", "All", all.length) + cbtn("online", "Online", onlineCnt) + cbtn("hot", "High Load", hotCnt);
  }

  // Filter rows
  const ql = MON.q.trim().toLowerCase();
  let rows = all.filter((r) => {
    if (MON.filter === "online" && (r.disabled || !r.online)) return false;
    if (MON.filter === "hot" && (r.disabled || !isHot(r))) return false;
    if (ql && ![r.name, r.host, r.model, r.family].some((v) => (v || "").toLowerCase().includes(ql))) return false;
    return true;
  });

  // Sort rows: online & live first unless sorting explicitly by name/host, with chosen metric
  const { by, dir } = MON.sort;
  rows.sort((a, b) => {
    if (by !== "name" && by !== "host") {
      const ra = a.disabled ? 2 : a.online ? 0 : 1;
      const rb = b.disabled ? 2 : b.online ? 0 : 1;
      if (ra !== rb) return ra - rb;
    }
    let cmp = 0;
    if (by === "name") cmp = (a.name || "").localeCompare(b.name || "", undefined, { numeric: true });
    else if (by === "host") cmp = ipKey(a.host) < ipKey(b.host) ? -1 : ipKey(a.host) > ipKey(b.host) ? 1 : 0;
    else if (by === "cpu") cmp = (a.cpu ?? -1) - (b.cpu ?? -1);
    else if (by === "mem") cmp = (a.memory ?? -1) - (b.memory ?? -1);
    else if (by === "ports") cmp = (a.portsUp || 0) - (b.portsUp || 0);
    else cmp = ((a.hasTraffic ? a.rx + a.tx : -1)) - ((b.hasTraffic ? b.rx + b.tx : -1));
    if (cmp !== 0) return cmp * dir;
    return (a.name || "").localeCompare(b.name || "", undefined, { numeric: true });
  });

  const scroll = $("#mon-scroll");
  if (!scroll) return;
  if (!MON.loaded) return;

  if (!all.length) {
    scroll.innerHTML = `<div class="empty"><h3>No switches yet</h3><p>Add your switches from the Switches page to monitor live CPU load and interface traffic.</p><button class="btn primary" data-go="#/switches">Go to Switches</button></div>`;
    return;
  }
  if (!rows.length) {
    scroll.innerHTML = `<div class="empty"><h3>No switches match</h3><p>Try clearing the filter or search query.</p></div>`;
    return;
  }

  const th = (key, label, cls = "") => {
    const active = MON.sort.by === key;
    const arrow = active ? (MON.sort.dir > 0 ? " ↑" : " ↓") : "";
    return `<th class="mon-th${active ? " sorted" : ""} ${cls}" onclick="monSetSort('${key}')">${label}${arrow}</th>`;
  };

  const prevTop = scroll.scrollTop;
  scroll.innerHTML = `<table class="mon-table">
    <thead><tr>
      ${th("name", "Switch")}
      ${th("host", "Address")}
      ${th("cpu", "CPU Load")}
      ${th("mem", "Memory")}
      ${th("traffic", "Traffic (Rx / Tx)")}
      ${th("ports", "Ports", "num")}
      <th style="width:94px"></th>
    </tr></thead>
    <tbody>${rows.map(renderMonitorRow).join("")}</tbody>
  </table>`;
  scroll.scrollTop = prevTop;
}

function renderMonitorRow(r) {
  const dotCls = r.disabled ? "" : r.online ? " up" : " down";
  const cLvl = cpuLevel(r.cpu);
  const mLvl = memLevel(r.memory);
  const cpuSparkColor = cLvl === "crit" ? "#f43f5e" : cLvl === "warn" ? "#f59e0b" : "#38bdf8";
  const cpuSparkFill = cLvl === "crit" ? "rgba(244,63,94,.16)" : cLvl === "warn" ? "rgba(245,158,11,.15)" : "rgba(56,189,248,.13)";
  const cpuVals = (r.history || []).map((s) => s.cpu);
  const isOpen = MON.open.has(r.id);

  let statusMeta = r.disabled ? `<span class="tag">Disabled</span>`
    : !r.online ? `<span class="sev critical">Offline</span>`
    : !r.supported ? `<span class="tag" title="Live CLI telemetry only supports SSH switches">Unsupported</span>`
    : r.error ? `<span class="sev warning" title="${esc(r.error)}">Poll warning</span>`
    : r.live ? `<span class="mon-ago ok">${ago(r.at)}</span>`
    : `<span class="mon-ago faint">${ago(r.at)}</span>`;

  let trafficCell = "";
  if (r.disabled || !r.online) {
    trafficCell = `<span class="faint">—</span>`;
  } else if (!r.supported) {
    trafficCell = `<span class="faint">No SSH counters</span>`;
  } else if (!r.hasTraffic) {
    trafficCell = `<div class="mon-tf-wait">${MON.paused ? `<span class="faint">Paused</span>` : `<span class="mon-measuring">Measuring…</span><small class="faint">next poll ${MON.interval}s</small>`}</div>`;
  } else {
    trafficCell = `<div class="mon-tf-cell">
      <div class="mon-tf-vals mono">
        <div class="mon-rx"><span class="mon-dir">↓</span> ${fmtBps(r.rx)}</div>
        <div class="mon-tx"><span class="mon-dir">↑</span> ${fmtBps(r.tx)}</div>
      </div>
      ${monTrafficSpark(r.history)}
    </div>`;
  }

  const topCount = (r.top || []).length;
  const subRow = isOpen ? `<tr class="mon-subrow"><td colspan="7">${renderMonitorPorts(r)}</td></tr>` : "";

  const cpuTipText = typeof cpuTip === "function" ? cpuTip(r) : "";
  const cpuKind = r.cpu1m != null && r.cpu1m >= 0 ? "1m" : r.cpu5m != null && r.cpu5m >= 0 ? "5m" : "";
  return `<tr class="mon-row${isOpen ? " expanded" : ""}${r.online && !r.disabled ? "" : " is-down"}">
    <td>
      <div class="mon-sw-cell">
        <span class="dot${dotCls}"></span>
        <div class="mon-sw-meta">
          <div class="mon-sw-top">
            <a class="mon-sw-name" href="#/switch/${encodeURIComponent(r.id)}">${esc(r.name)}</a>
            ${statusMeta}
          </div>
          <div class="mon-sw-model faint">${esc(r.model || r.family || "Aruba switch")}${r.error && r.online ? ` · <span class="warnc" title="${esc(r.error)}">${esc(r.error.slice(0, 46))}${r.error.length > 46 ? "…" : ""}</span>` : ""}</div>
        </div>
      </div>
    </td>
    <td><span class="ip-pill mono">${esc(r.host)}</span></td>
    <td>
      <div class="mon-metric-cell" title="${esc(cpuTipText)}">
        <div class="mon-bar-col">
          <div class="mon-val-row">
            <span class="mon-pct mono ${cLvl}">${r.cpu >= 0 ? r.cpu + "%" : "—"}</span>
            ${cpuKind ? `<span class="faint" style="font-size:10.5px">${cpuKind}</span>` : ""}
          </div>
          <div class="mon-meter"><i class="${cLvl}" style="width:${Math.max(0, Math.min(100, r.cpu))}%"></i></div>
        </div>
        ${monSpark(cpuVals, 100, cpuSparkColor, cpuSparkFill)}
      </div>
    </td>
    <td>
      <div class="mon-bar-col">
        <div class="mon-val-row">
          <span class="mon-pct mono ${mLvl}">${r.memory >= 0 ? r.memory + "%" : "—"}</span>
        </div>
        <div class="mon-meter"><i class="${mLvl}" style="width:${Math.max(0, Math.min(100, r.memory))}%"></i></div>
      </div>
    </td>
    <td>${trafficCell}</td>
    <td class="num mono">${r.portsTotal ? `<span class="ok">${r.portsUp}</span><span class="faint">/${r.portsTotal}</span>` : "—"}</td>
    <td class="num">
      ${topCount ? `<button class="btn small mon-exp-btn${isOpen ? " on" : ""}" onclick="monToggleRow('${esc(r.id)}')" title="Show busiest ports">${isOpen ? "Hide ports" : `Top ${topCount}`}</button>`
        : `<button class="btn small" data-go="#/switch/${encodeURIComponent(r.id)}">Open</button>`}
    </td>
  </tr>${subRow}`;
}

function renderMonitorPorts(r) {
  const ports = r.top || [];
  if (!ports.length) return `<div class="mon-ports-empty faint">No active port traffic recorded yet.</div>`;
  return `<div class="mon-ports-drawer">
    <div class="mon-ports-hdr">
      <span>Top Active Ports on <b>${esc(r.name)}</b></span>
      <a href="#/switch/${encodeURIComponent(r.id)}/ports">View all ports →</a>
    </div>
    <div class="mon-ports-grid">
      ${ports.map((p) => {
        const u = p.util >= 0 ? p.util : -1;
        const uCls = utilLevel(u);
        return `<div class="mon-port-card">
          <div class="mon-port-top">
            <span class="mono mon-port-id">${esc(p.port)}</span>
            ${p.name ? `<span class="mon-port-desc" title="${esc(p.name)}">${esc(p.name)}</span>` : ""}
            <span class="grow"></span>
            <span class="tag mono">${fmtSpeed(p.speed)}</span>
          </div>
          <div class="mon-port-rates mono">
            <span class="mon-rx">↓ ${fmtBps(p.rx)}</span>
            <span class="mon-tx">↑ ${fmtBps(p.tx)}</span>
            <span class="grow"></span>
            <span class="${uCls}">${u >= 0 ? `${u.toFixed(1)}%` : "—"}</span>
          </div>
          ${u >= 0 ? `<div class="mon-meter thin"><i class="${uCls}" style="width:${Math.max(2, Math.min(100, u))}%"></i></div>` : ""}
        </div>`;
      }).join("")}
    </div>
  </div>`;
}
