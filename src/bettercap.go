package src

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Bettercap struct {
	config  *Config
	process *exec.Cmd
	mutex   sync.Mutex
}

func NewBettercap(config *Config) *Bettercap {
	return &Bettercap{
		config: config,
	}
}

func (b *Bettercap) Start() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if _, err := exec.LookPath("bettercap"); err != nil {
		return fmt.Errorf("bettercap not found in PATH: %v", err)
	}

	if _, err := os.Stat("/sys/class/net/" + b.config.Interface); os.IsNotExist(err) {
		return fmt.Errorf("network interface %s not found", b.config.Interface)
	}

	// Try to randomize MAC address before starting bettercap
	b.randomizeMACBeforeStart()

	apiAddress := "127.0.0.1"
	if b.config.BettercapApiExpose {
		apiAddress = "0.0.0.0"
	}

	evalCmd := fmt.Sprintf(
		"set api.rest.port %s; set api.rest.address %s; api.rest on; events.stream on; set wifi.handshakes.aggregate false",
		b.config.BettercapAPIPort,
		apiAddress,
	)

	if b.config.GPSDevice != "" {
		evalCmd += fmt.Sprintf("; set gps.device %s; set gps.baudrate %d; gps on", 
			b.config.GPSDevice, 
			b.config.GPSBaudRate)
	}

	b.process = exec.Command("bettercap", "-iface", b.config.Interface, "-eval", evalCmd)

	if err := b.process.Start(); err != nil {
		return fmt.Errorf("failed to start bettercap: %v", err)
	}

	log.Printf("[INIT] Bettercap started (API: %s)", b.config.BettercapAPIPort)

	time.Sleep(5 * time.Second)

	return nil
}

func (b *Bettercap) Stop() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.process != nil {
		b.process.Process.Kill()
		b.process.Wait()
		b.process = nil
	}
}

func (b *Bettercap) RunCommand(command string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	cmd := BettercapCommand{Cmd: command}
	jsonData, err := json.Marshal(cmd)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf(BettercapSessionURL, b.config.BettercapAPIPort)
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if data, ok := result["data"].(string); ok {
		return data, nil
	}

	return "", nil
}

func (b *Bettercap) GetSessionData() (*SessionData, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	apiURL := fmt.Sprintf(BettercapSessionURL, b.config.BettercapAPIPort)
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var sessionData SessionData
	if err := json.NewDecoder(resp.Body).Decode(&sessionData); err != nil {
		return nil, err
	}

	return &sessionData, nil
}

func (b *Bettercap) GetEvents() ([]BettercapEvent, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	apiURL := fmt.Sprintf(BettercapEventsURL, b.config.BettercapAPIPort)
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var events []BettercapEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, err
	}

	return events, nil
}

func (b *Bettercap) randomizeMACBeforeStart() {
	log.Printf("[INIT] Attempting to randomize MAC address before starting bettercap")

	// Generate a random MAC address
	randomMAC := generateRandomMAC()

	// Commands to change MAC address using ip command (more modern than ifconfig)
	commands := [][]string{
		{"sudo", "ip", "link", "set", "dev", b.config.Interface, "down"},
		{"sudo", "ip", "link", "set", "dev", b.config.Interface, "address", randomMAC},
		{"sudo", "ip", "link", "set", "dev", b.config.Interface, "up"},
	}

	// Try using ip command first
	success := true
	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			success = false
			break
		}
	}

	if !success {
		// Fallback to ifconfig method
		log.Printf("[INFO] Trying ifconfig method for MAC change")
		commands = [][]string{
			{"sudo", "ifconfig", b.config.Interface, "down"},
			{"sudo", "ifconfig", b.config.Interface, "hw", "ether", randomMAC},
			{"sudo", "ifconfig", b.config.Interface, "up"},
		}

		for _, cmd := range commands {
			if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
				log.Printf("[INFO] MAC randomization not available (requires sudo or interface doesn't support it)")
				return
			}
		}
	}

	log.Printf("[INIT] MAC address changed to %s", randomMAC)
}

func generateRandomMAC() string {
	// Generate a random MAC with locally administered bit set
	mac := make([]byte, 6)
	nano := time.Now().UnixNano()

	// First octet: set locally administered bit (bit 1) and clear multicast bit (bit 0)
	mac[0] = byte((nano & 0xFC) | 0x02)

	// Generate remaining 5 octets
	for i := 1; i < 6; i++ {
		nano = time.Now().UnixNano()
		mac[i] = byte(nano >> (i * 8))
		time.Sleep(1 * time.Nanosecond) // Ensure different values
	}

	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

func (b *Bettercap) GetLatestGPSData() (*GPSData, error) {
	events, err := b.GetEvents()
	if err != nil {
		return nil, err
	}

	var latestGPS *GPSData
	var latestTime time.Time

	for _, event := range events {
		if event.Tag == "gps.new" {
			eventTime, err := time.Parse(time.RFC3339Nano, event.Time)
			if err != nil {
				continue
			}

			if latestGPS == nil || eventTime.After(latestTime) {
				var gpsData GPSData
				if err := json.Unmarshal(event.Data, &gpsData); err != nil {
					continue
				}
				latestGPS = &gpsData
				latestTime = eventTime
			}
		}
	}

	return latestGPS, nil
}
