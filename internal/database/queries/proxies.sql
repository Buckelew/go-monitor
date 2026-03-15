-- name: GetProxiesByListID :many
SELECT * FROM proxies
WHERE proxy_list_id = $1;

-- name: GetProxyLists :many
SELECT
  pl.id, pl.name, pl.platform, pl.created_at,
  COALESCE(pc.proxy_count, 0)::int AS proxy_count
FROM proxy_lists pl
LEFT JOIN (
  SELECT proxy_list_id, COUNT(*) AS proxy_count FROM proxies GROUP BY proxy_list_id
) pc ON pc.proxy_list_id = pl.id
ORDER BY pl.created_at DESC;

-- name: CreateProxyList :one
INSERT INTO proxy_lists (name, platform) VALUES ($1, $2) RETURNING *;

-- name: DeleteProxyList :exec
DELETE FROM proxy_lists WHERE id = $1;

-- name: UpdateProxyListName :exec
UPDATE proxy_lists SET name = $2 WHERE id = $1;

-- name: CreateProxy :exec
INSERT INTO proxies (proxy_list_id, host, port, username, password)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteProxiesByListID :exec
DELETE FROM proxies WHERE proxy_list_id = $1;

-- name: GetPlatformsByProxyListID :many
SELECT DISTINCT t.platform FROM tasks t WHERE t.proxy_list_id = $1;

-- name: CountProxiesByPlatform :one
SELECT COUNT(DISTINCT p.id)
FROM proxies p
JOIN proxy_lists pl ON pl.id = p.proxy_list_id
WHERE pl.platform = $1;

-- name: GetProxyListByPlatform :one
SELECT * FROM proxy_lists WHERE platform = $1 LIMIT 1;
