#!/usr/bin/env bash
set -euo pipefail

echo "🔧 BinCrypt Setup Script"
echo "========================"
echo

check_docker() {
    if ! command -v docker &> /dev/null; then
        echo "❌ Docker is not installed"
        echo "Please install Docker: https://docs.docker.com/get-docker/"
        exit 1
    fi

    if ! docker info &> /dev/null; then
        echo "❌ Docker daemon is not running"
        echo "Please start Docker and try again"
        exit 1
    fi

    echo "✅ Docker is installed and running"
}

check_docker_compose() {
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null 2>&1; then
        echo "❌ Docker Compose is not installed"
        echo "Please install Docker Compose: https://docs.docker.com/compose/install/"
        exit 1
    fi
    echo "✅ Docker Compose is installed"
}

create_firebase_json() {
    if [ ! -f "firebase.json" ]; then
        echo "📝 Creating firebase.json..."
        cat > firebase.json << 'EOF'
{
  "hosting": {
    "public": "public",
    "ignore": [
      "firebase.json",
      "**/.*",
      "**/node_modules/**"
    ]
  },
  "emulators": {
    "storage": {
      "port": 9199,
      "host": "0.0.0.0"
    },
    "ui": {
      "enabled": true,
      "port": 4000,
      "host": "0.0.0.0"
    }
  },
  "storage": {
    "rules": "storage.rules.dev"
  }
}
EOF
        echo "✅ Created firebase.json"
    fi
}

create_storage_rules() {
    if [ ! -f "storage.rules" ]; then
        echo "📝 Creating storage.rules (locked down template)..."
        cat > storage.rules << 'EOF'
rules_version = '2';
service firebase.storage {
  match /b/{bucket}/o {
    // Deny access by default; customise for your deployment.
    match /{allPaths=**} {
      allow read, write: if false;
    }
  }
}
EOF
        echo "✅ Created storage.rules"
    fi

    if [ ! -f "storage.rules.dev" ]; then
        echo "📝 Creating storage.rules.dev (emulator-only)..."
        cat > storage.rules.dev << 'EOF'
// Development Firebase Storage rules for local emulators ONLY.
rules_version = '2';
service firebase.storage {
  match /b/{bucket}/o {
    match /{allPaths=**} {
      allow read, write: if true;
    }
  }
}
EOF
        echo "✅ Created storage.rules.dev"
    fi

    echo "ℹ️ firebase.json points at storage.rules.dev for emulator use. Replace storage.rules with real production rules before deploying."
}

choose_backend() {
    echo
    echo "Choose storage backend:"
    echo "  1) SQLite (simplest, no external DB)"
    echo "  2) PostgreSQL (recommended for self-hosting)"
    echo "  3) Google Cloud Storage (with Firebase emulator)"
    echo
    read -p "Enter choice [1-3]: " choice

    case $choice in
        1)
            BACKEND="sqlite"
            ;;
        2)
            BACKEND="postgres"
            ;;
        3)
            BACKEND="gcs"
            create_firebase_json
            create_storage_rules
            ;;
        *)
            echo "Invalid choice. Defaulting to SQLite."
            BACKEND="sqlite"
            ;;
    esac

    echo
    echo "🎯 Selected backend: $BACKEND"
}

check_docker
check_docker_compose
choose_backend

echo
echo "🚀 Starting BinCrypt with $BACKEND backend..."
echo "   Access at: http://localhost:8080"
echo

if docker compose version &> /dev/null 2>&1; then
    docker compose --profile "$BACKEND" up --build
else
    docker-compose --profile "$BACKEND" up --build
fi
