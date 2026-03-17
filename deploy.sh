#!/bin/bash
set -e

MSG="${1:?Usage: ./deploy.sh \"commit message\"}"

# Commit staged changes
git add -A
git commit -m "$MSG

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
git push

# Deploy to monitors server
sshpass -p 'bingus123' ssh -o StrictHostKeyChecking=no monitors \
  "cd /opt/go-monitor && git pull && docker compose -f docker-compose.prod.yml up -d --build"
