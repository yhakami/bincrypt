# BTCPay Server on Google Cloud Platform

A complete, production-ready deployment solution for BTCPay Server on Google Cloud Platform, optimized for cost-efficiency using free-tier resources where possible.

## 🚀 Overview

This solution provides a fully automated deployment of BTCPay Server on Google Cloud Platform with:

- **Cloud Run** for containerized BTCPay Server hosting
- **Compute Engine e2-micro VM** (free tier) for PostgreSQL database
- **Cloud Build** for CI/CD automation
- **Secret Manager** for secure credential management
- **Cloud Load Balancer** with managed SSL certificates
- **Artifact Registry** for Docker image storage
- **Monitoring & Logging** for observability

## 💰 Estimated Monthly Costs

Optimized for minimal costs until payment traffic arrives:

| Service | Configuration | Monthly Cost |
|---------|--------------|--------------|
| Compute Engine (DB) | e2-micro (free tier) | **$0.00** |
| Cloud Run | 1 CPU, 512MB RAM, 0 min instances | **$0-5** |
| VPC Connector | 2 instances minimum | ~$7 |
| Cloud Storage | Logs, backups (<5GB) | ~$0.10 |
| **Total Estimate** | | **~$7-12/month** |

*Note: Cloud Run scales to zero when not in use. Load Balancer only needed for custom domain SSL.*

## 📋 Prerequisites

1. **Google Cloud Account** with billing enabled
2. **gcloud CLI** installed and configured
3. **Git** repository for your code
4. **Domain name** for SSL certificate
5. **Firebase project** (for storage emulator compatibility)

## 🛠️ Deployment Instructions

### Step 1: Initial Setup

```bash
# Clone the repository
git clone <your-repo-url>
cd bincrypt/BTCPay

# Make scripts executable
chmod +x scripts/*.sh
```

### Step 2: Configure Google Cloud Project

```bash
# Set your project ID
export PROJECT_ID="your-project-id"
gcloud config set project $PROJECT_ID

# Enable billing (required for paid services)
# Visit: https://console.cloud.google.com/billing
```

### Step 3: Deploy BTCPay Server

Run the automated deployment script:

```bash
# Option 1: Using environment variables
export PROJECT_ID="your-project-id"
export REGION="us-central1"  # optional
export DOMAIN_NAME="btcpay.example.com"  # optional
export ALERT_EMAIL="admin@example.com"  # optional
./scripts/deploy.sh

# Option 2: Using command line arguments
./scripts/deploy.sh your-project-id us-central1 btcpay.example.com admin@example.com
```

The script will:
1. Enable required Google Cloud APIs
2. Create networking infrastructure (VPC, subnets)
3. Deploy PostgreSQL on Compute Engine VM
4. Set up Secret Manager with credentials
5. Build and deploy BTCPay Server to Cloud Run
6. Configure HTTPS with managed SSL certificates
7. Set up monitoring and alerts

### Step 4: DNS Configuration

After deployment completes:

1. Note the static IP address shown in deployment output
2. Add DNS A record: `your-domain.com → <static-ip>`
3. Wait 15-20 minutes for SSL certificate provisioning

### Step 5: Complete BTCPay Setup

1. Navigate to `https://your-domain.com`
2. Create admin account (first user becomes admin)
3. Configure your stores and payment methods
4. Generate API keys for integration

## 🔧 Post-Deployment Management

### Viewing Logs

```bash
# BTCPay Server logs
gcloud run logs read --service=btcpay-server --region=us-central1

# PostgreSQL logs
gcloud compute ssh btcpay-postgres --zone=us-central1-a \
  --command="sudo tail -f /var/log/postgresql/*.log"
```

### Database Backups

Automated daily backups are configured. To manually backup:

```bash
gcloud compute ssh btcpay-postgres --zone=us-central1-a \
  --command="sudo /etc/cron.daily/backup-btcpay-db"
```

### Updating BTCPay Server

1. Update the version in `Dockerfile`
2. Commit and push to main branch
3. Cloud Build will automatically deploy

Or manually:
```bash
gcloud builds submit --config=cloudbuild.yaml
```

### Managing Secrets

```bash
# View secrets
gcloud secrets list

# Update a secret
echo -n "new-password" | gcloud secrets versions add btcpay-postgres-password --data-file=-
```

### Monitoring

- **Uptime**: Cloud Monitoring dashboard
- **Alerts**: Email notifications for downtime
- **Metrics**: CPU, memory, request latency
- **Logs**: Centralized in Cloud Logging

## 🔐 Security Configuration

### Secrets Management

All sensitive data stored in Secret Manager:
- `btcpay-postgres-password`
- `btcpay-postgres-host`
- `btcpay-bitcoin-rpc-user`
- `btcpay-bitcoin-rpc-password`

### Network Security

- PostgreSQL accessible only within VPC
- Cloud Run connected via VPC connector
- Firewall rules restrict database access
- SSL/TLS enforced for all connections

### Application Security

- CSP headers configured
- HSTS enabled with preload
- Rate limiting implemented
- Automatic security updates enabled

## 🚨 Troubleshooting

### BTCPay Server Won't Start

```bash
# Check Cloud Run logs
gcloud run logs read --service=btcpay-server --limit=50

# Verify secrets are set
gcloud secrets list
```

### Database Connection Issues

```bash
# Test PostgreSQL connectivity
gcloud compute ssh btcpay-postgres --zone=us-central1-a \
  --command="sudo -u postgres psql -c '\l'"

# Check VPC connector
gcloud compute networks vpc-access connectors describe btcpay-connector \
  --region=us-central1
```

### SSL Certificate Issues

```bash
# Check certificate status
gcloud compute ssl-certificates describe btcpay-cert --global

# Verify DNS propagation
nslookup your-domain.com
```

## 📊 Monitoring & Maintenance

### Health Checks

- Cloud Run: `/api/v1/health`
- PostgreSQL: Port 5432 connectivity
- Uptime monitoring: 60-second intervals

### Regular Maintenance

**Daily**:
- Automated PostgreSQL backups
- Log rotation

**Weekly**:
- Database vacuum and reindex
- Security updates check

**Monthly**:
- Review resource usage
- Update BTCPay if needed
- Audit access logs

## 🔄 CI/CD Pipeline

The Cloud Build pipeline automatically:
1. Builds Docker image on push to main
2. Pushes to Artifact Registry
3. Deploys to Cloud Run
4. Updates monitoring configuration

### Manual Deployment

```bash
# Submit build manually
gcloud builds submit --config=BTCPay/cloudbuild.yaml

# View build history
gcloud builds list --limit=10
```

## 📝 Configuration Files

- `Dockerfile`: BTCPay Server container definition
- `docker-compose.yml`: Local development setup
- `cloudbuild.yaml`: CI/CD pipeline configuration
- `configs/nginx.conf`: Reverse proxy configuration
- `configs/btc-config.yml`: Bitcoin Core settings
- `scripts/deploy.sh`: Automated deployment script
- `scripts/setup-db.sh`: PostgreSQL initialization

## 🆘 Support & Resources

### Official Documentation
- [BTCPay Server Docs](https://docs.btcpayserver.org/)
- [Google Cloud Run](https://cloud.google.com/run/docs)
- [Cloud Build](https://cloud.google.com/build/docs)

### Getting Help
- BTCPay Community: [Mattermost](https://chat.btcpayserver.org/)
- Google Cloud Support: [Console](https://console.cloud.google.com/support)

### Common Issues
- [BTCPay Troubleshooting](https://docs.btcpayserver.org/Troubleshooting/)
- [Cloud Run Troubleshooting](https://cloud.google.com/run/docs/troubleshooting)

## 📜 License

This deployment solution is provided under the MIT License. BTCPay Server is licensed under the MIT License.