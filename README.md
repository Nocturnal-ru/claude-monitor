# Claude Monitor

System tray app for Windows — shows Claude Pro usage limits at a glance.

## What it shows

- **Tray tooltip** (hover): `S:45% W:67%` (session / weekly)
- **Icon color**: green (<50%), yellow (50-80%), red (>80%)
- **Right-click menu**: detailed info with reset timers

## Build (on Fedora)

```bash
cd ~/projects/claude-monitor
chmod +x build.sh
./build.sh
```

## Setup (on Windows)

1. Copy `claude-monitor.exe` to any folder
2. Run it — creates `config.json` and `README-config.txt`
3. Edit `config.json` with your Firefox cookies
4. Run `claude-monitor.exe` again

## Getting cookies from Firefox

1. Open https://claude.ai in Firefox, log in
2. F12 → Storage → Cookies → https://claude.ai
3. Copy `sessionKey` value (starts with `sk-ant-sid01-...`)
4. Copy `lastActiveOrg` value (UUID format)
5. Paste both into `config.json`

## Autostart

Win+R → `shell:startup` → create shortcut to `claude-monitor.exe`
