<p align="center">
  <img src="docs/banner.svg" alt="MySwitch - Aruba NOC" width="840">
</p>

<p align="center">
  <b>MySwitch</b> is a desktop Network Operations Center (NOC) &amp; configuration manager built specifically for <b>Aruba Switches</b> (AOS-CX, AOS-S / ProCurve, and Instant On).
</p>

<p align="center">
  <img alt="macOS" src="https://img.shields.io/badge/macOS-Universal-0f172a?style=flat-square&logo=apple&logoColor=white">
  <img alt="Windows" src="https://img.shields.io/badge/Windows-10%20%7C%2011-0f172a?style=flat-square&logo=windows&logoColor=white">
  <img alt="Version" src="https://img.shields.io/badge/version-1.0.21-0ea5e9?style=flat-square">
</p>

<p align="center">
  <a href="https://github.com/kepinandreew/switchAruba/releases"><b>Releases</b></a> ·
  <a href="https://github.com/kepinandreew/switchAruba/actions"><b>Builds (Actions)</b></a>
</p>

---

## Key Features

- **Real-Time Fleet Dashboard & KPI Cards** — Instant overview of online/offline switches, critical alerts, warnings, and interactive numbered port faceplates (`S / M / L / XL` zoom).
- **Direct Interactive Port VLAN Configuration** — Click ports directly on the visual faceplate or select rows in the Ports table to configure **Access (Untagged)** or **Trunk (Native + Tagged)** VLANs without writing CLI commands manually.
- **Bulk IP Range Discovery & Auto-Cleanup** — Add entire subnets or IP ranges (e.g. `10.0.0.2 - 10.0.0.50`), sort switches naturally by IP, and remove unreachable hosts in one click with **Delete offline**.
- **Port Naming & LLDP Auto-Discovery** — Rename ports directly from the table or auto-populate port descriptions from connected LLDP neighbors.
- **Network Topology Map & Client Locator** — Interactive LLDP/CDP topology map and network-wide MAC address path finder.
- **Safe Batch Firmware Management** — Upload firmware via SSH/SFTP (AOS-S) or HTTPS REST (AOS-CX) with bottom-up topology reboot ordering and per-switch firmware hold locks.

---

## Building from Source

```bash
wails build -clean -ldflags "-X main.Version=1.0.21"
```
