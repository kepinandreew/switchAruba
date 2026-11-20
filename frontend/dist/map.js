"use strict";
/* Network map: backbone-first hierarchical layout, attached leaf clusters,
   collision-free traffic pills, pan/zoom, and smooth directional flow animation. */

const SDMap = (() => {
  let cv, ctx, opts, topo = { nodes: [], edges: [] };
  let nodes = new Map(); // id -> node with x,y,vx,vy
  let view = { k: 1, tx: 0, ty: 0 };
  let alpha = 0, raf = 0, running = false, selected = null, hover = null;
  let drag = null, saveTimer = 0, ro = null, firstFit = true, pendingFit = false;
  let liveTraffic = true, traffic = { edges: {}, nodes: {} };
  let keepTimer = 0, pollTimer = 0, lastAnimTs = 0, hasFlow = false;
  let highlightVlan = 0;

  const DPR = () => window.devicePixelRatio || 1;
  const C = {
    line: "#5d7088",
    text: "#e4e8ef",
    muted: "#94a3b8",
    panel: "#131c2e",
    panel2: "#1e293b",
    border: "#2c415e",
    up: "#10b981",
    crit: "#f43f5e",
    warn: "#f59e0b",
    info: "#38bdf8",
    accent: "#38bdf8",
    vlan: "#22d3ee",
    tx: "#38bdf8",
    rx: "#34d399",
  };
  const SW = { w: 176, h: 46 };
  const LEAF_R = 12;

  function looksLikeMAC(s) {
    if (!s) return false;
    const t = s.trim();
    return /^([0-9a-f]{2}[:-]){5}[0-9a-f]{2}$/i.test(t) ||
      /^([0-9a-f]{4}\.){2}[0-9a-f]{4}$/i.test(t) ||
      /^[0-9a-f]{6}-[0-9a-f]{6}$/i.test(t);
  }

  function edgeVlanState(e, a, b, vid) {
    if (!vid || !e || !a || !b) return null;
    const onA = Array.isArray(e.fromVlans) && e.fromVlans.includes(vid);
    const onB = Array.isArray(e.toVlans) && e.toVlans.includes(vid);
    const isBackbone = !!(a.managed && b.managed);
    if (isBackbone) {
      const hasA = Array.isArray(a.vlans) && a.vlans.includes(vid);
      const hasB = Array.isArray(b.vlans) && b.vlans.includes(vid);
      if (!e.up) return (onA || onB || (hasA && hasB)) ? "down" : "dim";
      if (onA && onB) return e.blocking ? "blocking" : "active";
      if (onA !== onB) return "mismatch";
      if (hasA && hasB) return "untagged";
      return "dim";
    }
    const managedOn = a.managed ? onA : b.managed ? onB : false;
    if (!managedOn) return "dim";
    if (!e.up) return "down";
    if (e.blocking) return "blocking";
    return "leaf-active";
  }

  function computeVlanOverlay(vid) {
    const activeNodes = new Set();
    const edgeStates = new Map();
    const stats = { switches: 0, endpoints: 0, activeTrunks: 0, mismatches: 0, blocking: 0, down: 0, untagged: 0 };
    if (!vid) return null;
    for (const n of nodes.values()) {
      if (n.managed && Array.isArray(n.vlans) && n.vlans.includes(vid)) {
        activeNodes.add(n.id);
        stats.switches++;
      }
    }
    for (const e of topo.edges) {
      const a = nodes.get(e.from), b = nodes.get(e.to);
      if (!a || !b) continue;
      const st = edgeVlanState(e, a, b, vid) || "dim";
      edgeStates.set(e.id, st);
      if (st === "active") stats.activeTrunks++;
      else if (st === "mismatch") { stats.mismatches++; activeNodes.add(a.id); activeNodes.add(b.id); }
      else if (st === "blocking") { stats.blocking++; activeNodes.add(a.id); activeNodes.add(b.id); }
      else if (st === "down") { stats.down++; activeNodes.add(a.id); activeNodes.add(b.id); }
      else if (st === "untagged") stats.untagged++;
      else if (st === "leaf-active") {
        const leaf = a.managed ? b : a;
        if (!activeNodes.has(leaf.id)) { activeNodes.add(leaf.id); stats.endpoints++; }
      }
    }
    return { activeNodes, edgeStates, stats };
  }

  function setHighlightVlan(vid) {
    highlightVlan = Number(vid) > 0 ? Number(vid) : 0;
    const vo = highlightVlan ? computeVlanOverlay(highlightVlan) : null;
    if (opts?.onVlanChange) opts.onVlanChange(highlightVlan, vo ? vo.stats : null, topo);
    if (selected && opts?.select) opts.select(nodes.get(selected), topo, traffic);
    draw(performance.now());
    return vo ? vo.stats : null;
  }

  function loadTrafficPref() {
    try {
      const v = localStorage.getItem("ms-map-traffic");
      return v === null ? true : JSON.parse(v) !== false;
    } catch {
      return true;
    }
  }

  function size() {
    if (!cv) return;
    const r = cv.getBoundingClientRect();
    cv.width = Math.max(1, r.width * DPR());
    cv.height = Math.max(1, r.height * DPR());
    draw(performance.now());
  }

  function shortBps(bps) {
    const v = Number(bps) || 0;
    if (v <= 0) return "0";
    if (v < 1000) return `${Math.round(v)}b`;
    if (v < 1e6) return `${(v / 1e3).toFixed(v >= 1e5 ? 0 : 1)}K`;
    if (v < 1e9) return `${(v / 1e6).toFixed(v >= 1e8 ? 0 : 1)}M`;
    return `${(v / 1e9).toFixed(2)}G`;
  }

  function edgeActiveFlow(e) {
    const lt = traffic.edges?.[e.id];
    return !!(e.up && !e.blocking && lt && lt.live && (lt.tx > 100 || lt.rx > 100));
  }

  function refreshFlowFlag() {
    hasFlow = !!(liveTraffic && topo.edges.some(edgeActiveFlow));
    if (hasFlow && running && !raf) raf = requestAnimationFrame(tick);
  }

  async function pollTraffic() {
    if (!running || !opts?.loadTraffic) return;
    try {
      const res = await opts.loadTraffic();
      if (!running) return;
      traffic = res || { edges: {}, nodes: {} };
      refreshFlowFlag();
      if (selected && opts.onTraffic) opts.onTraffic(nodes.get(selected), topo, traffic);
      draw(performance.now());
    } catch {}
  }

  function startTelemetryTimers() {
    stopTelemetryTimers(false);
    if (!running) return;
    pollTraffic();
    pollTimer = setInterval(pollTraffic, 2000);
    if (liveTraffic && opts?.keepLive) {
      opts.keepLive(true);
      keepTimer = setInterval(() => { if (running && liveTraffic) opts.keepLive(true); }, 10000);
    }
  }

  function stopTelemetryTimers(sendOff) {
    if (keepTimer) { clearInterval(keepTimer); keepTimer = 0; }
    if (pollTimer) { clearInterval(pollTimer); pollTimer = 0; }
    if (sendOff && opts?.keepLive) opts.keepLive(false);
  }

  function toggleTraffic(next) {
    liveTraffic = typeof next === "boolean" ? next : !liveTraffic;
    try { localStorage.setItem("ms-map-traffic", JSON.stringify(liveTraffic)); } catch {}
    if (opts?.keepLive) opts.keepLive(liveTraffic);
    if (liveTraffic) startTelemetryTimers();
    else if (keepTimer) { clearInterval(keepTimer); keepTimer = 0; }
    refreshFlowFlag();
    draw(performance.now());
    return liveTraffic;
  }

  async function reload() {
    if (!running) return;
    try {
      topo = await opts.load();
    } catch (e) {
      opts.status("Couldn't load the map: " + e.message);
      return;
    }
    const next = new Map();
    let unplaced = 0;
    topo.nodes.forEach((n) => {
      const old = nodes.get(n.id);
      const node = Object.assign({}, n, { vx: 0, vy: 0 });
      if (old) {
        node.x = old.x;
        node.y = old.y;
        node.pinned = old.pinned;
      } else if (n.x != null && n.y != null) {
        node.x = n.x;
        node.y = n.y;
        node.pinned = true;
      } else {
        unplaced++;
      }
      next.set(n.id, node);
    });
    nodes = next;
    if (unplaced > 0) arrangeClean(unplaced < nodes.size);
    if (selected && !nodes.has(selected)) selected = null;
    const managed = topo.nodes.filter((n) => n.managed).length;
    opts.status(`${managed} switches, ${topo.nodes.length - managed} other devices, ${topo.edges.length} links`);
    if (highlightVlan > 0 && !(topo.vlans || []).some((v) => v.id === highlightVlan)) {
      highlightVlan = 0;
    }
    if (opts?.onTopo) opts.onTopo(topo, highlightVlan);
    if (highlightVlan > 0 && opts?.onVlanChange) {
      const vo = computeVlanOverlay(highlightVlan);
      opts.onVlanChange(highlightVlan, vo ? vo.stats : null, topo);
    }
    refreshFlowFlag();
    if (firstFit && topo.nodes.length) { firstFit = false; fit(); }
    if (selected) opts.select(nodes.get(selected), topo, traffic);
    draw(performance.now());
  }

  // Backbone-first tidy layout:
  // 1) Separate leaf endpoints (APs/phones/hosts/unmanaged leaves) connected to a single
  //    managed switch so they tuck into neat mini-grids right beneath their parent switch.
  // 2) Pick backbone roots by high managed-link degree (preferring true core STP roots),
  //    lay out managed switches top-down with generous row wrapping and anti-overlap slots.
  function arrangeClean(onlyUnplaced) {
    const allAdj = new Map([...nodes.keys()].map((id) => [id, []]));
    for (const e of topo.edges) {
      if (allAdj.has(e.from) && allAdj.has(e.to)) {
        allAdj.get(e.from).push(e.to);
        allAdj.get(e.to).push(e.from);
      }
    }

    // Attach unmanaged single-parent (or 1-neighbor) leaves directly to their parent switch
    const leafParent = new Map(); // leafId -> parentSwitchId
    const leavesByParent = new Map(); // switchId -> Node[]
    for (const n of nodes.values()) {
      if (n.managed) continue;
      const nbs = [...new Set(allAdj.get(n.id) || [])];
      const managedNbs = nbs.filter((id) => nodes.get(id)?.managed);
      if (managedNbs.length === 1 && nbs.length === 1) {
        const pid = managedNbs[0];
        leafParent.set(n.id, pid);
        if (!leavesByParent.has(pid)) leavesByParent.set(pid, []);
        leavesByParent.get(pid).push(n);
      }
    }
    for (const list of leavesByParent.values()) {
      list.sort((a, b) => (a.kind || "").localeCompare(b.kind || "") || (a.label || "").localeCompare(b.label || "", undefined, { numeric: true }));
    }

    const backbone = [...nodes.values()].filter((n) => !leafParent.has(n.id));
    const bbSet = new Set(backbone.map((n) => n.id));
    const bbAdj = new Map(backbone.map((n) => [n.id, []]));
    for (const e of topo.edges) {
      if (bbSet.has(e.from) && bbSet.has(e.to)) {
        bbAdj.get(e.from).push(e.to);
        bbAdj.get(e.to).push(e.from);
      }
    }

    // Count links to managed switches so a standalone 1-link switch claiming STP root
    // doesn't displace the real Core switch from Layer 0.
    const managedDeg = new Map();
    let maxManagedDeg = 0;
    for (const n of backbone) {
      const deg = (bbAdj.get(n.id) || []).filter((id) => nodes.get(id)?.managed).length;
      managedDeg.set(n.id, deg);
      if (n.managed && deg > maxManagedDeg) maxManagedDeg = deg;
    }

    const connectedBB = backbone.filter((n) => (bbAdj.get(n.id)?.length || 0) > 0);
    const minCoreDeg = Math.max(2, Math.floor(maxManagedDeg * 0.45));
    let roots = connectedBB.filter((n) => n.managed && n.stpRoot && managedDeg.get(n.id) >= minCoreDeg).map((n) => n.id);
    if (!roots.length) {
      const coreManaged = connectedBB.filter((n) => n.managed).sort((a, b) =>
        (managedDeg.get(b.id) - managedDeg.get(a.id)) ||
        ((bbAdj.get(b.id)?.length || 0) - (bbAdj.get(a.id)?.length || 0))
      );
      if (coreManaged.length) {
        const topDeg = managedDeg.get(coreManaged[0].id);
        roots = coreManaged.filter((n) => managedDeg.get(n.id) === topDeg && topDeg > 0).slice(0, 2).map((n) => n.id);
      }
      if (!roots.length) {
        const any = [...connectedBB].sort((a, b) => (bbAdj.get(b.id)?.length || 0) - (bbAdj.get(a.id)?.length || 0))[0];
        if (any) roots = [any.id];
      }
    }

    // BFS layer assignment across backbone nodes
    const layerOf = new Map(roots.map((id) => [id, 0]));
    const q = [...roots];
    while (q.length) {
      const id = q.shift();
      for (const nb of bbAdj.get(id) || []) {
        if (!layerOf.has(nb)) {
          layerOf.set(nb, layerOf.get(id) + 1);
          q.push(nb);
        }
      }
    }
    for (const n of connectedBB) {
      if (!layerOf.has(n.id)) {
        layerOf.set(n.id, 0);
        const cq = [n.id];
        while (cq.length) {
          const id = cq.shift();
          for (const nb of bbAdj.get(id) || []) {
            if (!layerOf.has(nb)) {
              layerOf.set(nb, layerOf.get(id) + 1);
              cq.push(nb);
            }
          }
        }
      }
    }
    const maxConnLayer = layerOf.size ? Math.max(...layerOf.values()) : 0;
    for (const n of backbone) {
      if (!layerOf.has(n.id)) layerOf.set(n.id, maxConnLayer + 1);
      n.layer = layerOf.get(n.id);
    }

    // Compute per-backbone-node bounding footprint (switch card + its tucked leaf cluster)
    const leafColsFor = (count) => (count <= 3 ? count : count <= 8 ? 4 : 5);
    const leafPitchX = 86;
    const leafPitchY = 54;
    const footOf = (n) => {
      const lc = (leavesByParent.get(n.id) || []).length;
      const baseW = n.managed ? SW.w + 76 : 112;
      const baseH = n.managed ? SW.h + 46 : 70;
      if (!lc) return { w: baseW, h: baseH, cols: 0, rows: 0 };
      const cols = leafColsFor(lc);
      const rows = Math.ceil(lc / cols);
      return {
        w: Math.max(baseW, cols * leafPitchX + 42),
        h: baseH + 34 + rows * leafPitchY,
        cols,
        rows,
      };
    };

    const byLayer = new Map();
    for (const n of backbone) {
      const l = n.layer ?? 0;
      if (!byLayer.has(l)) byLayer.set(l, []);
      byLayer.get(l).push(n);
    }
    const sortedLayers = [...byLayer.keys()].sort((a, b) => a - b);
    let curY = 0;

    for (const l of sortedLayers) {
      const list = byLayer.get(l);
      // Barycenter sort against already-positioned earlier-layer neighbors
      list.sort((a, b) => {
        const bary = (n) => {
          const xs = [];
          for (const nbId of bbAdj.get(n.id) || []) {
            const nb = nodes.get(nbId);
            if (nb && nb.x != null && (nb.layer ?? 0) < l) xs.push(nb.x);
          }
          return xs.length ? xs.reduce((s, v) => s + v, 0) / xs.length : 0;
        };
        const d = bary(a) - bary(b);
        if (Math.abs(d) > 1) return d;
        if (a.managed !== b.managed) return a.managed ? -1 : 1;
        return (a.label || "").localeCompare(b.label || "", undefined, { numeric: true });
      });

      // Split layer into balanced visual rows by cumulative width (~2100px max row width, max 8 nodes)
      const rows = [];
      let curRow = [], curW = 0;
      const maxRowNodes = list.some((n) => n.managed) ? 7 : 10;
      const maxRowW = 1950;
      for (const n of list) {
        const fw = footOf(n).w;
        if (curRow.length >= maxRowNodes || (curRow.length > 0 && curW + fw > maxRowW)) {
          rows.push(curRow);
          curRow = [];
          curW = 0;
        }
        curRow.push(n);
        curW += fw;
      }
      if (curRow.length) rows.push(curRow);

      for (let ri = 0; ri < rows.length; ri++) {
        const row = rows[ri];
        const totalW = row.reduce((s, n) => s + footOf(n).w, 0);
        let cx = -totalW / 2;
        let maxRowH = 0;
        for (const n of row) {
          const ft = footOf(n);
          const nx = cx + ft.w / 2;
          cx += ft.w;
          maxRowH = Math.max(maxRowH, ft.h);
          if (!(onlyUnplaced && n.x != null)) {
            n.x = nx;
            n.y = curY;
            n.vx = 0;
            n.vy = 0;
          }
          // Arrange attached leaves directly below this switch in a tidy mini-grid
          const leaves = leavesByParent.get(n.id) || [];
          const px = n.x, py = n.y;
          leaves.forEach((lf, idx) => {
            if (onlyUnplaced && lf.x != null) return;
            const col = idx % ft.cols;
            const r = Math.floor(idx / ft.cols);
            const rowCount = Math.min(ft.cols, leaves.length - r * ft.cols);
            lf.x = px + (col - (rowCount - 1) / 2) * leafPitchX;
            lf.y = py + 76 + r * leafPitchY;
            lf.vx = 0;
            lf.vy = 0;
          });
        }
        curY += maxRowH + (ri < rows.length - 1 ? 68 : 118);
      }
    }

    resolveOverlaps(onlyUnplaced);
    if (!onlyUnplaced) scheduleSave();
  }

  function resolveOverlaps(respectPinned) {
    const arr = [...nodes.values()].filter((n) => n.x != null && n.y != null);
    const dim = (n) => (n.managed ? [SW.w + 34, SW.h + 36] : [76, 48]);
    for (let pass = 0; pass < 40; pass++) {
      let moved = false;
      for (let i = 0; i < arr.length; i++) {
        for (let j = i + 1; j < arr.length; j++) {
          const a = arr[i], b = arr[j];
          const [wa, ha] = dim(a), [wb, hb] = dim(b);
          const dx = b.x - a.x, dy = b.y - a.y;
          const ox = (wa + wb) / 2 - Math.abs(dx), oy = (ha + hb) / 2 - Math.abs(dy);
          if (ox > 0 && oy > 0) {
            moved = true;
            const ap = !!(respectPinned && a.pinned), bp = !!(respectPinned && b.pinned);
            if (ox <= oy) {
              const sx = dx === 0 ? (i % 2 ? 1 : -1) : Math.sign(dx);
              const shift = ox + 4;
              if (ap && !bp) b.x += sx * shift;
              else if (bp && !ap) a.x -= sx * shift;
              else { a.x -= sx * shift * 0.5; b.x += sx * shift * 0.5; }
            } else {
              const sy = dy === 0 ? 1 : Math.sign(dy);
              const shift = oy + 4;
              if (ap && !bp) b.y += sy * shift;
              else if (bp && !ap) a.y -= sy * shift;
              else { a.y -= sy * shift * 0.5; b.y += sy * shift * 0.5; }
            }
          }
        }
      }
      if (!moved) break;
    }
  }

  function kick(a) {
    alpha = Math.max(alpha, a);
    if (!raf) raf = requestAnimationFrame(tick);
  }

  function tick(ts) {
    raf = 0;
    if (!running) return;
    if (alpha > 0.02) {
      step();
      if (alpha <= 0.02) {
        resolveOverlaps(true);
        scheduleSave();
        if (pendingFit) { pendingFit = false; fit(); }
      }
    }
    draw(ts || performance.now());
    if (alpha > 0.02 || hasFlow) {
      raf = requestAnimationFrame(tick);
    }
  }

  function step() {
    const arr = [...nodes.values()];
    const k = alpha;
    for (let i = 0; i < arr.length; i++) {
      for (let j = i + 1; j < arr.length; j++) {
        const a = arr[i], b = arr[j];
        const wa = a.managed ? SW.w + 34 : 76, wb = b.managed ? SW.w + 34 : 76;
        const ha = a.managed ? SW.h + 36 : 48, hb = b.managed ? SW.h + 36 : 48;
        const dx = b.x - a.x, dy = b.y - a.y;
        const ox = (wa + wb) / 2 - Math.abs(dx), oy = (ha + hb) / 2 - Math.abs(dy);
        if (ox > 0 && oy > 0) {
          if (ox <= oy) {
            const push = ox * 0.45 * Math.max(k, 0.25);
            const sx = dx === 0 ? 1 : Math.sign(dx);
            a.vx -= sx * push; b.vx += sx * push;
          } else {
            const push = oy * 0.45 * Math.max(k, 0.25);
            const sy = dy === 0 ? 1 : Math.sign(dy);
            a.vy -= sy * push; b.vy += sy * push;
          }
        }
      }
    }
    for (const n of arr) {
      if (n.pinned || (drag && drag.node === n)) { n.vx = n.vy = 0; continue; }
      n.vx *= 0.55; n.vy *= 0.55;
      n.x += Math.max(-20, Math.min(20, n.vx));
      n.y += Math.max(-20, Math.min(20, n.vy));
    }
    alpha *= 0.9;
  }

  // ---------- drawing ----------
  function toScreen(x, y) { return [x * view.k + view.tx, y * view.k + view.ty]; }
  function toWorld(x, y) { return [(x - view.tx) / view.k, (y - view.ty) / view.k]; }

  function qbez(ax, ay, cx, cy, bx, by, t) {
    const u = 1 - t;
    return [
      u * u * ax + 2 * u * t * cx + t * t * bx,
      u * u * ay + 2 * u * t * cy + t * t * by,
    ];
  }

  function rr(x, y, w, h, r) {
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.arcTo(x + w, y, x + w, y + h, r);
    ctx.arcTo(x + w, y + h, x, y + h, r);
    ctx.arcTo(x, y + h, x, y, r);
    ctx.arcTo(x, y, x + w, y, r);
    ctx.closePath();
  }

  function rectsOverlap(a, b, pad = 4) {
    return !(a.x + a.w + pad < b.x || b.x + b.w + pad < a.x || a.y + a.h + pad < b.y || b.y + b.h + pad < a.y);
  }

  function drawFlowDots(ax, ay, cx, cy, bx, by, bps, reverse, nowSec, seed, color, subtle) {
    if (!bps || bps <= 100) return;
    const rateScale = Math.log10(Math.max(100, bps) / 100);
    const speed = 0.15 + Math.min(0.62, rateScale * 0.105);
    const count = bps >= 5e7 ? 3 : 2;
    const rad = ((bps >= 1e7 ? 3.0 : 2.3) / Math.max(0.65, Math.min(1.4, view.k))) * (subtle ? 0.8 : 1);
    for (let p = 0; p < count; p++) {
      let t = (nowSec * speed + p / count + seed) % 1;
      if (reverse) t = 1 - t;
      const [px, py] = qbez(ax, ay, cx, cy, bx, by, t);
      ctx.beginPath();
      ctx.arc(px, py, rad * 2, 0, Math.PI * 2);
      ctx.fillStyle = color === C.tx
        ? (subtle ? "rgba(56,189,248,0.12)" : "rgba(56,189,248,0.2)")
        : (subtle ? "rgba(52,211,153,0.12)" : "rgba(52,211,153,0.2)");
      ctx.fill();

      ctx.beginPath();
      ctx.arc(px, py, rad, 0, Math.PI * 2);
      ctx.fillStyle = color;
      ctx.globalAlpha = subtle ? 0.65 : 1;
      ctx.fill();
      ctx.globalAlpha = 1;
    }
  }

  function measureRatePill(lt, hl) {
    const utilStr = lt.util >= 5 ? ` ${Math.round(lt.util)}%` : "";
    const txtTx = lt.tx >= 100 ? `↑${shortBps(lt.tx)}` : "";
    const txtRx = lt.rx >= 100 ? `↓${shortBps(lt.rx)}` : "";
    const parts = [txtTx, txtRx].filter(Boolean);
    if (!parts.length && !hl) return null;
    const text = (parts.length ? parts.join(" ") : "0 bps") + utilStr;
    ctx.font = '500 9.5px "Plex Mono", monospace';
    const w = ctx.measureText(text).width + 14;
    return { text, txtTx, txtRx, utilStr, parts, w, h: 16, lt, hl };
  }

  function drawMeasuredPill(mx, my, m) {
    const { text, txtTx, txtRx, utilStr, parts, w, h, lt, hl } = m;
    ctx.fillStyle = hl ? "rgba(15,23,42,.97)" : "rgba(11,17,32,.9)";
    ctx.strokeStyle = lt.util >= 80 ? "rgba(244,63,94,.7)" : lt.util >= 50 ? "rgba(245,158,11,.65)" : hl ? "rgba(56,189,248,.75)" : "rgba(56,189,248,.3)";
    ctx.lineWidth = 1 / Math.max(0.7, Math.min(1.2, view.k));
    rr(mx - w / 2, my - h / 2, w, h, 8);
    ctx.fill();
    ctx.stroke();

    ctx.font = '500 9.5px "Plex Mono", monospace';
    ctx.textBaseline = "middle";
    if (parts.length === 2) {
      const gap = 5;
      const wTx = ctx.measureText(txtTx).width;
      const wRx = ctx.measureText(txtRx).width;
      const wUtil = utilStr ? ctx.measureText(utilStr).width : 0;
      const total = wTx + gap + wRx + wUtil;
      let cx = mx - total / 2;
      ctx.textAlign = "left";
      ctx.fillStyle = C.tx;
      ctx.fillText(txtTx, cx, my + 0.5);
      cx += wTx + gap;
      ctx.fillStyle = C.rx;
      ctx.fillText(txtRx, cx, my + 0.5);
      if (utilStr) {
        cx += wRx;
        ctx.fillStyle = lt.util >= 80 ? C.crit : lt.util >= 50 ? C.warn : C.muted;
        ctx.fillText(utilStr, cx, my + 0.5);
      }
    } else {
      ctx.textAlign = "center";
      ctx.fillStyle = txtTx ? C.tx : txtRx ? C.rx : C.muted;
      ctx.fillText(text, mx, my + 0.5);
    }
  }

  function measureStatusPill(text, tone) {
    ctx.font = '600 9.5px "Plex Sans", sans-serif';
    const w = ctx.measureText(text).width + 14;
    return { kind: "vlan", text, tone, w, h: 17 };
  }

  function drawStatusPill(mx, my, m) {
    const { text, tone, w, h } = m;
    const bg = tone === "crit" ? "rgba(56,12,24,.95)" : tone === "warn" ? "rgba(52,32,8,.95)" : "rgba(10,32,48,.95)";
    const br = tone === "crit" ? "rgba(244,63,94,.85)" : tone === "warn" ? "rgba(245,158,11,.85)" : "rgba(34,211,238,.75)";
    const fg = tone === "crit" ? "#fda4af" : tone === "warn" ? "#fcd34d" : "#67e8f9";
    ctx.fillStyle = bg;
    ctx.strokeStyle = br;
    ctx.lineWidth = 1.1 / Math.max(0.7, Math.min(1.2, view.k));
    rr(mx - w / 2, my - h / 2, w, h, 8);
    ctx.fill();
    ctx.stroke();
    ctx.font = '600 9.5px "Plex Sans", sans-serif';
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillStyle = fg;
    ctx.fillText(text, mx, my + 0.5);
  }

  function draw(ts) {
    if (!ctx) return;
    lastAnimTs = ts || performance.now();
    const nowSec = lastAnimTs / 1000;
    const dpr = DPR();
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, cv.width, cv.height);
    // faint floor-plan dot grid
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    const stepGrid = 40 * view.k;
    if (stepGrid > 12) {
      ctx.fillStyle = "rgba(139,149,167,.10)";
      const ox = view.tx % stepGrid, oy = view.ty % stepGrid;
      for (let x = ox; x < cv.clientWidth; x += stepGrid) {
        for (let y = oy; y < cv.clientHeight; y += stepGrid) ctx.fillRect(x, y, 1.2, 1.2);
      }
    }
    ctx.setTransform(dpr * view.k, 0, 0, dpr * view.k, dpr * view.tx, dpr * view.ty);

    const pairCount = new Map(), pairIdx = new Map(), portLabels = [], badgeCandidates = [];
    for (const e of topo.edges) {
      const key = [e.from, e.to].sort().join("|");
      pairCount.set(key, (pairCount.get(key) || 0) + 1);
    }
    const focus = selected || hover;
    const vo = highlightVlan > 0 ? computeVlanOverlay(highlightVlan) : null;

    // 1) Draw edges & flow particles behind node boxes
    topo.edges.forEach((e, idx) => {
      const a = nodes.get(e.from), b = nodes.get(e.to);
      if (!a || !b) return;
      const key = [e.from, e.to].sort().join("|");
      const i = pairIdx.get(key) || 0; pairIdx.set(key, i + 1);
      const n = pairCount.get(key);
      const off = (i - (n - 1) / 2) * 28;
      const dx = b.x - a.x, dy = b.y - a.y, d = Math.hypot(dx, dy) || 1;
      const nx = -dy / d, ny = dx / d;
      const cx = (a.x + b.x) / 2 + nx * off, cy = (a.y + b.y) / 2 + ny * off;
      const isBackbone = !!(a.managed && b.managed);
      const hl = !!(focus && (e.from === focus || e.to === focus));
      const lt = traffic.edges?.[e.id];
      const active = !!(liveTraffic && e.up && !e.blocking && lt && lt.live && (lt.tx > 100 || lt.rx > 100));
      const vst = vo ? (vo.edgeStates.get(e.id) || "dim") : null;
      const kScale = Math.min(1, view.k + 0.2) / Math.max(0.6, view.k);

      if (vst) {
        if (vst === "active" || vst === "leaf-active") {
          // Soft outer cyan glow for active VLAN path
          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.quadraticCurveTo(cx, cy, b.x, b.y);
          ctx.lineWidth = (vst === "active" ? 7.5 : 4.8) * kScale;
          ctx.strokeStyle = vst === "active" ? "rgba(34,211,238,.22)" : "rgba(34,211,238,.14)";
          ctx.stroke();

          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.quadraticCurveTo(cx, cy, b.x, b.y);
          ctx.lineWidth = (vst === "active" ? 2.8 : 1.8) * kScale;
          ctx.strokeStyle = C.vlan;
          ctx.stroke();
        } else if (vst === "mismatch") {
          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.quadraticCurveTo(cx, cy, b.x, b.y);
          ctx.lineWidth = 7 * kScale;
          ctx.strokeStyle = "rgba(244,63,94,.20)";
          ctx.stroke();

          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.quadraticCurveTo(cx, cy, b.x, b.y);
          ctx.lineWidth = 2.8 * kScale;
          ctx.setLineDash([8, 5]);
          ctx.strokeStyle = C.crit;
          ctx.stroke();
          ctx.setLineDash([]);

          badgeCandidates.push({ a, b, cx, cy, measured: measureStatusPill("⚠ 1-side tag", "crit"), priority: 9e15 });
        } else if (vst === "blocking") {
          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.quadraticCurveTo(cx, cy, b.x, b.y);
          ctx.lineWidth = 6.5 * kScale;
          ctx.strokeStyle = "rgba(245,158,11,.20)";
          ctx.stroke();

          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.quadraticCurveTo(cx, cy, b.x, b.y);
          ctx.lineWidth = 2.6 * kScale;
          ctx.setLineDash([7, 5]);
          ctx.strokeStyle = C.warn;
          ctx.stroke();
          ctx.setLineDash([]);

          badgeCandidates.push({ a, b, cx, cy, measured: measureStatusPill("STP BLK", "warn"), priority: 8e15 });
        } else if (vst === "down") {
          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.quadraticCurveTo(cx, cy, b.x, b.y);
          ctx.lineWidth = 2.8 * kScale;
          ctx.strokeStyle = C.crit;
          ctx.stroke();

          badgeCandidates.push({ a, b, cx, cy, measured: measureStatusPill("LINK DOWN", "crit"), priority: 8.5e15 });
        } else if (vst === "untagged") {
          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.quadraticCurveTo(cx, cy, b.x, b.y);
          ctx.lineWidth = 1.6 * kScale;
          ctx.setLineDash([4, 5]);
          ctx.strokeStyle = "rgba(245,158,11,.48)";
          ctx.stroke();
          ctx.setLineDash([]);

          if (hl || view.k >= 0.75) {
            badgeCandidates.push({ a, b, cx, cy, measured: measureStatusPill("Not tagged", "warn"), priority: 5e15 });
          }
        } else {
          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.quadraticCurveTo(cx, cy, b.x, b.y);
          ctx.lineWidth = 0.9 * kScale;
          ctx.strokeStyle = "rgba(93,112,136,.12)";
          ctx.stroke();
        }
      } else {
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.quadraticCurveTo(cx, cy, b.x, b.y);
        ctx.lineWidth = (hl || (active && isBackbone) ? 2.2 : isBackbone ? 1.7 : 1.1) * kScale;
        ctx.setLineDash(e.blocking ? [7, 5] : []);
        if (!e.up) ctx.strokeStyle = C.crit;
        else if (e.blocking) ctx.strokeStyle = C.warn;
        else if (lt?.util >= 80 && active) ctx.strokeStyle = "rgba(244,63,94,.72)";
        else if (lt?.util >= 50 && active) ctx.strokeStyle = "rgba(245,158,11,.72)";
        else if (hl) ctx.strokeStyle = C.accent;
        else if (active && isBackbone) ctx.strokeStyle = "rgba(56,189,248,.48)";
        else if (!isBackbone) ctx.strokeStyle = "rgba(93,112,136,.42)";
        else ctx.strokeStyle = C.line;
        ctx.stroke();
        ctx.setLineDash([]);
      }

      // Animate dots prominently on backbone/external uplinks or focused leaf edges (dimmed VLAN edges stay quiet)
      const allowFlow = !vst || vst === "active" || vst === "leaf-active";
      if (allowFlow && active && (isBackbone || hl || view.k >= 0.85)) {
        const seed = ((idx * 37) % 100) / 100;
        const subtle = !isBackbone && !hl;
        drawFlowDots(a.x, a.y, cx, cy, b.x, b.y, lt.tx, false, nowSec, seed, C.tx, subtle);
        drawFlowDots(a.x, a.y, cx, cy, b.x, b.y, lt.rx, true, nowSec, (seed + 0.35) % 1, C.rx, subtle);
      }

      // Queue traffic pills: backbone links + focused links always; leaf links only when focused or zoomed in
      const showPill = allowFlow && liveTraffic && lt && lt.live && (lt.tx + lt.rx >= 1000 || hl) && (hl || (isBackbone && view.k >= 0.42) || view.k >= 1.15);
      if (showPill) {
        const measured = measureRatePill(lt, hl);
        if (measured) {
          badgeCandidates.push({
            a, b, cx, cy, measured,
            priority: (hl ? 1e14 : 0) + (isBackbone ? 1e12 : 0) + (lt.tx + lt.rx),
          });
        }
      }

      if (view.k > 1.5 || hl || vst === "mismatch" || vst === "blocking" || vst === "down") {
        if (!vst || vst !== "dim" || hl) {
          portLabels.push([e.fromPort, a, cx, cy], [e.toPort, b, cx, cy]);
        }
      }
    });

    // 2) Draw nodes cleanly over edges & particles
    const obstacles = [];
    for (const n of nodes.values()) {
      const vActive = !!(vo && vo.activeNodes.has(n.id));
      const vDim = !!(vo && !vActive);
      (n.managed ? drawSwitch : drawDevice)(n, vDim, vActive);
      obstacles.push(n.managed
        ? { x: n.x - SW.w / 2 - 4, y: n.y - SW.h / 2 - 4, w: SW.w + 8, h: SW.h + 8 }
        : { x: n.x - 16, y: n.y - 16, w: 32, h: 42 });
    }

    // 3) Draw port labels when hovered/focused/zoomed
    for (const l of portLabels) label(...l);

    // 4) Draw status & rate pills without overlapping any switch box or higher-priority pill
    badgeCandidates.sort((p, q) => q.priority - p.priority);
    const placedPills = [];
    const tryTs = [0.5, 0.64, 0.36, 0.74, 0.26, 0.82, 0.18];
    for (const c of badgeCandidates) {
      const { a, b, cx, cy, measured } = c;
      for (const t of tryTs) {
        const [mx, my] = qbez(a.x, a.y, cx, cy, b.x, b.y, t);
        const rect = { x: mx - measured.w / 2, y: my - measured.h / 2, w: measured.w, h: measured.h };
        const hitsNode = obstacles.some((o) => rectsOverlap(rect, o, 3));
        const hitsPill = placedPills.some((o) => rectsOverlap(rect, o, 5));
        if (!hitsNode && !hitsPill) {
          placedPills.push(rect);
          if (measured.kind === "vlan") drawStatusPill(mx, my, measured);
          else drawMeasuredPill(mx, my, measured);
          break;
        }
      }
    }
  }

  function label(text, n, cx, cy) {
    if (!text) return;
    const dx = cx - n.x, dy = cy - n.y, d = Math.hypot(dx, dy) || 1;
    const edge = n.managed ? Math.min(SW.w / 2 / (Math.abs(dx / d) || 1e-6), SW.h / 2 / (Math.abs(dy / d) || 1e-6)) : 16;
    const dist = Math.min(edge + 16, d * 0.45);
    const x = n.x + (dx / d) * dist, y = n.y + (dy / d) * dist;
    ctx.font = '500 10.5px "Plex Mono", monospace';
    const w = ctx.measureText(text).width + 8;
    ctx.fillStyle = "rgba(17,24,39,.94)";
    rr(x - w / 2, y - 8, w, 16, 3);
    ctx.fill();
    ctx.fillStyle = C.muted;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText(text, x, y + 0.5);
  }

  function drawSwitch(n, vDim, vActive) {
    const w = SW.w, h = SW.h, x = n.x - w / 2, y = n.y - h / 2;
    const sel = selected === n.id, hov = hover === n.id;
    const nt = traffic.nodes?.[n.id];

    ctx.save();
    if (vDim) ctx.globalAlpha = 0.2;

    if (vActive) {
      ctx.strokeStyle = "rgba(34,211,238,.24)";
      ctx.lineWidth = 6;
      rr(x - 1, y - 1, w + 2, h + 2, 9);
      ctx.stroke();
    }

    ctx.fillStyle = sel ? "#1e2e46" : hov ? C.panel2 : C.panel;
    ctx.strokeStyle = sel ? C.accent : vActive ? C.vlan : n.disabled || n.online ? C.border : C.crit;
    ctx.lineWidth = sel || vActive ? 2 : 1.25;
    rr(x, y, w, h, 8);
    ctx.fill();
    ctx.stroke();

    ctx.fillStyle = n.disabled ? "#64748b" : n.online ? C.up : C.crit;
    rr(x + 6, y + 9, 4, h - 18, 2);
    ctx.fill();

    ctx.fillStyle = C.text;
    ctx.font = '600 12.5px "Plex Sans", sans-serif';
    ctx.textAlign = "left";
    ctx.textBaseline = "middle";
    ctx.fillText(clip(n.label, w - 34), x + 17, y + 15);

    ctx.fillStyle = C.muted;
    ctx.font = '400 10.5px "Plex Mono", monospace';
    const maxHostW = nt?.live && nt.cpu >= 0 ? w - 68 : w - 30;
    ctx.fillText(clip(n.host, maxHostW), x + 17, y + 32);

    if (nt?.live && nt.cpu >= 0) {
      const cpuTxt = `${nt.cpu}%`;
      ctx.font = '600 9.5px "Plex Mono", monospace';
      const cw = ctx.measureText(cpuTxt).width + 8;
      ctx.fillStyle = nt.cpu >= 85 ? "rgba(244,63,94,.2)" : nt.cpu >= 65 ? "rgba(245,158,11,.2)" : "rgba(16,185,129,.16)";
      rr(x + w - cw - 8, y + 24, cw, 15, 4);
      ctx.fill();
      ctx.fillStyle = nt.cpu >= 85 ? "#fda4af" : nt.cpu >= 65 ? "#fcd34d" : "#6ee7b7";
      ctx.textAlign = "center";
      ctx.fillText(cpuTxt, x + w - cw / 2 - 8, y + 32);
    }

    if (n.severity) {
      ctx.fillStyle = { critical: C.crit, warning: C.warn, info: C.info }[n.severity];
      ctx.beginPath();
      ctx.arc(x + w - 11, y + 12, 4, 0, Math.PI * 2);
      ctx.fill();
    }
    if (n.stpRoot) {
      ctx.font = '600 10px "Plex Sans", sans-serif';
      const tw = ctx.measureText("STP root").width + 10;
      ctx.fillStyle = "#1e3a5f";
      rr(n.x - tw / 2, y - 18, tw, 15, 7);
      ctx.fill();
      ctx.fillStyle = "#7dd3fc";
      ctx.textAlign = "center";
      ctx.fillText("STP root", n.x, y - 10);
    }
    ctx.restore();
  }

  const KIND = { "external-switch": ["#7b8db0", "S"], router: ["#b08ad8", "R"], ap: ["#5bc2d6", "AP"], phone: ["#9aa5b8", "P"], host: ["#9aa5b8", "H"] };

  function drawDevice(n, vDim, vActive) {
    const [col, glyph] = KIND[n.kind] || KIND.host;
    const r = n.kind === "phone" || n.kind === "host" ? 10 : 13;
    const sel = selected === n.id;

    ctx.save();
    if (vDim) ctx.globalAlpha = 0.2;

    if (vActive) {
      ctx.beginPath();
      ctx.arc(n.x, n.y, r + 3.5, 0, Math.PI * 2);
      ctx.strokeStyle = "rgba(34,211,238,.38)";
      ctx.lineWidth = 3;
      ctx.stroke();
    }

    ctx.beginPath();
    ctx.arc(n.x, n.y, r, 0, Math.PI * 2);
    ctx.fillStyle = sel ? "#1e2e46" : "#111827";
    ctx.fill();
    ctx.lineWidth = sel || vActive ? 2 : 1.5;
    ctx.strokeStyle = sel ? C.accent : vActive ? C.vlan : col;
    ctx.stroke();
    ctx.fillStyle = vActive ? C.vlan : col;
    ctx.font = `600 ${glyph.length > 1 ? 8.5 : 10}px "Plex Sans", sans-serif`;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText(glyph, n.x, n.y + 0.5);
    if (view.k > 0.58 || selected === n.id || hover === n.id || vActive) {
      const macOnly = !n.label || looksLikeMAC(n.label);
      const mainText = (macOnly && n.vendor) ? n.vendor : (n.label || n.vendor || n.mac || "");
      ctx.fillStyle = vActive ? C.text : C.muted;
      ctx.font = (macOnly && n.vendor ? '500' : '400') + ' 10px "Plex Sans", sans-serif';
      ctx.fillText(clip(mainText, 88), n.x, n.y + r + 10);
      if (n.vendor && !macOnly && (view.k > 1.05 || sel || hover === n.id) && !mainText.toLowerCase().includes(n.vendor.toLowerCase())) {
        ctx.fillStyle = "rgba(148,163,184,.72)";
        ctx.font = '400 8.5px "Plex Sans", sans-serif';
        ctx.fillText(clip(n.vendor, 86), n.x, n.y + r + 21);
      }
    }
    ctx.restore();
  }

  function clip(t, max) {
    t = t || "";
    if (ctx.measureText(t).width <= max) return t;
    while (t.length > 1 && ctx.measureText(t + "…").width > max) t = t.slice(0, -1);
    return t + "…";
  }

  // ---------- interaction ----------
  function hit(px, py) {
    const [x, y] = toWorld(px, py);
    let best = null;
    for (const n of nodes.values()) {
      if (n.managed) {
        if (Math.abs(x - n.x) <= SW.w / 2 && Math.abs(y - n.y) <= SW.h / 2) best = n;
      } else if (Math.hypot(x - n.x, y - n.y) <= 15) best = n;
    }
    return best;
  }

  function pos(e) {
    const r = cv.getBoundingClientRect();
    return [e.clientX - r.left, e.clientY - r.top];
  }

  function onDown(e) {
    const [px, py] = pos(e);
    const n = hit(px, py);
    drag = { node: n, sx: px, sy: py, tx: view.tx, ty: view.ty, moved: false };
    if (n) { const [wx, wy] = toWorld(px, py); drag.ox = n.x - wx; drag.oy = n.y - wy; }
    cv.classList.add("dragging");
    cv.setPointerCapture(e.pointerId);
  }

  function onMove(e) {
    const [px, py] = pos(e);
    if (!drag) {
      const n = hit(px, py);
      const id = n ? n.id : null;
      if (id !== hover) { hover = id; cv.style.cursor = n ? "pointer" : ""; draw(performance.now()); }
      return;
    }
    if (Math.hypot(px - drag.sx, py - drag.sy) > 3) drag.moved = true;
    if (drag.node) {
      const [wx, wy] = toWorld(px, py);
      drag.node.x = wx + drag.ox;
      drag.node.y = wy + drag.oy;
      drag.node.pinned = true;
      kick(0.06);
    } else {
      view.tx = drag.tx + (px - drag.sx);
      view.ty = drag.ty + (py - drag.sy);
    }
    draw(performance.now());
  }

  function onUp() {
    if (!drag) return;
    const d = drag;
    drag = null;
    cv.classList.remove("dragging");
    if (!d.moved) {
      selected = d.node ? d.node.id : null;
      opts.select(d.node ? nodes.get(selected) : null, topo, traffic);
      draw(performance.now());
    } else if (d.node) {
      scheduleSave();
    }
  }

  function onWheel(e) {
    e.preventDefault();
    const [px, py] = pos(e);
    const factor = Math.exp(-e.deltaY * (e.ctrlKey ? 0.01 : 0.0015));
    const k = Math.min(3, Math.max(0.2, view.k * factor));
    const [wx, wy] = toWorld(px, py);
    view.k = k;
    view.tx = px - wx * k;
    view.ty = py - wy * k;
    draw(performance.now());
  }

  function scheduleSave() {
    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      const l = {};
      for (const n of nodes.values()) if (n.pinned || n.managed) l[n.id] = { x: Math.round(n.x), y: Math.round(n.y) };
      opts?.saveLayout?.(l).catch(() => {});
    }, 500);
  }

  function fit() {
    if (!nodes.size || !cv) return;
    let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
    for (const n of nodes.values()) {
      x0 = Math.min(x0, n.x - 105);
      y0 = Math.min(y0, n.y - 45);
      x1 = Math.max(x1, n.x + 105);
      y1 = Math.max(y1, n.y + 45);
    }
    const w = cv.clientWidth || 800, h = Math.max(200, (cv.clientHeight || 600) - 96);
    const k = Math.min(1.2, Math.max(0.2, Math.min(w / Math.max(1, x1 - x0), h / Math.max(1, y1 - y0))));
    view.k = k;
    view.tx = w / 2 - ((x0 + x1) / 2) * k;
    view.ty = 64 + h / 2 - ((y0 + y1) / 2) * k;
    draw(performance.now());
  }

  function relayout() {
    for (const n of nodes.values()) { n.pinned = false; n.x = null; n.y = null; }
    alpha = 0;
    arrangeClean(false);
    fit();
    draw(performance.now());
  }

  return {
    start(canvas, o) {
      cv = canvas; opts = o; ctx = cv.getContext("2d"); running = true; firstFit = true;
      liveTraffic = loadTrafficPref();
      highlightVlan = Number(o?.highlightVlan) > 0 ? Number(o.highlightVlan) : 0;
      cv.addEventListener("pointerdown", onDown);
      cv.addEventListener("pointermove", onMove);
      cv.addEventListener("pointerup", onUp);
      cv.addEventListener("pointercancel", onUp);
      cv.addEventListener("wheel", onWheel, { passive: false });
      cv.addEventListener("dblclick", (e) => { const n = hit(...pos(e)); if (n?.managed) location.hash = "#/switch/" + n.id; });
      ro = new ResizeObserver(size);
      ro.observe(cv);
      size();
      document.fonts?.ready.then(() => draw(performance.now()));
      reload();
      startTelemetryTimers();
    },
    stop(keepLive) {
      running = false;
      hasFlow = false;
      stopTelemetryTimers(!keepLive);
      if (raf) cancelAnimationFrame(raf);
      raf = 0;
      ro?.disconnect();
      ro = null;
    },
    reload, fit, relayout, toggleTraffic,
    setHighlightVlan,
    getHighlightVlan: () => highlightVlan,
    edgeVlanState,
    isTrafficOn: () => liveTraffic,
  };
})();
