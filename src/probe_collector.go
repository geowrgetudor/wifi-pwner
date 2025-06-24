package src

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

type ProbeCollector struct {
	bettercap *Bettercap
	db        *Database
	running   bool
	mutex     sync.Mutex
	stopChan  chan bool
}

func NewProbeCollector(bettercap *Bettercap, db *Database) *ProbeCollector {
	return &ProbeCollector{
		bettercap: bettercap,
		db:        db,
		stopChan:  make(chan bool, 1),
	}
}

func (pc *ProbeCollector) Start() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	
	if pc.running {
		return
	}
	
	pc.running = true
	go pc.collectProbes()
	log.Println("[PROBE] Probe collector started")
}

func (pc *ProbeCollector) Stop() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	
	if !pc.running {
		return
	}
	
	pc.running = false
	select {
	case pc.stopChan <- true:
	default:
	}
	log.Println("[PROBE] Probe collector stopped")
}

func (pc *ProbeCollector) IsRunning() bool {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()
	return pc.running
}

func (pc *ProbeCollector) collectProbes() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-pc.stopChan:
			return
		case <-ticker.C:
			pc.processEvents()
		}
	}
}

func (pc *ProbeCollector) processEvents() {
	events, err := pc.bettercap.GetEvents()
	if err != nil {
		log.Printf("[PROBE] Error getting events: %v", err)
		return
	}
	
	// Collect unique probes using a map with essid+mac as key
	uniqueProbes := make(map[string]BettercapEvent)
	
	for _, event := range events {
		if event.Tag == "wifi.client.probe" {
			// Parse the event to get essid and mac for deduplication
			var probeData map[string]interface{}
			if err := json.Unmarshal(event.Data, &probeData); err != nil {
				log.Printf("[PROBE] Error parsing probe data for deduplication: %v", err)
				continue
			}
			
			essid, _ := probeData["essid"].(string)
			mac, _ := probeData["mac"].(string)
			
			// Create composite key for uniqueness
			key := essid + "|" + mac
			
			// Only keep the latest event for each unique combination
			uniqueProbes[key] = event
		}
	}
	
	// Process only unique probes
	for _, event := range uniqueProbes {
		pc.processProbeEvent(event)
	}
	
	if len(uniqueProbes) > 0 {
		log.Printf("[PROBE] Processed %d unique probes out of %d total probe events", len(uniqueProbes), len(events))
	}
}

func (pc *ProbeCollector) processProbeEvent(event BettercapEvent) {
	// Parse the raw JSON data into a map for probe events
	var probeData map[string]interface{}
	if err := json.Unmarshal(event.Data, &probeData); err != nil {
		log.Printf("[PROBE] Error parsing probe data: %v", err)
		return
	}
	
	// Extract data from the parsed structure
	essid, _ := probeData["essid"].(string)
	mac, _ := probeData["mac"].(string)
	vendor, _ := probeData["vendor"].(string)
	
	// Convert RSSI to int (might be float64 from JSON)
	var rssi int
	if rssiFloat, ok := probeData["rssi"].(float64); ok {
		rssi = int(rssiFloat)
	}
	
	// Extract channel from RSSI or other available data
	// For now, we'll use a placeholder since channel isn't directly available in probe events
	channel := "unknown"
	
	err := pc.db.SaveProbe(essid, mac, rssi, channel, vendor)
	
	if err != nil {
		log.Printf("[PROBE] Error saving probe: %v", err)
		return
	}
	
	log.Printf("[PROBE] Saved probe: %s -> %s (RSSI: %d)", mac, essid, rssi)
}