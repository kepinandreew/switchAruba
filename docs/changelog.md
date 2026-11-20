# Changelog

## 1.0.30
- **Auto MAC Vendor / OUI Detection (Offline, 0% SSH)**:
  - Embedded offline IEEE OUI hardware database (40,500+ prefixes) with specialized Pro-AV & networking brand labels (`Audinate (Dante)`, `Yamaha`, `Shure`, `QSC / Q-SYS`, `Allen & Heath`, `Biamp`, `Crestron`, `Extron`, `Blackmagic Design`, `NovaStar`, `MA Lighting`, `Focusrite`, `Sennheiser`, `Neutrik`, `HPE / Aruba`, `Hikvision`, `Dahua`, `Axis`, `Apple`, `Dell`, plus randomized/private MAC detection).
  - Automatically displays vendor chips across **Clients** (with MAC path view & CSV export), **Switch Ports**, **Global Ports**, and **Network Map** endpoints (replacing raw MAC labels with brand names).
  - Search directly by brand name (e.g. `audinate`, `yamaha`, `shure`, `hikvision`, `apple`, `aruba`) inside **Clients** and **Ports** (both per-switch and global).
- **Highlight VLAN Path Overlay on Network Map (0% SSH)**:
  - Added a **Highlight VLAN** dropdown in the Network Map toolbar listing all VLANs across visible switches (site-aware).
  - Highlights switches, endpoints, and healthy trunks carrying the chosen VLAN in bright cyan while dimming unrelated topology, and calls out broken paths visually: **⚠ 1-side tag** (asymmetric trunk mismatch, including LAG/Trk aliases), **STP BLK** (Spanning Tree blocked), **LINK DOWN**, and **Not tagged** trunks, with live summary stats and per-link diagnostics in the switch detail card.

## 1.0.29
- **On-Demand Cable Diagnostics (TDR Test) per Port**:
  - Added copper RJ45 Time-Domain Reflectometer (TDR) testing directly from each switch's **Ports** tab (`🔬` per port / **🔬 Cable Test** when 1 port is selected), using `diag cable-diagnostic test`/`show` on AOS-CX and `test cable-diagnostics`/`show cable-diagnostics` on AOS-S.
  - Displays a 4-pair health table (`1-2`, `3-6`, `4-5`, `7-8`) with status pills (`good`/`OK`, `open`, `short`/`intra-short`/`inter-short`, impedance, MDI mode, and cable length / distance to fault in meters), automatic uplink warnings before disrupting a link, automatic fiber/SFP port detection, and raw CLI transcript.
  - Strictly on-demand (**0% idle SSH/CPU load**).

## 1.0.28
- **Global Config Search & Upgraded Visual Config Diff (0% SSH load)**:
  - New **Configs** sidebar view searches across every switch's cached `running-config` in RAM (substring or Regex, site-filtered, with quick Aruba operational presets, line numbers, highlighting, CSV export, and 1-click jump straight to the matching line in that switch's Config tab).
  - Upgraded **Config Diff** on each switch's Config tab with a 1-click **Diff vs previous** button, `+added / -removed` summary bar, **Changes only (±3 lines)** collapsed hunk view, and **Export diff** (`.patch`).
- **Zero-SSH Network Health Audits**:
  - **Trunk Uplink VLAN Mismatch**: Detects switch-to-switch uplinks (including LAG/Trk members) where both switches define a VLAN but only one side of the trunk carries it.
  - **Port Speed Downgrade & Half-Duplex Alerts**: Flags half-duplex ports (Warning), Gigabit uplink/AP ports downgraded to `100M/10M` (Warning), and Gigabit access ports at `100M/10M` (Info).
  - **Switch Power / PoE Budget Monitoring**: Parses PSU wattage usage (`PowerUsedW / PowerMaxW`) directly from already-collected power-supply output (plus Instant On PoE budgets), displays a **Power / PSU** meter in Switch Overview, and alerts at ≥80% (Warning) / ≥90% (Critical).
- **Excel-Ready CSV Export (UTF-8 BOM)**: One-click **Export CSV** buttons on **Switches**, **Ports** (global & per-switch), **VLAN Matrix**, **Clients**, and **Configs** search results.

## 1.0.27
- **Auto-Enable Dante QoS & Aruba Dante AV Profile**:
  - Selecting an audio/Dante VLAN (e.g. `Sound`, `Dante`, `Audio`, `AES67`) inside **Configure VLAN** now automatically selects **QoS Trust: DSCP** and **Disable EEE** (`no energy-efficient-ethernet`), while keeping full CLI transparency and manual override.
  - Zero-extra-SSH **Dante AV Readiness** audit parses global/per-port DSCP trust, DSCP 56/46 priority maps, Strict Priority schedule profiles, IGMP Snooping, and EEE directly from `running-config`, surfaced both in Issues and a dedicated **Dante AV Readiness** card on Switch Overview.
  - One-click **Aruba Dante AV Profile** modal (AOS-CX & AOS-S) previews and applies DSCP trust, DSCP 56 (PTP) → Queue 7 & DSCP 46 (Audio) → Queue 6 mappings, Strict Priority (`dante_strict`), IGMP Snooping / optional Querier, and `no energy-efficient-ethernet`.
- **Port Admin Enable / Disable & Link Bounce**: Enable (`no shutdown`/`enable`), disable (`shutdown`/`disable`), or bounce single or multiple ports directly from the Switch Ports tab with uplink neighbor warnings and CLI preview.

## 1.0.26
- **Direct Client MAC Search in Switches & Ports**: Search client MAC addresses (`aa:bb:cc`, `9abc`, etc.) directly from the **Switches** and global **Ports** search bars using zero-overhead in-memory MAC table matching, with clickable port jump pills (`MAC ... → port`).
- **Multicast / IGMP Snooping Audit & Configuration**: Zero-extra-SSH running-config audit flags audio/DANTE/video/DSCP VLANs with IGMP snooping disabled and detects L2 domains lacking an IGMP querier; displays per-VLAN and matrix IGMP badges (`IGMP On`, `IGMP + Querier`, `IGMP Off`) and lets you enable/disable **IGMP Snooping** and **IGMP Querier** per switch or in batch across VLANs.

## 1.0.25
- Accurate rolling **1-minute / 5-minute average CPU** telemetry (`show system resource-utilization` on AOS-CX and lightweight `show cpu` on AOS-S) without adding polling overhead, eliminating false instantaneous CLI spikes while preserving instant/1m/5m details on hover and switch overview.

## 1.0.24
- Improved **Network map** layout engine: attaches AP/phone/host leaf devices in compact clusters under their parent switch, prioritizes true core switches at the top, and prevents rate pills from overlapping switch cards or one another.

## 1.0.23
- Animated live bidirectional link traffic (`Tx`/`Rx` bps particles and rate badges) on **Network map** with a quick **Live traffic** toggle.
- Tidy hierarchical **Network map** auto-layout and non-overlapping **Arrange again** placement for large switch topologies.

## 1.0.22
- Live **Monitor** dashboard tracking per-switch CPU load, memory usage, aggregate Rx/Tx traffic sparklines, and top active ports.
- Per-port PoE control (On / Off / Bounce) and faster SSH command execution.
- **QoS Trust DSCP** (`qos trust dscp` / DANTE preset) option inside Port VLAN configuration.
- Search switches by port description with one-click port jump highlights.

## 1.0.21
- Redesigned MySwitch Cyber-NOC user interface, custom branding, and KPI dashboard cards.
- Interactive Port VLAN configuration (Access & Trunk) directly from the Ports faceplate and table.
- Port naming, LLDP auto-naming, global Ports view, sticky VLAN matrix pan, and safer bottom-up firmware batch updates.
