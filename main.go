package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/getlantern/systray"
)

const (
	appName        = "Claude Monitor"
	updateInterval = 5 * time.Minute
)

var (
	configPath string
	logFile    *os.File
)

func main() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal("Cannot determine executable path:", err)
	}
	exeDir := filepath.Dir(exePath)
	configPath = filepath.Join(exeDir, "config.json")

	// Setup logging
	logPath := filepath.Join(exeDir, "claude-monitor.log")
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting", appName)

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconGray)
	systray.SetTitle("")
	systray.SetTooltip(appName + ": loading...")

	mHeader := systray.AddMenuItem(appName, "")
	mHeader.Disable()
	systray.AddSeparator()

	mSession := systray.AddMenuItem("Session (5h): ...", "5-hour sliding window limit")
	mSession.Disable()
	mWeekly := systray.AddMenuItem("Weekly: ...", "Weekly limit")
	mWeekly.Disable()
	mSonnet := systray.AddMenuItem("Sonnet: ...", "Weekly Sonnet limit")
	mSonnet.Disable()

	systray.AddSeparator()
	mRefresh := systray.AddMenuItem("Refresh now", "Fetch data now")
	mEditCfg := systray.AddMenuItem("Open config", "Edit config.json")
	mOpenLog := systray.AddMenuItem("Open log", "Open log file")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Close application")

	// Check config
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Println("Config not found, creating template:", err)
		createTemplateConfig(configPath)
		systray.SetTooltip(appName + ": setup config.json!")
		mHeader.SetTitle("! Setup config.json first")
	} else {
		log.Println("Config loaded, org_id:", cfg.OrgID[:8]+"...")
	}

	// Menu click handlers
	go func() {
		for {
			select {
			case <-mRefresh.ClickedCh:
				log.Println("Manual refresh")
				doUpdate(mSession, mWeekly, mSonnet)
			case <-mEditCfg.ClickedCh:
				openFile(configPath)
			case <-mOpenLog.ClickedCh:
				dir := filepath.Dir(configPath)
				openFile(filepath.Join(dir, "claude-monitor.log"))
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()

	// Auto-update loop with jitter to avoid predictable request patterns
	go func() {
		time.Sleep(2 * time.Second)
		doUpdate(mSession, mWeekly, mSonnet)

		for {
			// ±30 second jitter around updateInterval
			jitter := time.Duration(rand.Int63n(60)-30) * time.Second
			time.Sleep(updateInterval + jitter)
			doUpdate(mSession, mWeekly, mSonnet)
		}
	}()
}

func onExit() {
	log.Println("Exiting", appName)
	if logFile != nil {
		logFile.Close()
	}
}

func doUpdate(mSession, mWeekly, mSonnet *systray.MenuItem) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Println("Config error:", err)
		systray.SetIcon(iconGray)
		systray.SetTooltip(appName + ": config error")
		mSession.SetTitle("! Error: setup config.json")
		return
	}

	usage, err := fetchUsage(cfg)
	if err != nil {
		log.Println("API error:", err)
		systray.SetIcon(iconGray)
		systray.SetTooltip(appName + ": API error")
		mSession.SetTitle("! API error (see log)")
		return
	}

	sessionPct := int(usage.FiveHour.Utilization)
	weeklyPct := int(usage.SevenDay.Utilization)

	// Tooltip: compact two numbers
	systray.SetTooltip(fmt.Sprintf("S:%d%% W:%d%%", sessionPct, weeklyPct))

	// Generate two-color icon: left=session remaining, right=weekly remaining
	systray.SetIcon(makeIcon(100-sessionPct, 100-weeklyPct))

	// Detailed menu items
	mSession.SetTitle(fmt.Sprintf("Session (5h): %d%% — reset %s",
		sessionPct, formatReset(usage.FiveHour.ResetsAt)))
	mWeekly.SetTitle(fmt.Sprintf("Weekly: %d%% — reset %s",
		weeklyPct, formatReset(usage.SevenDay.ResetsAt)))

	if usage.SevenDaySonnet != nil {
		mSonnet.SetTitle(fmt.Sprintf("Sonnet: %d%% — reset %s",
			int(usage.SevenDaySonnet.Utilization),
			formatReset(usage.SevenDaySonnet.ResetsAt)))
	} else {
		mSonnet.SetTitle("Sonnet: n/a")
	}

	log.Printf("OK: session=%d%% weekly=%d%%", sessionPct, weeklyPct)
}

func formatReset(isoTime string) string {
	t, err := time.Parse(time.RFC3339Nano, isoTime)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.000000+00:00", isoTime)
		if err != nil {
			return "?"
		}
	}

	diff := time.Until(t)
	if diff <= 0 {
		return "soon"
	}

	h := int(diff.Hours())
	m := int(diff.Minutes()) % 60

	if h > 24 {
		return fmt.Sprintf("in %dd %dh", h/24, h%24)
	}
	if h > 0 {
		return fmt.Sprintf("in %dh %dm", h, m)
	}
	return fmt.Sprintf("in %dm", m)
}

func openFile(path string) {
	if runtime.GOOS == "windows" {
		exec.Command("notepad.exe", path).Start()
	} else {
		exec.Command("xdg-open", path).Start()
	}
}
