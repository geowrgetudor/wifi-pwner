package src

import (
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
	
	for _, event := range events {
		if event.Tag == "wifi.client.probe" {
			pc.processProbeEvent(event)
		}
	}
}

func (pc *ProbeCollector) processProbeEvent(event BettercapEvent) {
	// Extract data from the flexible data structure
	essid, _ := event.Data["essid"].(string)
	mac, _ := event.Data["mac"].(string)
	vendor, _ := event.Data["vendor"].(string)
	
	// Convert RSSI to int (might be float64 from JSON)
	var rssi int
	if rssiFloat, ok := event.Data["rssi"].(float64); ok {
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