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
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            min-height: 100vh;
            color: #e4e4e4;
            padding: 40px 20px;
        }

        .container {
            max-width: 900px;
            margin: 0 auto;
        }

        .header {
            text-align: center;
            margin-bottom: 40px;
        }

        .header h1 {
            font-size: 2.5rem;
            font-weight: 700;
            margin-bottom: 10px;
            background: linear-gradient(90deg, #00d4ff, #7b2cbf);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .header .subtitle {
            color: #888;
            font-size: 1rem;
        }

        .overall-status {
            background: rgba(255, 255, 255, 0.05);
            border-radius: 16px;
            padding: 30px;
            text-align: center;
            margin-bottom: 30px;
            border: 1px solid rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
        }

        .status-indicator {
            display: inline-flex;
            align-items: center;
            gap: 12px;
            font-size: 1.8rem;
            font-weight: 600;
        }

        .status-dot {
            width: 20px;
            height: 20px;
            border-radius: 50%;
            animation: pulse 2s infinite;
        }

        .status-dot.healthy {
            background: #00ff88;
            box-shadow: 0 0 20px rgba(0, 255, 136, 0.5);
        }

        .status-dot.degraded {
            background: #ffaa00;
            box-shadow: 0 0 20px rgba(255, 170, 0, 0.5);
        }

        .status-dot.unhealthy {
            background: #ff4757;
            box-shadow: 0 0 20px rgba(255, 71, 87, 0.5);
        }

        @keyframes pulse {
            0%, 100% { opacity: 1; transform: scale(1); }
            50% { opacity: 0.7; transform: scale(1.1); }
        }

        .timestamp {
            color: #666;
            font-size: 0.9rem;
            margin-top: 15px;
        }

        .section-title {
            font-size: 1.2rem;
            font-weight: 600;
            margin-bottom: 20px;
            color: #aaa;
            text-transform: uppercase;
            letter-spacing: 1px;
        }

        .services-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 20px;
            margin-bottom: 40px;
        }

        .service-card {
            background: rgba(255, 255, 255, 0.05);
            border-radius: 12px;
            padding: 24px;
            border: 1px solid rgba(255, 255, 255, 0.1);
            transition: transform 0.2s, box-shadow 0.2s;
        }

        .service-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.3);
        }

        .service-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 16px;
        }

        .service-name {
            font-size: 1.1rem;
            font-weight: 600;
        }

        .service-status {
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 6px 12px;
            border-radius: 20px;
            font-size: 0.85rem;
            font-weight: 500;
        }

        .service-status.healthy {
            background: rgba(0, 255, 136, 0.15);
            color: #00ff88;
        }

        .service-status.unhealthy {
            background: rgba(255, 71, 87, 0.15);
            color: #ff4757;
        }

        .service-status .dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
        }

        .service-status.healthy .dot {
            background: #00ff88;
        }

        .service-status.unhealthy .dot {
            background: #ff4757;
        }

        .service-latency {
            display: flex;
            align-items: center;
            gap: 8px;
            color: #888;
            font-size: 0.9rem;
        }

        .latency-bar {
            flex: 1;
            height: 6px;
            background: rgba(255, 255, 255, 0.1);
            border-radius: 3px;
            overflow: hidden;
        }

        .latency-fill {
            height: 100%;
            border-radius: 3px;
            transition: width 0.5s ease;
        }

        .latency-fill.fast {
            background: linear-gradient(90deg, #00ff88, #00d4ff);
        }

        .latency-fill.medium {
            background: linear-gradient(90deg, #ffaa00, #ff6b6b);
        }

        .latency-fill.slow {
            background: linear-gradient(90deg, #ff4757, #c0392b);
        }

        .service-error {
            margin-top: 12px;
            padding: 10px;
            background: rgba(255, 71, 87, 0.1);
            border-radius: 8px;
            color: #ff6b6b;
            font-size: 0.85rem;
        }

        .gateway-stats {
            background: rgba(255, 255, 255, 0.05);
            border-radius: 12px;
            padding: 24px;
            border: 1px solid rgba(255, 255, 255, 0.1);
        }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
            gap: 20px;
        }

        .stat-item {
            text-align: center;
        }

        .stat-value {
            font-size: 1.5rem;
            font-weight: 700;
            background: linear-gradient(90deg, #00d4ff, #7b2cbf);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .stat-label {
            font-size: 0.8rem;
            color: #666;
            margin-top: 4px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .refresh-btn {
            position: fixed;
            bottom: 30px;
            right: 30px;
            width: 56px;
            height: 56px;
            border-radius: 50%;
            background: linear-gradient(135deg, #00d4ff, #7b2cbf);
            border: none;
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            box-shadow: 0 4px 20px rgba(0, 212, 255, 0.3);
            transition: transform 0.2s, box-shadow 0.2s;
        }

        .refresh-btn:hover {
            transform: scale(1.1);
            box-shadow: 0 6px 30px rgba(0, 212, 255, 0.5);
        }

        .refresh-btn svg {
            width: 24px;
            height: 24px;
            fill: white;
        }

        .refresh-btn.loading svg {
            animation: spin 1s linear infinite;
        }

        @keyframes spin {
            from { transform: rotate(0deg); }
            to { transform: rotate(360deg); }
        }

        .footer {
            text-align: center;
            margin-top: 40px;
            color: #555;
            font-size: 0.85rem;
        }

        .footer a {
            color: #00d4ff;
            text-decoration: none;
        }

        .auto-refresh {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 8px;
            margin-top: 10px;
            color: #666;
            font-size: 0.85rem;
        }

        .auto-refresh input {
            accent-color: #00d4ff;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Fermi API Gateway</h1>
            <p class="subtitle">System Status Dashboard</p>
        </div>

        <div class="overall-status">
            <div class="status-indicator">
                <span class="status-dot {{if eq .Status "healthy"}}healthy{{else}}degraded{{end}}"></span>
                <span>{{if eq .Status "healthy"}}All Systems Operational{{else}}System Degraded{{end}}</span>
            </div>
            <div class="timestamp">Last updated: <span id="timestamp">{{.Timestamp.Format "Jan 02, 2006 15:04:05 MST"}}</span></div>
            <div class="auto-refresh">
                <input type="checkbox" id="autoRefresh" checked>
                <label for="autoRefresh">Auto-refresh every 30s</label>
            </div>
        </div>

        <h2 class="section-title">Services</h2>
        <div class="services-grid">
            <div class="service-card">
                <div class="service-header">
                    <span class="service-name">Rollup Service</span>
                    <span class="service-status {{.Services.Rollup.Status}}">
                        <span class="dot"></span>
                        {{.Services.Rollup.Status}}
                    </span>
                </div>
                <div class="service-latency">
                    <div class="latency-bar">
                        <div class="latency-fill {{latencyClass .Services.Rollup.LatencyMs}}" style="width: {{latencyWidth .Services.Rollup.LatencyMs}}%"></div>
                    </div>
                    <span>{{printf "%.1f" .Services.Rollup.LatencyMs}}ms</span>
                </div>
                {{if .Services.Rollup.Error}}
                <div class="service-error">{{.Services.Rollup.Error}}</div>
                {{end}}
            </div>

            <div class="service-card">
                <div class="service-header">
                    <span class="service-name">Continuum Service</span>
                    <span class="service-status {{.Services.Continuum.Status}}">
                        <span class="dot"></span>
                        {{.Services.Continuum.Status}}
                    </span>
                </div>
                <div class="service-latency">
                    <div class="latency-bar">
                        <div class="latency-fill {{latencyClass .Services.Continuum.LatencyMs}}" style="width: {{latencyWidth .Services.Continuum.LatencyMs}}%"></div>
                    </div>
                    <span>{{printf "%.1f" .Services.Continuum.LatencyMs}}ms</span>
                </div>
                {{if .Services.Continuum.Error}}
                <div class="service-error">{{.Services.Continuum.Error}}</div>
                {{end}}
            </div>

            <div class="service-card">
                <div class="service-header">
                    <span class="service-name">TimescaleDB</span>
                    <span class="service-status {{.Services.TimescaleDB.Status}}">
                        <span class="dot"></span>
                        {{.Services.TimescaleDB.Status}}
                    </span>
                </div>
                <div class="service-latency">
                    <div class="latency-bar">
                        <div class="latency-fill {{latencyClass .Services.TimescaleDB.LatencyMs}}" style="width: {{latencyWidth .Services.TimescaleDB.LatencyMs}}%"></div>
                    </div>
                    <span>{{printf "%.1f" .Services.TimescaleDB.LatencyMs}}ms</span>
                </div>
                {{if .Services.TimescaleDB.Error}}
                <div class="service-error">{{.Services.TimescaleDB.Error}}</div>
                {{end}}
            </div>
        </div>

        <h2 class="section-title">Gateway Statistics</h2>
        <div class="gateway-stats">
            <div class="stats-grid">
                <div class="stat-item">
                    <div class="stat-value">{{.Gateway.Uptime}}</div>
                    <div class="stat-label">Uptime</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value">{{.Gateway.Goroutines}}</div>
                    <div class="stat-label">Goroutines</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value">{{printf "%.1f" .Gateway.MemAllocMB}} MB</div>
                    <div class="stat-label">Memory Used</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value">{{printf "%.1f" .Gateway.MemSysMB}} MB</div>
                    <div class="stat-label">System Memory</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value">{{.Gateway.NumGC}}</div>
                    <div class="stat-label">GC Cycles</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value">{{.Gateway.GoVersion}}</div>
                    <div class="stat-label">Go Version</div>
                </div>
            </div>
        </div>

        <div class="footer">
            <p>Powered by <a href="https://fermilabs.xyz">Fermi Labs</a></p>
        </div>
    </div>

    <button class="refresh-btn" onclick="refresh()" title="Refresh">
        <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
            <path d="M17.65 6.35A7.958 7.958 0 0012 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08A5.99 5.99 0 0112 18c-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/>
        </svg>
    </button>

    <script>
        let autoRefreshInterval;

        function refresh() {
            const btn = document.querySelector('.refresh-btn');
            btn.classList.add('loading');
            window.location.reload();
        }

        function startAutoRefresh() {
            autoRefreshInterval = setInterval(() => {
                window.location.reload();
            }, 30000);
        }

        function stopAutoRefresh() {
            if (autoRefreshInterval) {
                clearInterval(autoRefreshInterval);
            }
        }

        document.getElementById('autoRefresh').addEventListener('change', (e) => {
            if (e.target.checked) {
                startAutoRefresh();
            } else {
                stopAutoRefresh();
            }
        });

        // Start auto-refresh by default
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
			// Map latency to width percentage (0-1000ms -> 0-100%)
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
		// Get the status data by calling the JSON handler
		rec := &responseRecorder{headers: make(http.Header)}
		statusHandler.ServeHTTP(rec, r)

		// Parse the JSON response
		var status SystemStatus
		if err := json.Unmarshal(rec.body, &status); err != nil {
			http.Error(w, "Failed to get status", http.StatusInternalServerError)
			return
		}

		// Render the HTML template
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := statusPageTmpl.Execute(w, status); err != nil {
			http.Error(w, "Failed to render page", http.StatusInternalServerError)
			return
		}
	}
}

// responseRecorder captures the response from the status handler
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
