# Claude Monitor

Cross-platform system tray app (Windows 11 / Debian / Fedora) — shows Claude Pro usage limits at a glance.

## What it shows

- **Tray icon**: two-color split — left half = session (5h) remaining, right half = weekly remaining
- **Icon colors**: green ≥50%, amber 20–49%, red <20%
- **Icon text**: percentage remaining (e.g. `73%` / `41%`)
- **Tray tooltip** (hover): `S:73% W:41%`
- **Right-click menu**: detailed breakdown with reset timers

## Quick setup

On first launch, the app **automatically imports cookies from Firefox** (if you're logged in to claude.ai).
No manual editing needed in most cases.

If auto-import fails, use the menu item **"Import from Firefox"** or edit `config.json` manually.

---

## Download (pre-built binaries)

Go to [Releases](../../releases) and download:
- `claude-monitor-windows-amd64.exe` — Windows 11
- `claude-monitor-linux-amd64` — Debian 12+ / Fedora 39+

---

## Setup on Windows

1. Download `claude-monitor-windows-amd64.exe`
2. Run it — it auto-imports Firefox cookies and starts monitoring
3. If the tray shows "! Setup config.json first", open Firefox, log in to claude.ai, then use "Import from Firefox" in the tray menu

**Autostart:** Win+R → `shell:startup` → create a shortcut to `claude-monitor-windows-amd64.exe`

---

## Setup on Linux (Debian / Fedora / Ubuntu)

### 1. Install runtime dependency

The app uses the system tray via AppIndicator. Install it once:

**Debian / Ubuntu:**
```bash
sudo apt install libayatana-appindicator3-1
```

**Fedora:**
```bash
sudo dnf install libappindicator-gtk3
```

> **GNOME users:** GNOME Shell removed system tray support. Install the
> [AppIndicator and KStatusNotifierItem Support](https://extensions.gnome.org/extension/615/appindicator-support/)
> extension, then log out and back in.

### 2. Run the binary

```bash
chmod +x claude-monitor-linux-amd64
./claude-monitor-linux-amd64
```

The app auto-imports Firefox cookies on first launch.

### 3. Autostart (Linux)

Create `~/.config/autostart/claude-monitor.desktop`:

```ini
[Desktop Entry]
Type=Application
Name=Claude Monitor
Exec=/path/to/claude-monitor-linux-amd64
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
```

---

## Getting cookies manually (if auto-import fails)

1. Open https://claude.ai in Firefox, log in
2. F12 → Storage → Cookies → https://claude.ai
3. Copy `sessionKey` value (starts with `sk-ant-sid01-...`)
4. Copy `lastActiveOrg` value (UUID format)
5. Edit `config.json` next to the executable and paste both values

---

## Build from source

Requires Go 1.21+.

**Build for Windows** (cross-compile from Linux):
```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o claude-monitor-windows-amd64.exe .
```

**Build for Linux** (native on Linux):
```bash
# Install build dependencies first:
# Debian: sudo apt install libayatana-appindicator3-dev gcc
# Fedora:  sudo dnf install libayatana-appindicator-gtk3-devel gcc
CGO_ENABLED=1 go build -ldflags="-s -w" -o claude-monitor-linux-amd64 .
```

---

## Work with the repository locally

```bash
git clone https://github.com/Nocturnal-ru/claude-monitor.git
cd claude-monitor
```

---

## Creating a release

Push a version tag — GitHub Actions automatically builds both binaries and creates a release:

```bash
git tag v1.0.1
git push origin v1.0.1
```

The release will appear at https://github.com/Nocturnal-ru/claude-monitor/releases with both
`claude-monitor-windows-amd64.exe` and `claude-monitor-linux-amd64` attached.

---

## Notes

- `sessionKey` expires roughly once a month — use "Import from Firefox" to refresh
- `cf_clearance` (Cloudflare token) in `config.json` is optional; the app retries without it
- Logs are written to `claude-monitor.log` next to the executable
