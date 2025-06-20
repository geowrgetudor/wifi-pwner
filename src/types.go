package src

import "time"

type Target struct {
	BSSID      string
	ESSID      string
	Channel    string
	Signal     int
	Frequency  int
	Encryption string
}

type Status string

const (
	StatusDiscovered        Status = "Discovered"
	StatusScanning          Status = "Scanning"
	StatusFailedToScan      Status = "Failed to Scan"
	StatusFailedToCap       Status = "Failed to Cap Handshake"
	StatusHandshakeCaptured Status = "Handshake Captured"
)

type Config struct {
	Interface          string
	Mode               string
	Clean              bool
	WhitelistFile      string
	BettercapAPIPort   string
	BettercapApiExpose bool
	WebUI              bool
	WorkingDir         string
}

type BettercapCommand struct {
	Cmd string `json:"cmd"`
}

type WiFiAP struct {
	MAC        string `json:"mac"`
	Hostname   string `json:"hostname"`
	Frequency  int    `json:"frequency"`
	RSSI       int    `json:"rssi"`
	Channel    int    `json:"channel"`
	Encryption string `json:"encryption"`
}

type SessionData struct {
	WiFi struct {
		APs []WiFiAP `json:"aps"`
	} `json:"wifi"`
}

const (
	DefaultWebPort  = "8080"
	BettercapAPIURL = "http://127.0.0.1:%s/api/session"
	RetryDelay      = 5 * time.Minute
)
