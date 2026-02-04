# GCP Regions Reference

## Common Regions

| Location | Region | Default Zone | Description |
|----------|--------|--------------|-------------|
| **Europe** | | | |
| Frankfurt, Germany | `europe-west3` | `europe-west3-a` | Low latency for EU |
| London, UK | `europe-west2` | `europe-west2-a` | UK/EU access |
| Paris, France | `europe-west9` | `europe-west9-a` | EU central |
| Amsterdam, Netherlands | `europe-west4` | `europe-west4-a` | EU west |
| **US** | | | |
| Iowa | `us-central1` | `us-central1-a` | Central US |
| Oregon | `us-west1` | `us-west1-a` | West coast US |
| Virginia | `us-east4` | `us-east4-a` | East coast US |
| **Asia** | | | |
| Tokyo, Japan | `asia-northeast1` | `asia-northeast1-a` | Japan/Asia Pacific |
| Singapore | `asia-southeast1` | `asia-southeast1-a` | Southeast Asia |
| Mumbai, India | `asia-south1` | `asia-south1-a` | South Asia |

## Deployment Examples

### Frankfurt (Europe)
```bash
export GCP_PROJECT_ID="fermi-testnet"
./deploy-gce-simple.sh europe-west3 europe-west3-a fermi-gateway-eu e2-medium
```

### US Central (Iowa)
```bash
export GCP_PROJECT_ID="fermi-testnet"
./deploy-gce-simple.sh us-central1 us-central1-a fermi-gateway-us e2-medium
```

### Asia (Singapore)
```bash
export GCP_PROJECT_ID="fermi-testnet"
./deploy-gce-simple.sh asia-southeast1 asia-southeast1-a fermi-gateway-asia e2-medium
```

## Quick Deploy Commands

```bash
# Frankfurt
export GCP_PROJECT_ID="fermi-testnet"
./deploy-gce-simple.sh europe-west3

# Defaults to:
# - Zone: europe-west3-a
# - Instance: fermi-gateway
# - Machine: e2-medium
```

## SSL Setup for Regional Deployments

```bash
# After deployment, set up SSL
./setup-nginx-ssl.sh your-domain.com admin@email.com europe-west3-a fermi-gateway-eu
```

## Multi-Region Deployment

To deploy to multiple regions:

```bash
# Europe
./deploy-gce-simple.sh europe-west3 europe-west3-a fermi-eu

# US
./deploy-gce-simple.sh us-central1 us-central1-a fermi-us

# Asia
./deploy-gce-simple.sh asia-southeast1 asia-southeast1-a fermi-asia
```

Then use GCP Load Balancer to route traffic to nearest region.
