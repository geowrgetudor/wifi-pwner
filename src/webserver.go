package src

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed src/templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

type WebServer struct {
	db        *Database
	templates *template.Template
}

func NewWebServer(db *Database) *WebServer {
	ws := &WebServer{db: db}
	ws.loadTemplates()
	return ws
}

func (w *WebServer) Start() {
	mux := http.NewServeMux()

	// Static file serving
	staticContent, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	// Page handlers
	mux.HandleFunc("/", w.handleHomepage)
	mux.HandleFunc("/aps", w.handleAPs)
	mux.HandleFunc("/probes", w.handleProbes)

	// API handlers
	mux.HandleFunc("/api/toggle-scanning", w.handleToggleScanning)
	mux.HandleFunc("/api/toggle-cracking", w.handleToggleCracking)
	mux.HandleFunc("/api/status", w.handleStatus)
	mux.HandleFunc("/api/download-handshake", w.handleDownloadHandshake)
	mux.HandleFunc("/api/delete-target", w.handleDeleteTarget)

	log.Printf("[INIT] Web UI: http://localhost:%s", DefaultWebPort)
	go http.ListenAndServe(":"+DefaultWebPort, mux)
}

type ApsData struct {
	Result      *PaginatedResult
	Search      string
	Encryption  string
	Channel     string
	Status      string
	Encryptions []string
	Channels    []string
	Statuses    []string
}

type ProbePageData struct {
	Result *PaginatedResult
	Search string
}

type TemplateData struct {
	Title         string
	ApsData       *ApsData
	ProbePageData *ProbePageData
}

func (w *WebServer) loadTemplates() {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"pageRange": func(totalPages, currentPage int) []int {
			start := currentPage - 2
			if start < 1 {
				start = 1
			}
			end := start + 4
			if end > totalPages {
				end = totalPages
				start = end - 4
				if start < 1 {
					start = 1
				}
			}

			var pages []int
			for i := start; i <= end; i++ {
				pages = append(pages, i)
			}
			return pages
		},
	}

	var err error
	w.templates, err = template.New("").Funcs(funcMap).ParseFS(templateFS, "src/templates/*.html")
	if err != nil {
		log.Fatalf("Error loading embedded templates: %v", err)
	}
}


func (w *WebServer) handleAPs(resp http.ResponseWriter, req *http.Request) {
	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	search := strings.TrimSpace(req.URL.Query().Get("search"))
	encryption := req.URL.Query().Get("encryption")
	channel := req.URL.Query().Get("channel")
	status := req.URL.Query().Get("status")

	params := FilterParams{
		Search:     search,
		Encryption: encryption,
		Channel:    channel,
		Status:     status,
		Page:       page,
		PerPage:    20,
	}

	result, err := w.db.GetPaginatedTargets(params)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	encryptions, _ := w.db.GetUniqueEncryptions()
	channels, _ := w.db.GetUniqueChannels()
	statuses := GetAllStatuses()

	data := ApsData{
		Result:      result,
		Search:      search,
		Encryption:  encryption,
		Channel:     channel,
		Status:      status,
		Encryptions: encryptions,
		Channels:    channels,
		Statuses:    statuses,
	}

	templateData := TemplateData{
		Title:   "APs",
		ApsData: &data,
	}

	resp.Header().Set("Content-Type", "text/html")
	err = w.templates.ExecuteTemplate(resp, "base.html", templateData)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (w *WebServer) handleToggleScanning(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enabled := !GetScanningEnabled()
	SetScanningEnabled(enabled)

	if enabled && GlobalScanner != nil {
		if err := GlobalScanner.StartScanning(); err != nil {
			log.Printf("[ERROR] Failed to start scanning: %v", err)
		}
	} else if !enabled && GlobalScanner != nil {
		GlobalScanner.StopScanning()
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.Write([]byte(`{"scanning": ` + strconv.FormatBool(enabled) + `}`))
}

func (w *WebServer) handleToggleCracking(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if GlobalCracker == nil {
		http.Error(resp, "Cracker not initialized", http.StatusBadRequest)
		return
	}

	enabled := !GetCrackingEnabled()
	SetCrackingEnabled(enabled)

	resp.Header().Set("Content-Type", "application/json")
	resp.Write([]byte(`{"cracking": ` + strconv.FormatBool(enabled) + `}`))
}

func (w *WebServer) handleStatus(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.Write([]byte(`{"scanning": ` + strconv.FormatBool(GetScanningEnabled()) +
		`, "cracking": ` + strconv.FormatBool(GetCrackingEnabled()) +
		`, "crackerAvailable": ` + strconv.FormatBool(GlobalCracker != nil) + `}`))
}

func (w *WebServer) handleDownloadHandshake(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bssid := req.URL.Query().Get("bssid")
	if bssid == "" {
		http.Error(resp, "BSSID parameter required", http.StatusBadRequest)
		return
	}

	target := w.db.GetTarget(bssid)
	if target == nil {
		http.Error(resp, "Target not found", http.StatusNotFound)
		return
	}

	handshakePath, ok := target["handshakePath"].(string)
	if !ok || handshakePath == "" {
		http.Error(resp, "No handshake available", http.StatusNotFound)
		return
	}

	if _, err := os.Stat(handshakePath); os.IsNotExist(err) {
		http.Error(resp, "Handshake file not found", http.StatusNotFound)
		return
	}

	filename := filepath.Base(handshakePath)

	resp.Header().Set("Content-Disposition", "attachment; filename="+filename)
	resp.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")

	http.ServeFile(resp, req, handshakePath)
}

func (w *WebServer) handleDeleteTarget(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		BSSID string `json:"bssid"`
	}

	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		http.Error(resp, "Invalid request body", http.StatusBadRequest)
		return
	}

	if data.BSSID == "" {
		http.Error(resp, "BSSID parameter required", http.StatusBadRequest)
		return
	}

	// Get target to check for handshake file
	target := w.db.GetTarget(data.BSSID)
	if target != nil {
		handshakePath, ok := target["handshakePath"].(string)
		if ok && handshakePath != "" {
			// Delete handshake file if it exists
			if _, err := os.Stat(handshakePath); err == nil {
				os.Remove(handshakePath)
			}
		}
	}

	// Delete from database
	if err := w.db.DeleteTarget(data.BSSID); err != nil {
		http.Error(resp, "Failed to delete target", http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.Write([]byte(`{"success": true}`))
}

func (w *WebServer) handleHomepage(resp http.ResponseWriter, req *http.Request) {
	templateData := TemplateData{
		Title: "Dashboard",
	}

	resp.Header().Set("Content-Type", "text/html")
	err := w.templates.ExecuteTemplate(resp, "base.html", templateData)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (w *WebServer) handleProbes(resp http.ResponseWriter, req *http.Request) {
	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	search := strings.TrimSpace(req.URL.Query().Get("search"))

	params := FilterParams{
		Search:  search,
		Page:    page,
		PerPage: 20,
	}

	result, err := w.db.GetPaginatedProbes(params)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	data := ProbePageData{
		Result: result,
		Search: search,
	}

	templateData := TemplateData{
		Title:         "Probes",
		ProbePageData: &data,
	}

	resp.Header().Set("Content-Type", "text/html")
	err = w.templates.ExecuteTemplate(resp, "base.html", templateData)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
}
