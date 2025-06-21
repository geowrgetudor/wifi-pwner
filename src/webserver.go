package src

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type WebServer struct {
	db *Database
}

func NewWebServer(db *Database) *WebServer {
	return &WebServer{db: db}
}

func (w *WebServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", w.handleDashboard)

	log.Printf("[INIT] Web UI: http://localhost:%s", DefaultWebPort)
	go http.ListenAndServe(":"+DefaultWebPort, mux)
}

type DashboardData struct {
	Result      *PaginatedResult
	Search      string
	Encryption  string
	Channel     string
	Status      string
	Encryptions []string
	Channels    []string
	Statuses    []string
}

func (w *WebServer) handleDashboard(resp http.ResponseWriter, req *http.Request) {
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

	data := DashboardData{
		Result:      result,
		Search:      search,
		Encryption:  encryption,
		Channel:     channel,
		Status:      status,
		Encryptions: encryptions,
		Channels:    channels,
		Statuses:    statuses,
	}

	tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>WiFi Scanner Dashboard</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script>
        tailwind.config = {
            theme: {
                extend: {
                    colors: {
                        'status-captured': '#10b981',
                        'status-failed': '#ef4444',
                        'status-scanning': '#f59e0b',
                        'status-discovered': '#3b82f6',
                        'status-cracked': '#059669',
                        'status-crack-failed': '#dc2626'
                    }
                }
            }
        }
    </script>
    <style>
        .tooltip {
            position: relative;
            display: inline-block;
        }
        
        .tooltip .tooltiptext {
            visibility: hidden;
            width: 220px;
            background-color: #1f2937;
            color: #fff;
            text-align: center;
            border-radius: 6px;
            padding: 8px 12px;
            position: absolute;
            z-index: 1;
            bottom: 125%;
            left: 50%;
            margin-left: -110px;
            opacity: 0;
            transition: opacity 0.3s;
            font-size: 14px;
        }
        
        .tooltip .tooltiptext::after {
            content: "";
            position: absolute;
            top: 100%;
            left: 50%;
            margin-left: -5px;
            border-width: 5px;
            border-style: solid;
            border-color: #1f2937 transparent transparent transparent;
        }
        
        .tooltip:hover .tooltiptext {
            visibility: visible;
            opacity: 1;
        }
        
        .info-icon {
            display: inline-block;
            width: 16px;
            height: 16px;
            margin-left: 4px;
            color: #f59e0b;
            cursor: help;
        }
    </style>
    <script>
        function copyToClipboard(text) {
            const button = event.target;
            const originalText = button.innerHTML;
            
            // Try modern clipboard API first
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text).then(function() {
                    showCopySuccess(button, originalText);
                }, function(err) {
                    console.error('Clipboard API failed: ', err);
                    fallbackCopyTextToClipboard(text, button, originalText);
                });
            } else {
                // Fallback for older browsers or HTTP contexts
                fallbackCopyTextToClipboard(text, button, originalText);
            }
        }
        
        function fallbackCopyTextToClipboard(text, button, originalText) {
            const textArea = document.createElement("textarea");
            textArea.value = text;
            
            // Avoid scrolling to bottom
            textArea.style.position = "fixed";
            textArea.style.top = "0";
            textArea.style.left = "0";
            textArea.style.width = "2em";
            textArea.style.height = "2em";
            textArea.style.padding = "0";
            textArea.style.border = "none";
            textArea.style.outline = "none";
            textArea.style.boxShadow = "none";
            textArea.style.background = "transparent";
            
            document.body.appendChild(textArea);
            textArea.focus();
            textArea.select();
            
            try {
                const successful = document.execCommand('copy');
                if (successful) {
                    showCopySuccess(button, originalText);
                } else {
                    showCopyError(button, originalText);
                }
            } catch (err) {
                console.error('Fallback copy failed: ', err);
                showCopyError(button, originalText);
            }
            
            document.body.removeChild(textArea);
        }
        
        function showCopySuccess(button, originalText) {
            button.innerHTML = '✓';
            setTimeout(() => {
                button.innerHTML = originalText;
            }, 2000);
        }
        
        function showCopyError(button, originalText) {
            button.innerHTML = '✗ Error';
            button.classList.add('bg-red-500');
            setTimeout(() => {
                button.innerHTML = originalText;
                button.classList.remove('bg-red-500');
            }, 2000);
        }
    </script>
</head>
<body class="bg-gray-50 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <div class="bg-white rounded-lg shadow-lg">
            <div class="px-6 py-4 border-b border-gray-200">
                <h1 class="text-3xl font-bold text-gray-900">WiFi Scanner Dashboard</h1>
                <p class="text-sm text-gray-600 mt-1">
                    Showing {{.Result.TotalCount}} total targets
                    {{if gt .Result.TotalPages 1}}
                        (Page {{.Result.Page}} of {{.Result.TotalPages}})
                    {{end}}
                </p>
            </div>

            <!-- Search and Filters -->
            <div class="px-6 py-4 bg-gray-50 border-b border-gray-200">
                <form method="GET" class="space-y-4">
                    <!-- Search Bar -->
                    <div class="flex space-x-4">
                        <div class="flex-1">
                            <input type="text" name="search" value="{{.Search}}" 
                                   placeholder="Search by ESSID or BSSID..." 
                                   class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
                        </div>
                        <button type="submit" class="px-6 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500">
                            Search
                        </button>
                    </div>

                    <!-- Filters -->
                    <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 mb-1">Encryption</label>
                            <select name="encryption" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
                                <option value="">All Encryptions</option>
                                {{range .Encryptions}}
                                <option value="{{.}}"{{if eq $.Encryption .}} selected{{end}}>{{.}}</option>
                                {{end}}
                            </select>
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 mb-1">Channel</label>
                            <select name="channel" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
                                <option value="">All Channels</option>
                                {{range .Channels}}
                                <option value="{{.}}"{{if eq $.Channel .}} selected{{end}}>{{.}}</option>
                                {{end}}
                            </select>
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 mb-1">Status</label>
                            <select name="status" class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
                                <option value="">All Statuses</option>
                                {{range .Statuses}}
                                <option value="{{.}}"{{if eq $.Status .}} selected{{end}}>{{.}}</option>
                                {{end}}
                            </select>
                        </div>
                        <div class="flex items-end">
                            <a href="/" class="w-full px-4 py-2 bg-gray-500 text-white text-center rounded-md hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-gray-500">
                                Clear Filters
                            </a>
                        </div>
                    </div>
                </form>
            </div>

            <!-- Table -->
            <div class="overflow-x-auto">
                <table class="min-w-full divide-y divide-gray-200">
                    <thead class="bg-gray-50">
                        <tr>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">BSSID</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">ESSID</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Signal</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Channel</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Encryption</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Password</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Last Scan</th>
                        </tr>
                    </thead>
                    <tbody class="bg-white divide-y divide-gray-200">
                        {{range .Result.Targets}}
                        <tr class="hover:bg-gray-50">
                            <td class="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-900">{{.bssid}}</td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{.essid}}</td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                                <div class="flex items-center">
                                    <span>{{.signal}} dBm</span>
                                    {{if and (eq .status "Discovered") (lt .signal -70)}}
                                    <div class="tooltip">
                                        <svg class="info-icon" fill="none" stroke="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                        </svg>
                                        <span class="tooltiptext">Signal too weak! Move closer to the target. Min signal required is -70.</span>
                                    </div>
                                    {{end}}
                                </div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{.channel}}</td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{.encryption}}</td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm">
                                <div class="flex items-center space-x-2">
                                    <span class="inline-flex px-2 py-1 text-xs font-semibold rounded-full
                                        {{if eq .status "Handshake Captured"}}bg-green-100 text-green-800
                                        {{else if eq .status "Cracked"}}bg-emerald-100 text-emerald-800
                                        {{else if eq .status "Failed to crack"}}bg-orange-100 text-orange-800
                                        {{else if eq .status "Failed to Scan"}}bg-red-100 text-red-800
                                        {{else if eq .status "Failed to Cap Handshake"}}bg-red-100 text-red-800
                                        {{else if eq .status "Scanning"}}bg-yellow-100 text-yellow-800
                                        {{else}}bg-blue-100 text-blue-800{{end}}">
                                        {{.status}}
                                    </span>
                                    {{if and .handshakePath (ne .handshakePath "") (or (eq .status "Handshake Captured") (eq .status "Cracked") (eq .status "Failed to crack"))}}
                                    <div class="tooltip">
                                        <button onclick="copyToClipboard('{{.handshakePath}}')" 
                                                class="p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded transition-colors duration-150">
                                            📋
                                        </button>
                                        <span class="tooltiptext">Copy handshake path</span>
                                    </div>
                                    {{end}}
                                </div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm">
                                {{if .crackedPassword}}
                                    <div class="flex items-center space-x-2">
                                        <span class="font-mono text-green-600">{{.crackedPassword}}</span>
                                        <div class="tooltip">
                                            <button onclick="copyToClipboard('{{.crackedPassword}}')" 
                                                    class="p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded transition-colors duration-150">
                                                📋
                                            </button>
                                            <span class="tooltiptext">Copy password</span>
                                        </div>
                                    </div>
                                {{else}}
                                    <span class="text-gray-400">-</span>
                                {{end}}
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{{.lastScan}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>

            <!-- Pagination -->
            {{if gt .Result.TotalPages 1}}
            <div class="px-6 py-4 bg-gray-50 border-t border-gray-200">
                <div class="flex items-center justify-between">
                    <div class="text-sm text-gray-700">
                        Showing {{if .Result.Targets}}{{add (mul (sub .Result.Page 1) .Result.PerPage) 1}} to {{min (mul .Result.Page .Result.PerPage) .Result.TotalCount}}{{else}}0{{end}} of {{.Result.TotalCount}} results
                    </div>
                    <div class="flex space-x-2">
                        {{if gt .Result.Page 1}}
                        <a href="?page={{sub .Result.Page 1}}{{if .Search}}&search={{.Search}}{{end}}{{if .Encryption}}&encryption={{.Encryption}}{{end}}{{if .Channel}}&channel={{.Channel}}{{end}}{{if .Status}}&status={{.Status}}{{end}}" 
                           class="px-3 py-1 bg-white border border-gray-300 rounded-md text-sm text-gray-700 hover:bg-gray-50">
                            Previous
                        </a>
                        {{end}}
                        
                        {{range $i := .Result.TotalPages | pageRange .Result.Page}}
                        {{if eq $i $.Result.Page}}
                        <span class="px-3 py-1 bg-blue-600 text-white rounded-md text-sm">{{$i}}</span>
                        {{else}}
                        <a href="?page={{$i}}{{if $.Search}}&search={{$.Search}}{{end}}{{if $.Encryption}}&encryption={{$.Encryption}}{{end}}{{if $.Channel}}&channel={{$.Channel}}{{end}}{{if $.Status}}&status={{$.Status}}{{end}}" 
                           class="px-3 py-1 bg-white border border-gray-300 rounded-md text-sm text-gray-700 hover:bg-gray-50">
                            {{$i}}
                        </a>
                        {{end}}
                        {{end}}
                        
                        {{if lt .Result.Page .Result.TotalPages}}
                        <a href="?page={{add .Result.Page 1}}{{if .Search}}&search={{.Search}}{{end}}{{if .Encryption}}&encryption={{.Encryption}}{{end}}{{if .Channel}}&channel={{.Channel}}{{end}}{{if .Status}}&status={{.Status}}{{end}}" 
                           class="px-3 py-1 bg-white border border-gray-300 rounded-md text-sm text-gray-700 hover:bg-gray-50">
                            Next
                        </a>
                        {{end}}
                    </div>
                </div>
            </div>
            {{end}}
        </div>
    </div>
</body>
</html>
`

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
			end := currentPage + 2
			if end > totalPages {
				end = totalPages
			}

			var pages []int
			for i := start; i <= end; i++ {
				pages = append(pages, i)
			}
			return pages
		},
	}

	t, err := template.New("dashboard").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "text/html")
	t.Execute(resp, data)
}
