# BTCPay Server Dockerfile for Google Cloud Run
FROM btcpayserver/btcpayserver:1.11.7

# Install minimal dependencies
RUN apt-get update && apt-get install -y \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security
RUN useradd -m -u 1000 btcpay && \
    mkdir -p /datadir /root/.btcpayserver && \
    chown -R btcpay:btcpay /datadir /root/.btcpayserver

# Set up environment for Cloud Run
ENV BTCPAY_DATADIR=/datadir
ENV BTCPAY_PLUGINDIR=/datadir/plugins
ENV BTCPAY_DOCKERDEPLOYMENT=false
ENV BTCPAY_NETWORK=mainnet
ENV BTCPAY_BIND=0.0.0.0:8080
ENV BTCPAY_ROOTPATH=/
ENV BTCPAY_DEBUGLOG=btcpay.log
ENV BTCPAY_DEBUGLOGLEVEL=2
ENV BTCPAY_BUNDLEJSCSS=true

# Google Cloud specific settings
ENV PORT=8080
ENV BTCPAY_ALLOWADMINREGISTRATION=false
ENV BTCPAY_DISABLEREGISTRATION=false

# Health check for Cloud Run
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/health || exit 1

# Expose port for Cloud Run
EXPOSE 8080

# Use BTCPay's native entrypoint - Cloud Run will inject secrets as env vars
USER btcpay
WORKDIR /app
ENTRYPOINT ["dotnet", "BTCPayServer.dll"]