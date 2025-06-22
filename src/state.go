package src

import "sync"

// Global state for runtime control
var (
	GlobalScanner      *Scanner
	GlobalCracker      *Cracker
	ScanningEnabled    = true
	CrackingEnabled    = false
	stateMutex         sync.Mutex
)

func SetScanningEnabled(enabled bool) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	ScanningEnabled = enabled
}

func GetScanningEnabled() bool {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	return ScanningEnabled
}

func SetCrackingEnabled(enabled bool) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	CrackingEnabled = enabled
}

func GetCrackingEnabled() bool {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	return CrackingEnabled
}