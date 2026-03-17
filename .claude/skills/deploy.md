---
name: deploy
description: Build, commit, push, and deploy go-monitor to the production server. Use when the user says "deploy", "ship it", "push and deploy", "update server", or wants changes live on monitors.pacificaio.com.
user_invocable: true
---

# Deploy go-monitor

Deploys the current changes to the production server at `monitors.pacificaio.com`.

## Steps

1. Run `go build ./...` and `go vet ./...` to verify the build is clean. Stop if either fails.

2. Check `git status` for uncommitted changes. If there are changes:
   - Stage only the modified/new files explicitly (no `git add -A`)
   - Commit with a descriptive message. If the user provided a message via `/deploy <message>`, use that. Otherwise, infer from the diff.
   - Append `Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>`

3. Push to remote: `git push`

4. GitHub Actions automatically builds the Docker image and deploys. Monitor the run:
   ```bash
   gh run list --limit 1
   gh run watch  # if you want to wait for it
   ```

5. If the user needs immediate deployment or CI is slow, manual deploy:
   ```bash
   sshpass -p 'bingus123' ssh -o StrictHostKeyChecking=no monitors \
     "cd /opt/go-monitor && git pull && docker compose -f docker-compose.prod.yml pull app && docker compose -f docker-compose.prod.yml up -d"
   ```

6. Verify the app started:
   ```bash
   sshpass -p 'bingus123' ssh -o StrictHostKeyChecking=no monitors \
     "docker logs go-monitor-app-1 --tail 5"
   ```

## Notes

- The Docker image is built in GitHub Actions (not on the VPS) to avoid OOM issues.
- Image is pushed to ghcr.io/buckelew/go-monitor
- The VPS just pulls the pre-built image — no compilation needed.
- If the migration container fails, check `docker logs go-monitor-migrate-1`. Common issue: dirty migration state, fix with `UPDATE schema_migrations SET dirty = false`.
- `CREATE INDEX CONCURRENTLY` cannot be used in migrations (they run inside transactions). Use `CREATE INDEX` instead.
