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
	StatusFailedToCap       Status = "Failed to Cap Handshake"
	StatusHandshakeCaptured Status = "Handshake Captured"
	StatusCracked           Status = "Cracked"
	StatusFailedToCrack     Status = "Failed to crack"
)

// GetAllStatuses returns all possible status values
func GetAllStatuses() []string {
	return []string{
		string(StatusDiscovered),
		string(StatusScanning),
		string(StatusFailedToCap),
		string(StatusHandshakeCaptured),
		string(StatusCracked),
		string(StatusFailedToCrack),
	}
}

type Config struct {
	Interface          string
	Mode               string
	Clean              bool
	WhitelistFile      string
	BettercapAPIPort   string
	BettercapApiExpose bool
	WebUI              bool
	WorkingDir         string
	AutoCrack          bool
	WordlistPath       string
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

type ProbeData struct {
	ESSID  string `json:"essid"`
	MAC    string `json:"mac"`   
	RSSI   int    `json:"rssi"`
	Vendor string `json:"vendor"`
}

type BettercapEvent struct {
	Tag  string      `json:"tag"`
	Time string      `json:"time"`
	Data ProbeData   `json:"data"`
}

const (
	DefaultWebPort        = "8080"
	BettercapSessionURL   = "http://127.0.0.1:%s/api/session"
	BettercapEventsURL    = "http://127.0.0.1:%s/api/events"
	RetryDelay            = 5 * time.Minute
)
