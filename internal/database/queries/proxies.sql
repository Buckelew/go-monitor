-- name: GetProxiesByListID :many
SELECT * FROM proxies
WHERE proxy_list_id = $1;

-- name: GetProxyLists :many
SELECT
  pl.id, pl.name, pl.created_at,
  COALESCE(pc.proxy_count, 0)::int AS proxy_count
FROM proxy_lists pl
LEFT JOIN (
  SELECT proxy_list_id, COUNT(*) AS proxy_count FROM proxies GROUP BY proxy_list_id
) pc ON pc.proxy_list_id = pl.id
ORDER BY pl.created_at DESC;

-- name: CreateProxyList :one
INSERT INTO proxy_lists (name) VALUES ($1) RETURNING *;

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
SELECT platform FROM proxy_list_platforms WHERE proxy_list_id = $1;

-- name: SetProxyListPlatform :exec
INSERT INTO proxy_list_platforms (proxy_list_id, platform)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: DeleteProxyListPlatforms :exec
DELETE FROM proxy_list_platforms WHERE proxy_list_id = $1;

-- name: CountProxiesByPlatform :one
SELECT COUNT(DISTINCT p.id)
FROM proxies p
JOIN proxy_list_platforms plp ON plp.proxy_list_id = p.proxy_list_id
WHERE plp.platform = $1;

-- name: GetProxyListByPlatform :one
SELECT pl.* FROM proxy_lists pl
JOIN proxy_list_platforms plp ON plp.proxy_list_id = pl.id
WHERE plp.platform = $1
LIMIT 1;
