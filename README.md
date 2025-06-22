<div align="center">
  <img src="https://raw.githubusercontent.com/geowrgetudor/wifi-pwner/refs/heads/main/media/cover.png" alt="WiFi Pwner Logo" />
</div>

<br />

<div align=center>
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux-blue.svg" />
    <img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/geowrgetudor/wifi-pwner" />
    <img alt="License" src="https://img.shields.io/github/license/geowrgetudor/wifi-pwner" />
    <img alt="Release" src="https://img.shields.io/github/v/release/geowrgetudor/wifi-pwner">
    <img alt="Checks run" src="https://img.shields.io/github/check-runs/geowrgetudor/wifi-pwner/main" />
    <img alt="Actions Workflow Status" src="https://img.shields.io/github/actions/workflow/status/geowrgetudor/wifi-pwner/.github%2Fworkflows%2Fci.yml?branch=main" />
    <img alt="Last commit" src="https://img.shields.io/github/last-commit/geowrgetudor/wifi-pwner/main" />
</div>

# WiFi Pwner

A fast, mobile-optimized (on the go - similar to pwnagotchi) WiFi handshake capture & cracking tool built on top of Bettercap & Aircrack-ng. Run WiFi Pwner on your Pi (as service) and start pwning without breaking a sweat.

<br />

<div align="center">
  <img src="https://raw.githubusercontent.com/geowrgetudor/wifi-pwner/refs/heads/main/media/demo.gif" alt="WiFi Pwner Demo" />
</div>

<br />

## Table of Contents

- [Features](#features)
- [Hardware Requirements](#hardware-requirements)
- [Software Requirements](#software-requirements)
- [Installation](#installation)
  - [Option 1: Download Pre-built Binary (Recommended)](#option-1-download-pre-built-binary-recommended)
  - [Option 2: Build from Source](#option-2-build-from-source)
- [Usage](#usage)
  - [Basic Usage](#basic-usage)
  - [Command Line Options](#command-line-options)
  - [Examples](#examples)
- [Automatic Password Cracking](#automatic-password-cracking)
  - [Features](#features-1)
  - [Usage](#usage-1)
  - [Status Meanings](#status-meanings)
  - [Wordlist Management](#wordlist-management)
- [Web Interface](#web-interface)
  - [Runtime Files](#runtime-files)
- [Whitelist Format](#whitelist-format)
- [Systemd Service](#systemd-service)
- [Troubleshooting](#troubleshooting)
- [Security Notice](#security-notice)
- [License](#license)

## Features

- **Mobile-First Design**: Optimized for Raspberry Pi on the move, but works on any Linux distro
- **Smart Target Selection**: Automatically targets the strongest AP with clients
- **Fast Capture**: ~20 seconds per attempt
- **Web Dashboard**: Real-time monitoring on port 8080 (optional)
- **Auto-Retry**: Failed captures retry after 5 minutes (if in range)
- **MAC Address Randomization**: Changes MAC address before each session for anonymity
- **Whitelist Support**: Skip specific BSSIDs
- **Clean Storage**: Only successful captures are saved
- **Automatic Password Cracking**: Built-in WPA2 handshake cracking using aircrack-ng
- **Wordlist Support**: Download and use popular wordlists like rockyou.txt

Upcoming:

- GPS support
- Interactive map

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
- Optionally download the rockyou.txt wordlist for password cracking
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
- `--autocrack`: Enable automatic WPA2 handshake cracking
- `--wordlist`: Path to wordlist file for cracking (required if --autocrack is used)

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

# Enable automatic cracking with rockyou.txt wordlist
sudo ./dist/wifi-pwner --interface wlan0 --autocrack --wordlist ./dist/rockyou.txt

# Enable automatic cracking with custom wordlist
sudo ./dist/wifi-pwner --interface wlan0 --autocrack --wordlist /path/to/custom/wordlist.txt
```

## Automatic Password Cracking

WiFi Pwner includes built-in automatic WPA2 handshake cracking functionality using aircrack-ng:

### Features

- **Automatic Processing**: Captured handshakes are automatically queued for cracking
- **Background Operation**: Cracking runs in parallel with scanning and capturing
- **Database Integration**: Cracked passwords are saved to the database
- **Status Tracking**: Track cracking attempts and results through the web interface
- **Wordlist Support**: Use popular wordlists like rockyou.txt

### Usage

1. **Enable autocrack mode** when starting wifi-pwner:

   ```bash
   sudo ./dist/wifi-pwner --interface wlan0 --autocrack --wordlist ./dist/rockyou.txt
   ```

2. **Download wordlist** during build (recommended):
   The build script will offer to download rockyou.txt automatically.

3. **Monitor progress** through the web interface at `http://localhost:8080`

### Status Meanings

- **Handshake Captured**: Ready for cracking
- **Cracked**: Password successfully recovered
- **Failed to crack**: Password not found in wordlist

### Wordlist Management

The build script can automatically download the popular rockyou.txt wordlist:

- **Size**: ~130MB compressed, ~540MB uncompressed
- **Passwords**: 14+ million common passwords
- **Location**: Saved to `dist/rockyou.txt`
- **Manual download**: https://github.com/brannondorsey/naive-hashcat/releases/download/data/rockyou.txt

You can also use custom wordlists by specifying the path with `--wordlist`.

## Web Interface

When enabled (default), access the web dashboard at `http://localhost:8080` to view:

- All discovered APs
- Capture status (Discovered, Scanning, Captured, Failed, Cracked, Failed to crack)
- Signal strength
- Cracked passwords with copy-to-clipboard functionality
- Handshake file paths with copy-to-clipboard functionality

### Runtime Files

All runtime files are created in the directory where `wifi-pwner` is executed:

```
./dist/
├── wifi-pwner              # Compiled binary
├── scanned.db              # SQLite database (includes cracked passwords)
├── whitelist.txt           # Optional BSSID whitelist
├── rockyou.txt             # Downloaded wordlist (optional)
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
