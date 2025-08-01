#!/bin/bash
set -euo pipefail

# BTCPay Server Deployment Script for Google Cloud Platform
# This script automates the complete deployment of BTCPay Server

echo "==============================================="
echo "BTCPay Server Google Cloud Deployment Script"
echo "==============================================="

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}Error: gcloud CLI is not installed${NC}"
    echo "Please install Google Cloud SDK: https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Get project configuration from environment or arguments
PROJECT_ID=${PROJECT_ID:-$1}
REGION=${REGION:-${2:-us-central1}}
DOMAIN_NAME=${DOMAIN_NAME:-$3}
ALERT_EMAIL=${ALERT_EMAIL:-$4}

if [ -z "$PROJECT_ID" ]; then
    echo -e "${RED}Error: PROJECT_ID not set${NC}"
    echo "Usage: PROJECT_ID=your-project-id ./deploy.sh"
    echo "   or: ./deploy.sh your-project-id [region] [domain] [alert-email]"
    exit 1
fi

# Set project
gcloud config set project ${PROJECT_ID}

echo -e "\n${GREEN}Step 1: Enabling required Google Cloud APIs...${NC}"
gcloud services enable \
    compute.googleapis.com \
    containerregistry.googleapis.com \
    cloudbuild.googleapis.com \
    run.googleapis.com \
    secretmanager.googleapis.com \
    vpcaccess.googleapis.com \
    servicenetworking.googleapis.com \
    sqladmin.googleapis.com \
    logging.googleapis.com \
    monitoring.googleapis.com \
    artifactregistry.googleapis.com

echo -e "\n${GREEN}Step 2: Creating Artifact Registry repository...${NC}"
gcloud artifacts repositories create btcpay \
    --repository-format=docker \
    --location=${REGION} \
    --description="BTCPay Server Docker images" || echo "Repository already exists"

echo -e "\n${GREEN}Step 3: Setting up VPC for database connectivity...${NC}"
# Create VPC network
gcloud compute networks create btcpay-network \
    --subnet-mode=custom \
    --bgp-routing-mode=regional || echo "Network already exists"

# Create subnet
gcloud compute networks subnets create btcpay-subnet \
    --network=btcpay-network \
    --region=${REGION} \
    --range=10.0.0.0/24 || echo "Subnet already exists"

# Create VPC connector for Cloud Run
gcloud compute networks vpc-access connectors create btcpay-connector \
    --region=${REGION} \
    --subnet=btcpay-subnet \
    --subnet-project=${PROJECT_ID} \
    --min-instances=2 \
    --max-instances=10 || echo "VPC connector already exists"

echo -e "\n${GREEN}Step 4: Creating PostgreSQL database on Compute Engine...${NC}"
# Check if VM already exists
if ! gcloud compute instances describe btcpay-postgres --zone=${REGION}-a &>/dev/null; then
    # Create firewall rule for PostgreSQL
    gcloud compute firewall-rules create allow-postgres-internal \
        --network=btcpay-network \
        --allow=tcp:5432 \
        --source-ranges=10.0.0.0/24 \
        --target-tags=postgres-server || echo "Firewall rule already exists"

    # Create Compute Engine instance (e2-micro for free tier)
    gcloud compute instances create btcpay-postgres \
        --zone=${REGION}-a \
        --machine-type=e2-micro \
        --network-interface=subnet=btcpay-subnet,no-address \
        --maintenance-policy=MIGRATE \
        --boot-disk-size=30GB \
        --boot-disk-type=pd-standard \
        --image-family=ubuntu-2204-lts \
        --image-project=ubuntu-os-cloud \
        --tags=postgres-server \
        --metadata-from-file startup-script=scripts/setup-db.sh
    
    echo "Waiting for PostgreSQL setup to complete..."
    sleep 60
else
    echo "PostgreSQL VM already exists"
fi

# Get internal IP of PostgreSQL VM
POSTGRES_IP=$(gcloud compute instances describe btcpay-postgres \
    --zone=${REGION}-a \
    --format='get(networkInterfaces[0].networkIP)')

echo -e "\n${GREEN}Step 5: Setting up Google Secret Manager...${NC}"
# Function to create or update secret
create_or_update_secret() {
    SECRET_NAME=$1
    SECRET_VALUE=$2
    
    if gcloud secrets describe ${SECRET_NAME} &>/dev/null; then
        echo "${SECRET_VALUE}" | gcloud secrets versions add ${SECRET_NAME} --data-file=-
    else
        echo "${SECRET_VALUE}" | gcloud secrets create ${SECRET_NAME} --data-file=-
    fi
}

# Generate secure passwords
POSTGRES_PASSWORD=$(openssl rand -base64 32)
BITCOIN_RPC_PASSWORD=$(openssl rand -base64 32)
BITCOIN_RPC_USER="btcpay"

# Create secrets
echo "Creating secrets in Secret Manager..."
# Create single connection string for BTCPay
POSTGRES_CONNECTION="User ID=btcpay;Password=${POSTGRES_PASSWORD};Host=${POSTGRES_IP};Port=5432;Database=btcpayserver;"
create_or_update_secret "btcpay-postgres-connection" "${POSTGRES_CONNECTION}"
create_or_update_secret "btcpay-external-url" "https://btcpay-server-${PROJECT_ID}.run.app"

echo -e "\n${GREEN}Step 6: Creating service account...${NC}"
gcloud iam service-accounts create btcpay-server \
    --display-name="BTCPay Server Service Account" || echo "Service account already exists"

# Grant permissions
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:btcpay-server@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:btcpay-server@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/logging.logWriter"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:btcpay-server@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/monitoring.metricWriter"

echo -e "\n${GREEN}Step 7: Building and deploying BTCPay Server...${NC}"
# Submit build to Cloud Build
gcloud builds submit \
    --config=BTCPay/cloudbuild.yaml \
    --substitutions=_PROJECT_ID=${PROJECT_ID},_REGION=${REGION} \
    .

echo -e "\n${GREEN}Step 8: Setting up Cloud Load Balancer with SSL...${NC}"
# Reserve static IP
gcloud compute addresses create btcpay-ip \
    --global || echo "IP address already reserved"

STATIC_IP=$(gcloud compute addresses describe btcpay-ip --global --format="get(address)")

# Create backend service
gcloud compute backend-services create btcpay-backend \
    --global \
    --protocol=HTTP \
    --port-name=http \
    --timeout=30s || echo "Backend service already exists"

# Create URL map
gcloud compute url-maps create btcpay-lb \
    --default-service=btcpay-backend || echo "URL map already exists"

# Create HTTPS proxy with managed certificate
if [ -z "$DOMAIN_NAME" ]; then
    echo -e "${YELLOW}Warning: No domain name provided, using Cloud Run auto-generated URL${NC}"
    DOMAIN_NAME="btcpay-server-${PROJECT_ID}.run.app"
fi

gcloud compute ssl-certificates create btcpay-cert \
    --domains=${DOMAIN_NAME} \
    --global || echo "SSL certificate already exists"

gcloud compute target-https-proxies create btcpay-https-proxy \
    --url-map=btcpay-lb \
    --ssl-certificates=btcpay-cert \
    --global || echo "HTTPS proxy already exists"

# Create forwarding rule
gcloud compute forwarding-rules create btcpay-https-rule \
    --address=btcpay-ip \
    --global \
    --target-https-proxy=btcpay-https-proxy \
    --ports=443 || echo "Forwarding rule already exists"

echo -e "\n${GREEN}Step 9: Creating Cloud Build trigger...${NC}"
gcloud builds triggers create github \
    --repo-name=bincrypt \
    --repo-owner=$(git config --get remote.origin.url | sed 's/.*github.com[:/]\([^/]*\).*/\1/') \
    --branch-pattern="^main$" \
    --build-config=BTCPay/cloudbuild.yaml \
    --name=btcpay-server-deploy \
    --description="Deploy BTCPay Server on push to main" || echo "Trigger already exists"

echo -e "\n${GREEN}Step 10: Setting up monitoring...${NC}"
# Create notification channel (email) only if provided
if [ -n "$ALERT_EMAIL" ]; then

CHANNEL_ID=$(gcloud alpha monitoring channels create \
    --display-name="BTCPay Alerts Email" \
    --type=email \
    --channel-labels=email_address=${ALERT_EMAIL} \
    --format="value(name)")

    # Update alert policy with notification channel
    gcloud alpha monitoring policies list --filter="displayName:'BTCPay Server Down'" \
        --format="value(name)" | xargs -I {} \
        gcloud alpha monitoring policies update {} --add-notification-channels=${CHANNEL_ID}
else
    echo -e "${YELLOW}No alert email provided, skipping email notifications${NC}"
fi

echo -e "\n${YELLOW}===============================================${NC}"
echo -e "${GREEN}Deployment Complete!${NC}"
echo -e "${YELLOW}===============================================${NC}"
echo -e "\nImportant Information:"
echo -e "- Static IP: ${STATIC_IP}"
echo -e "- Cloud Run URL: https://btcpay-server-${PROJECT_ID}.run.app"
echo -e "- Domain: ${DOMAIN_NAME}"
echo -e "\n${YELLOW}Next Steps:${NC}"
echo -e "1. Point your domain's DNS A record to: ${STATIC_IP}"
echo -e "2. Wait 15-20 minutes for SSL certificate provisioning"
echo -e "3. Access BTCPay Server at: https://${DOMAIN_NAME}"
echo -e "4. Complete BTCPay Server setup wizard"
echo -e "\n${YELLOW}Database Connection:${NC}"
echo -e "- Host: ${POSTGRES_IP}"
echo -e "- Database: btcpayserver"
echo -e "- User: btcpay"
echo -e "- Password: Stored in Secret Manager (btcpay-postgres-password)"
echo -e "\n${GREEN}Deployment logs available in Cloud Console${NC}"