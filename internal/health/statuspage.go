package health

import (
	"encoding/json"
	"html/template"
	"net/http"
)

const statusPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Fermi API Gateway - Status</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif;
            background: #fafafa;
            min-height: 100vh;
            color: #1a1a1a;
            padding: 48px 24px;
            line-height: 1.5;
        }

        .container {
            max-width: 720px;
            margin: 0 auto;
        }

        .header {
            margin-bottom: 48px;
        }

        .header h1 {
            font-size: 1.75rem;
            font-weight: 600;
            color: #111;
            margin-bottom: 4px;
        }

        .header .subtitle {
            color: #666;
            font-size: 0.9rem;
        }

        .overall-status {
            background: #fff;
            border-radius: 12px;
            padding: 32px;
            margin-bottom: 32px;
            border: 1px solid #e5e5e5;
        }

        .status-indicator {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .status-dot {
            width: 12px;
            height: 12px;
            border-radius: 50%;
        }

        .status-dot.healthy {
            background: #22c55e;
        }

        .status-dot.degraded {
            background: #f59e0b;
        }

        .status-text {
            font-size: 1.25rem;
            font-weight: 600;
            color: #111;
        }

        .meta {
            display: flex;
            gap: 24px;
            margin-top: 16px;
            padding-top: 16px;
            border-top: 1px solid #f0f0f0;
        }

        .meta-item {
            font-size: 0.8rem;
            color: #888;
        }

        .section {
            margin-bottom: 32px;
        }

        .section-title {
            font-size: 0.75rem;
            font-weight: 600;
            color: #888;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 12px;
        }

        .services {
            background: #fff;
            border-radius: 12px;
            border: 1px solid #e5e5e5;
            overflow: hidden;
        }

        .service {
            padding: 20px 24px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            border-bottom: 1px solid #f0f0f0;
        }

        .service:last-child {
            border-bottom: none;
        }

        .service-info {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .service-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
        }

        .service-dot.healthy {
            background: #22c55e;
        }

        .service-dot.unhealthy {
            background: #ef4444;
        }

        .service-name {
            font-weight: 500;
            color: #111;
        }

        .service-latency {
            font-size: 0.85rem;
            color: #888;
            font-variant-numeric: tabular-nums;
        }

        .service-error {
            font-size: 0.8rem;
            color: #ef4444;
            margin-top: 4px;
        }

        .stats {
            background: #fff;
            border-radius: 12px;
            border: 1px solid #e5e5e5;
            padding: 24px;
        }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(3, 1fr);
            gap: 24px;
        }

        @media (max-width: 600px) {
            .stats-grid {
                grid-template-columns: repeat(2, 1fr);
            }
        }

        .stat {
            text-align: center;
        }

        .stat-value {
            font-size: 1.5rem;
            font-weight: 600;
            color: #111;
            font-variant-numeric: tabular-nums;
        }

        .stat-label {
            font-size: 0.75rem;
            color: #888;
            margin-top: 2px;
        }

        .refresh-bar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 32px;
            padding-top: 24px;
            border-top: 1px solid #e5e5e5;
        }

        .auto-refresh {
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 0.85rem;
            color: #666;
        }

        .auto-refresh input[type="checkbox"] {
            width: 16px;
            height: 16px;
            accent-color: #111;
        }

        .refresh-btn {
            padding: 8px 16px;
            border-radius: 6px;
            background: #111;
            color: #fff;
            border: none;
            font-size: 0.85rem;
            font-weight: 500;
            cursor: pointer;
            transition: background 0.15s;
        }

        .refresh-btn:hover {
            background: #333;
        }

        .footer {
            text-align: center;
            margin-top: 48px;
            color: #999;
            font-size: 0.8rem;
        }

        .footer a {
            color: #666;
            text-decoration: none;
        }

        .footer a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Fermi API Gateway</h1>
            <p class="subtitle">System Status</p>
        </div>

        <div class="overall-status">
            <div class="status-indicator">
                <span class="status-dot {{if eq .Status "healthy"}}healthy{{else}}degraded{{end}}"></span>
                <span class="status-text">{{if eq .Status "healthy"}}All Systems Operational{{else}}Degraded Performance{{end}}</span>
            </div>
            <div class="meta">
                <span class="meta-item">Updated {{.Timestamp.Format "Jan 2, 15:04:05"}}</span>
                <span class="meta-item">Uptime {{.Gateway.Uptime}}</span>
            </div>
        </div>

        <div class="section">
            <h2 class="section-title">Services</h2>
            <div class="services">
                <div class="service">
                    <div class="service-info">
                        <span class="service-dot {{.Services.Rollup.Status}}"></span>
                        <div>
                            <div class="service-name">Rollup</div>
                            {{if .Services.Rollup.Error}}<div class="service-error">{{.Services.Rollup.Error}}</div>{{end}}
                        </div>
                    </div>
                    <span class="service-latency">{{printf "%.0f" .Services.Rollup.LatencyMs}} ms</span>
                </div>
                <div class="service">
                    <div class="service-info">
                        <span class="service-dot {{.Services.Continuum.Status}}"></span>
                        <div>
                            <div class="service-name">Continuum</div>
                            {{if .Services.Continuum.Error}}<div class="service-error">{{.Services.Continuum.Error}}</div>{{end}}
                        </div>
                    </div>
                    <span class="service-latency">{{printf "%.0f" .Services.Continuum.LatencyMs}} ms</span>
                </div>
                <div class="service">
                    <div class="service-info">
                        <span class="service-dot {{.Services.TimescaleDB.Status}}"></span>
                        <div>
                            <div class="service-name">TimescaleDB</div>
                            {{if .Services.TimescaleDB.Error}}<div class="service-error">{{.Services.TimescaleDB.Error}}</div>{{end}}
                        </div>
                    </div>
                    <span class="service-latency">{{printf "%.0f" .Services.TimescaleDB.LatencyMs}} ms</span>
                </div>
            </div>
        </div>

        <div class="section">
            <h2 class="section-title">Gateway</h2>
            <div class="stats">
                <div class="stats-grid">
                    <div class="stat">
                        <div class="stat-value">{{.Gateway.Goroutines}}</div>
                        <div class="stat-label">Goroutines</div>
                    </div>
                    <div class="stat">
                        <div class="stat-value">{{printf "%.1f" .Gateway.MemAllocMB}}</div>
                        <div class="stat-label">Memory (MB)</div>
                    </div>
                    <div class="stat">
                        <div class="stat-value">{{.Gateway.NumGC}}</div>
                        <div class="stat-label">GC Cycles</div>
                    </div>
                </div>
            </div>
        </div>

        <div class="refresh-bar">
            <label class="auto-refresh">
                <input type="checkbox" id="autoRefresh" checked>
                Auto-refresh (30s)
            </label>
            <button class="refresh-btn" onclick="refresh()">Refresh</button>
        </div>

        <div class="footer">
            <a href="https://fermilabs.xyz">Fermi Labs</a>
        </div>
    </div>

    <script>
        let autoRefreshInterval;

        function refresh() {
            window.location.reload();
        }

        function startAutoRefresh() {
            autoRefreshInterval = setInterval(refresh, 30000);
        }

        function stopAutoRefresh() {
            if (autoRefreshInterval) clearInterval(autoRefreshInterval);
        }

        document.getElementById('autoRefresh').addEventListener('change', (e) => {
            e.target.checked ? startAutoRefresh() : stopAutoRefresh();
        });

        startAutoRefresh();
    </script>
</body>
</html>`

var statusPageTmpl *template.Template

func init() {
	funcMap := template.FuncMap{
		"latencyClass": func(ms float64) string {
			if ms < 100 {
				return "fast"
			} else if ms < 500 {
				return "medium"
			}
			return "slow"
		},
		"latencyWidth": func(ms float64) float64 {
			width := ms / 10
			if width > 100 {
				width = 100
			}
			return width
		},
	}
	statusPageTmpl = template.Must(template.New("statuspage").Funcs(funcMap).Parse(statusPageTemplate))
}

// StatusPageHandler returns an HTTP handler that serves a beautiful HTML status page
func StatusPageHandler(deps *StatusDependencies) http.HandlerFunc {
	statusHandler := StatusHandler(deps)

	return func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{headers: make(http.Header)}
		statusHandler.ServeHTTP(rec, r)

		var status SystemStatus
		if err := json.Unmarshal(rec.body, &status); err != nil {
			http.Error(w, "Failed to get status", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := statusPageTmpl.Execute(w, status); err != nil {
			http.Error(w, "Failed to render page", http.StatusInternalServerError)
			return
		}
	}
}

type responseRecorder struct {
	headers    http.Header
	body       []byte
	statusCode int
}

func (r *responseRecorder) Header() http.Header {
	return r.headers
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}
