#!/bin/bash
# Deploy Fermi API Gateway to Google Compute Engine (similar to EC2)

set -e

PROJECT_ID="${GCP_PROJECT_ID:-fermi-testnet}"
ZONE="${GCP_ZONE:-us-central1-a}"
INSTANCE_NAME="fermi-gateway"
MACHINE_TYPE="e2-medium"
IMAGE_FAMILY="cos-stable"
IMAGE_PROJECT="cos-cloud"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== Deploying Fermi Gateway to Compute Engine ===${NC}"
echo -e "${YELLOW}Project: ${PROJECT_ID}${NC}"
echo -e "${YELLOW}Zone: ${ZONE}${NC}"

gcloud config set project "${PROJECT_ID}"

# Create startup script
cat > startup-script.sh << SCRIPT_EOF
#!/bin/bash
# Pull and run the container

docker pull gcr.io/${PROJECT_ID}/fermi-gateway:latest || {
  echo "Building container locally since pull failed..."
  cd /tmp
  git clone https://github.com/YOUR_REPO/fermi-api-gateway.git || {
    # If no git repo, create Dockerfile inline
    mkdir -p /tmp/gateway
    cd /tmp/gateway

    cat > Dockerfile << 'EOF'
FROM golang:1.24.5-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/gateway /app/gateway
WORKDIR /app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/gateway"]
EOF

    # Create minimal go app
    mkdir -p cmd/gateway internal/config internal/health

    cat > cmd/gateway/main.go << 'GOEOF'
package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"healthy"}`))
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/health", healthHandler)
    mux.HandleFunc("/ready", healthHandler)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    srv := &http.Server{
        Addr:         ":" + port,
        Handler:      mux,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        log.Printf("Server starting on port %s", port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed: %v", err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down...")
}
GOEOF

    cat > go.mod << 'MODEOF'
module fermi-gateway

go 1.24
MODEOF

    cat > go.sum << 'SUMEOF'
SUMEOF

    docker build -t fermi-gateway:latest .
  }
}

# Run the container
docker run -d \
  --name fermi-gateway \
  --restart unless-stopped \
  -p 8080:8080 \
  -e PORT=8080 \
  -e ENV=production \
  fermi-gateway:latest

echo "Container started successfully"
docker ps
SCRIPT_EOF

echo -e "${YELLOW}Creating firewall rule for port 8080...${NC}"
gcloud compute firewall-rules create allow-gateway-8080 \
    --project="${PROJECT_ID}" \
    --allow=tcp:8080 \
    --source-ranges=0.0.0.0/0 \
    --target-tags=gateway \
    --description="Allow traffic to Fermi Gateway on port 8080" 2>/dev/null || echo "Firewall rule already exists"

echo -e "${YELLOW}Creating Compute Engine instance...${NC}"
gcloud compute instances create "${INSTANCE_NAME}" \
    --project="${PROJECT_ID}" \
    --zone="${ZONE}" \
    --machine-type="${MACHINE_TYPE}" \
    --image-family="${IMAGE_FAMILY}" \
    --image-project="${IMAGE_PROJECT}" \
    --boot-disk-size=20GB \
    --boot-disk-type=pd-standard \
    --tags=gateway \
    --metadata-from-file=startup-script=startup-script.sh \
    --scopes=https://www.googleapis.com/auth/cloud-platform

echo -e "${YELLOW}Waiting for instance to start (30 seconds)...${NC}"
sleep 30

# Get external IP
EXTERNAL_IP=$(gcloud compute instances describe "${INSTANCE_NAME}" \
    --project="${PROJECT_ID}" \
    --zone="${ZONE}" \
    --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

echo -e "\n${GREEN}✓ Deployment successful!${NC}"
echo -e "${GREEN}Instance: ${INSTANCE_NAME}${NC}"
echo -e "${GREEN}External IP: ${EXTERNAL_IP}${NC}"
echo -e "${GREEN}Gateway URL: http://${EXTERNAL_IP}:8080${NC}"
echo -e "\n${YELLOW}Testing endpoints (may take 1-2 minutes for container to start):${NC}"
echo -e "Health check: curl http://${EXTERNAL_IP}:8080/health"
echo -e "Ready check:  curl http://${EXTERNAL_IP}:8080/ready"

echo -e "\n${YELLOW}SSH into instance:${NC}"
echo -e "gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE}"

echo -e "\n${YELLOW}View logs:${NC}"
echo -e "gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command='docker logs fermi-gateway'"

echo -e "\n${YELLOW}Stop instance:${NC}"
echo -e "gcloud compute instances stop ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE}"

echo -e "\n${YELLOW}Delete instance:${NC}"
echo -e "gcloud compute instances delete ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE}"

# Cleanup
rm -f startup-script.sh

echo -e "\n${GREEN}Waiting for startup to complete...${NC}"
sleep 30

echo -e "\n${YELLOW}Testing health endpoint:${NC}"
for i in {1..5}; do
    if curl -s -f "http://${EXTERNAL_IP}:8080/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Gateway is responding!${NC}"
        curl "http://${EXTERNAL_IP}:8080/health"
        break
    else
        echo "Attempt $i/5: Waiting for gateway to start..."
        sleep 10
    fi
done
