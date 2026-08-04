#!/bin/bash
# ─────────────────────────────────────────────
# HaloStory Worker Deployment Script (VPS 2)
# ─────────────────────────────────────────────
# Jalankan script ini di VPS 2 untuk deploy worker.
# Prerequisites: git, docker, docker compose
# ─────────────────────────────────────────────

set -euo pipefail

REPO_URL="https://github.com/your-username/halostory_go_backend.git"
DEPLOY_DIR="/opt/halostory-worker"
BRANCH="main"

echo "=== HaloStory Worker Deployment ==="
echo ""

# Check prerequisites
for cmd in git docker; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "Error: $cmd tidak ditemukan. Install dulu."
        exit 1
    fi
done

# Check docker compose
if ! docker compose version &>/dev/null; then
    echo "Error: docker compose tidak ditemukan."
    exit 1
fi

# Clone or update repo
if [ -d "$DEPLOY_DIR" ]; then
    echo "Updating repo di $DEPLOY_DIR..."
    cd "$DEPLOY_DIR"
    git pull origin "$BRANCH"
else
    echo "Cloning repo ke $DEPLOY_DIR..."
    git clone --branch "$BRANCH" --depth 1 "$REPO_URL" "$DEPLOY_DIR"
    cd "$DEPLOY_DIR"
fi

# Setup .env if not exists
if [ ! -f .env ]; then
    echo ""
    echo "Membuat .env dari template..."
    cp .env.worker.example .env
    echo ""
    echo "⚠️  Edit .env sebelum menjalankan worker!"
    echo "    nano $DEPLOY_DIR/.env"
    echo ""
    echo "Yang perlu diisi:"
    echo "  DB_HOST     → IP VPS 1 (MySQL)"
    echo "  DB_PASSWORD → password MySQL worker"
    echo "  REDIS_HOST  → IP VPS 1 (Redis)"
    echo "  REDIS_PASSWORD → password Redis"
    echo ""
    exit 0
fi

# Build and start worker
echo ""
echo "Building dan menjalankan worker..."
docker compose -f docker-compose.worker.yml up -d --build

echo ""
echo "=== Worker berhasil dijalankan! ==="
echo ""
echo "Status:    docker compose -f docker-compose.worker.yml ps"
echo "Logs:      docker compose -f docker-compose.worker.yml logs -f"
echo "Restart:   docker compose -f docker-compose.worker.yml restart"
echo "Stop:      docker compose -f docker-compose.worker.yml down"
echo ""
