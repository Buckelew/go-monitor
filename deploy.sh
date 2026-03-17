#!/bin/bash
set -e

MSG="${1:?Usage: ./deploy.sh \"commit message\"}"

# Commit and push — CI handles the rest
git add -A
git commit -m "$MSG

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
git push

echo "Pushed. GitHub Actions will build and deploy."
echo "Watch: gh run watch"
