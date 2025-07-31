#!/bin/bash

echo "🚀 Starting BinCrypt with Firebase Emulator..."

# Build if needed
if [ ! -f "bincrypt" ]; then
    echo "📦 Building BinCrypt..."
    cd src && go build -o ../bincrypt . && cd ..
fi

# Start Firebase emulator in background
echo "📦 Starting Firebase Storage Emulator..."
firebase emulators:start --only storage &
EMULATOR_PID=$!

# Wait for emulator to start
echo "⏳ Waiting for emulator to start..."
sleep 3

# Load environment variables and run the app
echo "🔧 Starting BinCrypt server..."
set -a
source .env
set +a
./bincrypt &
SERVER_PID=$!

echo ""
echo "✅ BinCrypt is running!"
echo "🌐 Main app: http://localhost:8080"
echo "📊 Firebase Emulator UI: http://localhost:4000"
echo ""
echo "Press Ctrl+C to stop all services"

# Handle shutdown
trap "echo '🛑 Shutting down...'; kill $EMULATOR_PID $SERVER_PID 2>/dev/null; exit" INT TERM

# Keep script running
wait