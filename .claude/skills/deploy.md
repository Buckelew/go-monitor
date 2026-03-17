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

4. Deploy to production server:
   ```bash
   sshpass -p 'bingus123' ssh -o StrictHostKeyChecking=no monitors \
     "cd /opt/go-monitor && git pull && docker compose -f docker-compose.prod.yml up -d --build"
   ```
   This takes ~60-90s (Go compile on VPS). Run it and wait for completion.

5. Verify the app started:
   ```bash
   sshpass -p 'bingus123' ssh -o StrictHostKeyChecking=no monitors \
     "docker logs go-monitor-app-1 --tail 5"
   ```

6. Report success or failure to the user.

## Notes

- The build step on the VPS can be slow (~60s) and memory-intensive. The VPS has limited RAM so the Go compiler may cause swapping.
- If the migration container fails, check `docker logs go-monitor-migrate-1` for details. Common issue: dirty migration state, fix with `UPDATE schema_migrations SET dirty = false`.
- `CREATE INDEX CONCURRENTLY` cannot be used in migrations (they run inside transactions). Use `CREATE INDEX` instead.
