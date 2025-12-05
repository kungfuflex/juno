#!/bin/bash
set -e

SNAPSHOT_URL="https://juno-snapshots.nethermind.io/files/mainnet/latest"
DEFAULT_DB_PATH="/data/juno-mainnet"
DB_PATH="${JUNO_DB_PATH:-$DEFAULT_DB_PATH}"

echo "Juno Mainnet Snapshot Downloader"
echo "================================"
echo "Target directory: $DB_PATH"
echo ""

# Check for zstd
if ! command -v zstd &> /dev/null; then
    echo "Error: zstd is not installed."
    echo "Install it with:"
    echo "  Ubuntu/Debian: sudo apt-get install zstd"
    echo "  macOS: brew install zstd"
    echo "  RHEL/CentOS/Fedora: sudo dnf install zstd"
    exit 1
fi

# Check for curl
if ! command -v curl &> /dev/null; then
    echo "Error: curl is not installed."
    exit 1
fi

# Get snapshot size
echo "Fetching snapshot size..."
SNAPSHOT_SIZE=$(curl -sI -L "$SNAPSHOT_URL" | grep -i content-length | tail -1 | awk '{print $2}' | tr -d '\r')
if [ -n "$SNAPSHOT_SIZE" ]; then
    SIZE_GB=$(echo "scale=2; $SNAPSHOT_SIZE / 1024 / 1024 / 1024" | bc)
    echo "Snapshot size: ${SIZE_GB} GB"
fi
echo ""

# Check if directory exists and has data
if [ -d "$DB_PATH" ] && [ "$(ls -A $DB_PATH 2>/dev/null)" ]; then
    echo "Warning: Directory $DB_PATH already contains data."
    read -p "Do you want to remove existing data and download fresh snapshot? (y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        echo "Aborted."
        exit 0
    fi
    echo "Removing existing data..."
    rm -rf "${DB_PATH:?}"/*
fi

# Create directory
mkdir -p "$DB_PATH"

# Check available disk space
AVAILABLE_SPACE=$(df -B1 "$DB_PATH" | tail -1 | awk '{print $4}')
REQUIRED_SPACE=$((250 * 1024 * 1024 * 1024))  # ~250GB recommended

if [ "$AVAILABLE_SPACE" -lt "$REQUIRED_SPACE" ]; then
    AVAILABLE_GB=$(echo "scale=2; $AVAILABLE_SPACE / 1024 / 1024 / 1024" | bc)
    echo "Warning: Low disk space. Available: ${AVAILABLE_GB} GB, Recommended: 250 GB"
    read -p "Continue anyway? (y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        echo "Aborted."
        exit 0
    fi
fi

echo "Downloading and extracting snapshot to $DB_PATH..."
echo "This may take a while depending on your internet connection."
echo ""

# Download and extract in one step (no double disk space needed)
curl -L "$SNAPSHOT_URL" | zstd -d | tar -xvf - -C "$DB_PATH"

echo ""
echo "Snapshot downloaded and extracted successfully!"
echo "You can now start Juno with:"
echo "  docker-compose -f docker-compose.mainnet.yaml up -d"
