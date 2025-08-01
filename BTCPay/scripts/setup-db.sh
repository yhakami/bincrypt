#!/bin/bash
set -euo pipefail

# PostgreSQL Setup Script for BTCPay Server
# This script runs on the Compute Engine VM to set up PostgreSQL

echo "Starting PostgreSQL setup for BTCPay Server..."

# Update system
apt-get update
apt-get upgrade -y

# Install PostgreSQL 14
apt-get install -y postgresql-14 postgresql-contrib-14

# Configure PostgreSQL for network access
cat > /etc/postgresql/14/main/postgresql.conf << EOF
# PostgreSQL configuration for BTCPay Server
listen_addresses = '*'
port = 5432
max_connections = 100
shared_buffers = 128MB
effective_cache_size = 512MB
maintenance_work_mem = 32MB
work_mem = 2MB
wal_buffers = 4MB
max_wal_size = 1GB
min_wal_size = 80MB

# Logging
log_destination = 'stderr'
logging_collector = on
log_directory = '/var/log/postgresql'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
log_rotation_age = 1d
log_rotation_size = 100MB
log_line_prefix = '%m [%p] %q%u@%d '
log_timezone = 'UTC'

# Performance
checkpoint_completion_target = 0.9
random_page_cost = 4.0
effective_io_concurrency = 2
default_statistics_target = 100

# Security
ssl = on
ssl_cert_file = '/etc/ssl/certs/ssl-cert-snakeoil.pem'
ssl_key_file = '/etc/ssl/private/ssl-cert-snakeoil.key'
EOF

# Configure authentication
cat > /etc/postgresql/14/main/pg_hba.conf << EOF
# PostgreSQL Client Authentication Configuration
# TYPE  DATABASE        USER            ADDRESS                 METHOD
local   all             all                                     peer
host    all             all             127.0.0.1/32            md5
host    all             all             ::1/128                 md5
host    all             all             10.0.0.0/24             md5
host    all             all             0.0.0.0/0               reject
EOF

# Restart PostgreSQL
systemctl restart postgresql

# Wait for PostgreSQL to start
sleep 5

# Generate secure password
POSTGRES_PASSWORD=$(openssl rand -base64 32)

# Create BTCPay database and user
sudo -u postgres psql << EOF
-- Create BTCPay user
CREATE USER btcpay WITH PASSWORD '${POSTGRES_PASSWORD}';

-- Create database
CREATE DATABASE btcpayserver OWNER btcpay;

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE btcpayserver TO btcpay;

-- Create schema
\c btcpayserver
CREATE SCHEMA IF NOT EXISTS public AUTHORIZATION btcpay;
GRANT ALL ON SCHEMA public TO btcpay;
EOF

# Set up automated backups
cat > /etc/cron.daily/backup-btcpay-db << 'EOF'
#!/bin/bash
# Daily backup script for BTCPay PostgreSQL database

BACKUP_DIR="/var/backups/postgresql"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DB_NAME="btcpayserver"

# Create backup directory if it doesn't exist
mkdir -p ${BACKUP_DIR}

# Perform backup
sudo -u postgres pg_dump ${DB_NAME} | gzip > ${BACKUP_DIR}/btcpay_${TIMESTAMP}.sql.gz

# Keep only last 7 days of backups
find ${BACKUP_DIR} -name "btcpay_*.sql.gz" -mtime +7 -delete

# Log backup completion
echo "$(date): BTCPay database backup completed" >> /var/log/btcpay-backup.log
EOF

chmod +x /etc/cron.daily/backup-btcpay-db

# Set up log rotation
cat > /etc/logrotate.d/postgresql-btcpay << EOF
/var/log/postgresql/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 640 postgres postgres
    sharedscripts
    postrotate
        /usr/bin/killall -SIGUSR1 postgres
    endscript
}
EOF

# Set up monitoring script
cat > /usr/local/bin/check-postgres-health.sh << 'EOF'
#!/bin/bash
# PostgreSQL health check script

# Check if PostgreSQL is running
if ! systemctl is-active --quiet postgresql; then
    echo "ERROR: PostgreSQL is not running"
    systemctl start postgresql
    exit 1
fi

# Check database connectivity
if ! sudo -u postgres psql -d btcpayserver -c "SELECT 1" > /dev/null 2>&1; then
    echo "ERROR: Cannot connect to btcpayserver database"
    exit 1
fi

# Check disk space
DISK_USAGE=$(df -h /var/lib/postgresql | awk 'NR==2 {print $5}' | sed 's/%//')
if [ $DISK_USAGE -gt 80 ]; then
    echo "WARNING: Disk usage is above 80%"
fi

echo "PostgreSQL health check passed"
EOF

chmod +x /usr/local/bin/check-postgres-health.sh

# Add health check to cron
echo "*/5 * * * * /usr/local/bin/check-postgres-health.sh >> /var/log/postgres-health.log 2>&1" | crontab -

# Configure automatic security updates
apt-get install -y unattended-upgrades
dpkg-reconfigure -plow unattended-upgrades

# Install fail2ban for security
apt-get install -y fail2ban

cat > /etc/fail2ban/jail.local << EOF
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 3

[sshd]
enabled = true

[postgresql]
enabled = true
port = 5432
filter = postgresql
logpath = /var/log/postgresql/*.log
EOF

systemctl enable fail2ban
systemctl start fail2ban

# Set up PostgreSQL performance tuning
cat > /usr/local/bin/tune-postgres.sh << 'EOF'
#!/bin/bash
# Auto-tune PostgreSQL based on available memory

TOTAL_MEM=$(free -m | awk 'NR==2{print $2}')
SHARED_BUFFERS=$((TOTAL_MEM / 4))
EFFECTIVE_CACHE=$((TOTAL_MEM * 3 / 4))

sudo -u postgres psql << SQL
ALTER SYSTEM SET shared_buffers = '${SHARED_BUFFERS}MB';
ALTER SYSTEM SET effective_cache_size = '${EFFECTIVE_CACHE}MB';
SQL

systemctl reload postgresql
EOF

chmod +x /usr/local/bin/tune-postgres.sh
/usr/local/bin/tune-postgres.sh

# Create maintenance script
cat > /usr/local/bin/maintain-btcpay-db.sh << 'EOF'
#!/bin/bash
# Database maintenance script

echo "Starting database maintenance..."

# Vacuum and analyze
sudo -u postgres vacuumdb -z btcpayserver

# Reindex
sudo -u postgres reindexdb btcpayserver

# Update statistics
sudo -u postgres psql -d btcpayserver -c "ANALYZE;"

echo "Database maintenance completed"
EOF

chmod +x /usr/local/bin/maintain-btcpay-db.sh

# Schedule weekly maintenance
echo "0 3 * * 0 /usr/local/bin/maintain-btcpay-db.sh >> /var/log/btcpay-maintenance.log 2>&1" | crontab -

echo "PostgreSQL setup completed successfully!"
echo "Database: btcpayserver"
echo "User: btcpay"
echo "Connection: postgresql://btcpay:PASSWORD@$(hostname -I | awk '{print $1}'):5432/btcpayserver"