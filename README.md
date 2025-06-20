# WiFi Pwner

A fast, mobile-optimized WiFi handshake capture tool built on top of Bettercap.

## Features

- **Mobile-First Design**: Optimized for Raspberry Pi on the move, but works on any Linux distro
- **Smart Target Selection**: Automatically targets the strongest AP with clients
- **Fast Capture**: ~20 seconds per attempt
- **Web Dashboard**: Real-time monitoring on port 8080 (optional)
- **Auto-Retry**: Failed captures retry after 5 minutes (if in range)
- **MAC Address Randomization**: Changes MAC address before each session for anonymity
- **Whitelist Support**: Skip specific BSSIDs
- **Clean Storage**: Only successful captures are saved

## Hardware Requirements

- Raspberry Pi (3 / 4 / 400 / 5) or any Laptop/PC with a Linux distro
- External WiFi adapter with monitor mode & package injection support

## Software Requirements

- Ubuntu Server 24 or similar Linux distribution
- Go 1.21+
- [Bettercap](https://github.com/bettercap/bettercap)
- [Aircrack-ng](https://github.com/aircrack-ng/aircrack-ng)
- SQLite3

## Installation

The installation guide is for Debian based Linux distros, but it should be similar for other distros.

### Option 1: Download Pre-built Binary (Recommended)

1. Download the appropriate binary for your system from the [releases page](https://github.com/georgegebbett/wifi-pwner/releases)
2. Extract the archive:

```bash
tar -xzf wifi-pwner_linux_amd64.tar.gz
```

3. Make the binary executable:

```bash
chmod +x wifi-pwner
```

4. Install system dependencies:

```bash
sudo apt update
sudo apt install -y bettercap aircrack-ng sqlite3
```

### Option 2: Build from Source

1. Clone the repository:

```bash
git clone https://github.com/georgegebbett/wifi-pwner.git
cd wifi-pwner
```

2. Install dependencies:

```bash
# Install Bettercap
sudo apt update
sudo apt install -y bettercap aircrack-ng sqlite3 build-essential

# Install Go dependencies
go mod download
```

3. Build the project:

```bash
chmod +x build.sh
./build.sh
```

The build script will:

- Compile the project to `dist/wifi-pwner`
- Copy the whitelist template if it doesn't exist
- Optionally set up a systemd service for auto-start

## Usage

### Basic Usage

```bash
sudo ./dist/wifi-pwner --interface wlan0
```

### Command Line Options

- `--interface` (required): WiFi interface to use
- `--mode`: Frequency band - `2.4` or `5` (default: `2.4`)
- `--clean`: Clean database and previous captures before starting
- `--b-api-port`: Bettercap API port (default: `8081`)
- `--b-expose`: Expose Bettercap API on 0.0.0.0 instead of 127.0.0.1
- `--webui`: Enable custom web UI on port 8080 (default: `true`)

### Examples

```bash
# Scan only 5GHz networks
sudo ./dist/wifi-pwner --interface wlan0 --mode 5

# Clean previous data and start fresh
sudo ./dist/wifi-pwner --interface wlan0 --clean

# Use custom Bettercap API port
sudo ./dist/wifi-pwner --interface wlan0 --b-api-port 9001

# Expose Bettercap API for remote access
sudo ./dist/wifi-pwner --interface wlan0 --b-expose

# Disable web UI
sudo ./dist/wifi-pwner --interface wlan0 --webui=false
```

## Web Interface

When enabled (default), access the web dashboard at `http://localhost:8080` to view:

- All discovered APs
- Capture status (Discovered, Scanning, Captured, Failed)
- Signal strength
- Handshake file paths

### Runtime Files

All runtime files are created in the directory where `wifi-pwner` is executed:

```
./dist/
├── wifi-pwner              # Compiled binary
├── scanned.db              # SQLite database
├── whitelist.txt           # Optional BSSID whitelist
└── scanned/                # Captured handshakes
    ├── AABBCCDDEEFF/       # BSSID
    │   └── handshake.pcap
    └── 112233445566/
        └── handshake.pcap
```

## Whitelist Format

The build script creates a `whitelist.txt` file from the `whitelist.txt.example` template. Edit this file to skip specific BSSIDs:

```
# WiFi Pwner Whitelist
# Add BSSIDs (MAC addresses) to skip during scanning
# One BSSID per line, format: XX:XX:XX:XX:XX:XX
# Lines starting with # are comments

# Example entries:
# 00:11:22:33:44:55
# AA:BB:CC:DD:EE:FF
```

The tool automatically looks for `whitelist.txt` in the directory where `wifi-pwner` is executed. If no whitelist file exists, all discovered networks (meeting signal requirements) will be targeted.

## Systemd Service

The build script can optionally set up a systemd service for automatic startup:

```bash
# Check service status
sudo systemctl status wifi-pwner.service

# Start the service
sudo systemctl start wifi-pwner.service

# Stop the service
sudo systemctl stop wifi-pwner.service

# View logs
sudo journalctl -u wifi-pwner.service -f
```

The service runs from the `dist/` directory and includes a 60-second delay on startup to ensure network interfaces are ready.

## Troubleshooting

### Interface not found

Ensure your WiFi adapter is connected and supports monitor mode:

```bash
iwconfig
ip link show | grep wl
```

### Bettercap fails to start

Check if another instance is running:

```bash
sudo killall bettercap
```

Test Bettercap manually:

```bash
sudo bettercap -iface wlan0 -eval "wifi.recon on"
```

### No handshakes captured

- Ensure there are active clients on the target AP
- Try moving closer (targets with signal < -70 dBm are ignored)
- Check if the AP uses WPA/WPA2 (not WPA3)
- Verify MAC randomization is working (check logs for [INIT] MAC address changed)

### Permission issues

```bash
# Run as root
sudo ./dist/wifi-pwner --interface wlan0

# Or set capabilities for Bettercap
sudo setcap cap_net_raw,cap_net_admin=eip $(which bettercap)
```

### Build errors

```bash
# Update dependencies
go mod tidy

# Verbose build
go build -v -x
```

## Security Notice

This tool is for educational and authorized security testing purposes only. Always ensure you have explicit permission before testing on any network. Unauthorized access to computer networks is illegal and unethical.

## License

This project is provided as-is for educational purposes. Use responsibly and legally.
