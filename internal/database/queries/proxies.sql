-- name: GetProxiesByListID :many
SELECT * FROM proxies
WHERE proxy_list_id = $1;
