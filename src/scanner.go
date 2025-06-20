package src

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type Scanner struct {
	config          *Config
	db              *Database
	bettercap       *Bettercap
	whitelistBSSIDs map[string]bool
	globalTargets   map[string]*Target
	targetsMutex    sync.RWMutex
	scanning        bool
	scanMutex       sync.Mutex
}

func NewScanner(config *Config, db *Database, bettercap *Bettercap) *Scanner {
	return &Scanner{
		config:          config,
		db:              db,
		bettercap:       bettercap,
		whitelistBSSIDs: make(map[string]bool),
		globalTargets:   make(map[string]*Target),
		scanning:        false,
	}
}

func (s *Scanner) LoadWhitelist() error {
	if s.config.WhitelistFile == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(s.config.WhitelistFile); os.IsNotExist(err) {
		// File doesn't exist, skip whitelist
		return nil
	}

	file, err := os.Open(s.config.WhitelistFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			s.whitelistBSSIDs[strings.ToUpper(line)] = true
			count++
		}
	}

	if count > 0 {
		log.Printf("[CONFIG] Whitelist loaded: %d BSSIDs", count)
	}
	return scanner.Err()
}

func (s *Scanner) StartContinuousScanning() error {
	s.scanMutex.Lock()
	if s.scanning {
		s.scanMutex.Unlock()
		return nil
	}
	s.scanning = true
	s.scanMutex.Unlock()

	// Set up bettercap for scanning
	s.bettercap.RunCommand(fmt.Sprintf("set wifi.interface %s", s.config.Interface))
	s.bettercap.RunCommand("set wifi.rssi.min -70")
	s.bettercap.RunCommand("set wifi.deauth.open false")

	channels := s.getChannelsForMode()
	s.bettercap.RunCommand(fmt.Sprintf("wifi.recon.channel %s", channels))
	s.bettercap.RunCommand("wifi.recon on")

	go s.continuousScanning()
	return nil
}

func (s *Scanner) continuousScanning() {
	for s.scanning {
		sessionData, err := s.bettercap.GetSessionData()
		if err != nil {
			log.Printf("[ERROR] Failed to get session data: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		targets := s.parseTargets(sessionData)
		
		// Refresh global targets with current session data
		s.targetsMutex.Lock()
		// Clear existing targets and replace with fresh data
		s.globalTargets = make(map[string]*Target)
		
		for _, target := range targets {
			targetCopy := target
			s.globalTargets[target.BSSID] = &targetCopy
			
			// Check if this target exists in the database
			exists, err := s.db.TargetExists(target.BSSID)
			if err != nil {
				log.Printf("[ERROR] Failed to check target existence: %v", err)
				continue
			}
			
			// Save new targets to database
			if !exists {
				log.Printf("[NEW] Discovered %s (%s) %ddBm", target.ESSID, target.BSSID, target.Signal)
				s.db.SaveTarget(&target, "", StatusDiscovered)
			}
		}
		s.targetsMutex.Unlock()

		time.Sleep(10 * time.Second) // Refresh every 10 seconds
	}
}

func (s *Scanner) StopContinuousScanning() {
	s.scanMutex.Lock()
	s.scanning = false
	s.scanMutex.Unlock()
}

func (s *Scanner) ScanForTargets() ([]Target, error) {
	// This method now returns targets from the global variable
	s.targetsMutex.RLock()
	defer s.targetsMutex.RUnlock()

	targets := make([]Target, 0, len(s.globalTargets))
	for _, target := range s.globalTargets {
		targets = append(targets, *target)
	}

	return targets, nil
}

func (s *Scanner) GetChannels() string {
	return s.getChannelsForMode()
}

func (s *Scanner) getChannelsForMode() string {
	switch s.config.Mode {
	case "2.4":
		return "1,2,3,4,5,6,7,8,9,10,11,12,13"
	case "5":
		return "36,40,44,48,52,56,60,64,100,104,108,112,116,120,124,128,132,136,140"
	default:
		return "1,2,3,4,5,6,7,8,9,10,11,12,13,36,40,44,48,52,56,60,64,100,104,108,112,116,120,124,128,132,136,140"
	}
}

func (s *Scanner) parseTargets(sessionData *SessionData) []Target {
	var targets []Target

	for _, ap := range sessionData.WiFi.APs {
		if ap.RSSI < -70 || ap.Hostname == "" {
			continue
		}

		if s.whitelistBSSIDs[strings.ToUpper(ap.MAC)] {
			continue
		}

		target := Target{
			BSSID:      ap.MAC,
			ESSID:      ap.Hostname,
			Signal:     ap.RSSI,
			Frequency:  ap.Frequency,
			Encryption: ap.Encryption,
		}

		if ap.Channel > 0 {
			target.Channel = fmt.Sprintf("%d", ap.Channel)
		} else if ap.Frequency > 0 {
			if ap.Frequency < 3000 {
				target.Channel = fmt.Sprintf("%d", (ap.Frequency-2412)/5+1)
			} else {
				target.Channel = fmt.Sprintf("%d", (ap.Frequency-5000)/5)
			}
		}

		targets = append(targets, target)
	}

	return targets
}

func (s *Scanner) FindBestAvailableTarget(targets []Target) *Target {
	type scoredTarget struct {
		target *Target
		score  int
	}

	var scoredTargets []scoredTarget

	for i := range targets {
		target := &targets[i]

		score := target.Signal
		scoredTargets = append(scoredTargets, scoredTarget{target: target, score: score})
	}

	// Sort by score (highest first)
	for i := 0; i < len(scoredTargets); i++ {
		for j := i + 1; j < len(scoredTargets); j++ {
			if scoredTargets[j].score > scoredTargets[i].score {
				scoredTargets[i], scoredTargets[j] = scoredTargets[j], scoredTargets[i]
			}
		}
	}

	// Find the first target that shouldn't be skipped
	for _, st := range scoredTargets {
		skip, err := s.db.ShouldSkipTarget(st.target.BSSID)
		if err != nil {
			continue
		}

		if !skip {
			return st.target
		}
	}

	return nil
}
