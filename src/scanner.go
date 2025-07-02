package src

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

type Scanner struct {
	config          *Config
	db              *Database
	bettercap       *Bettercap
	probeCollector  *ProbeCollector
	whitelistBSSIDs map[string]bool
	globalTargets   map[string]*Target
	targetsMutex    sync.RWMutex
	scanning        bool
	scanMutex       sync.Mutex
}

func NewScanner(config *Config, db *Database, bettercap *Bettercap) *Scanner {
	probeCollector := NewProbeCollector(bettercap, db)
	return &Scanner{
		config:          config,
		db:              db,
		bettercap:       bettercap,
		probeCollector:  probeCollector,
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

func (s *Scanner) StartScanning() error {
	s.scanMutex.Lock()
	if s.scanning {
		s.scanMutex.Unlock()
		return nil
	}
	s.scanning = true
	s.scanMutex.Unlock()

	s.bettercap.RunCommand(fmt.Sprintf("set wifi.interface %s; set wifi.deauth.open false; wifi.recon.channel %s", s.config.Interface, s.GetChannelsForMode()))
	s.bettercap.RunCommand("wifi.recon on")

	s.probeCollector.Start()

	return nil
}

func (s *Scanner) StopScanning() {
	s.scanMutex.Lock()
	s.bettercap.RunCommand("wifi.recon off")
	s.scanning = false
	s.probeCollector.Stop()
	s.scanMutex.Unlock()
}

func (s *Scanner) GetTargets() ([]Target, error) {
	if !GetScanningEnabled() {
		return []Target{}, nil
	}

	sessionData, err := s.bettercap.GetSessionData()
	if err != nil {
		log.Printf("[ERROR] Failed to get session data: %v", err)
		return []Target{}, err
	}

	parsedTargets := s.parseTargets(sessionData)

	s.targetsMutex.Lock()
	s.globalTargets = make(map[string]*Target)

	var validTargets []Target
	for _, target := range parsedTargets {
		targetCopy := target

		exists, err := s.db.TargetExists(target.BSSID)
		if err != nil {
			log.Printf("[ERROR] Failed to check target existence: %v", err)
			continue
		}

		if !exists {
			log.Printf("[NEW] Discovered %s (%s) %ddBm", target.ESSID, target.BSSID, target.Signal)
			
			// Get GPS coordinates if GPS feature is enabled
			var gpsLat, gpsLong *float64
			if s.config.GPSDevice != "" {
				if gpsData, err := s.bettercap.GetLatestGPSData(); err == nil && gpsData != nil {
					gpsLat = &gpsData.Latitude
					gpsLong = &gpsData.Longitude
				}
			}
			
			s.db.SaveTarget(&target, "", StatusDiscovered, gpsLat, gpsLong)
		} else {
			// Target exists, check if we should update it with better signal
			existingSignal, err := s.db.GetTargetSignal(target.BSSID)
			if err == nil && target.Signal > existingSignal {
				log.Printf("[UPDATE] Better signal for %s (%s) %ddBm (was %ddBm)", target.ESSID, target.BSSID, target.Signal, existingSignal)
				
				// Get GPS coordinates if GPS feature is enabled
				var gpsLat, gpsLong *float64
				if s.config.GPSDevice != "" {
					if gpsData, err := s.bettercap.GetLatestGPSData(); err == nil && gpsData != nil {
						gpsLat = &gpsData.Latitude
						gpsLong = &gpsData.Longitude
					}
				}
				
				s.db.SaveTarget(&target, "", StatusDiscovered, gpsLat, gpsLong)
			}
		}

		if target.Signal < -70 || target.ESSID == "" {
			continue
		}

		s.globalTargets[target.BSSID] = &targetCopy
		validTargets = append(validTargets, targetCopy)
	}
	s.targetsMutex.Unlock()

	return validTargets, nil
}

func (s *Scanner) GetChannelsForMode() string {
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

	for i := 0; i < len(scoredTargets); i++ {
		for j := i + 1; j < len(scoredTargets); j++ {
			if scoredTargets[j].score > scoredTargets[i].score {
				scoredTargets[i], scoredTargets[j] = scoredTargets[j], scoredTargets[i]
			}
		}
	}

	for _, st := range scoredTargets {
		if strings.EqualFold(st.target.Encryption, "Open") ||
			strings.EqualFold(st.target.Encryption, "None") ||
			st.target.Encryption == "" {
			continue
		}

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
