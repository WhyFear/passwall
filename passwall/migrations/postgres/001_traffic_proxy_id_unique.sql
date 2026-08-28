\set ON_ERROR_STOP on

-- Run this migration while all Traffic writers are paused. The dedupe commit and
-- CREATE UNIQUE INDEX CONCURRENTLY cannot otherwise close the insertion window.
\echo 'Traffic writers must be paused for this migration'

SELECT EXISTS (
    SELECT 1
    FROM pg_index AS i
    JOIN pg_class AS t ON t.oid = i.indrelid
    JOIN pg_attribute AS a ON a.attrelid = t.oid AND a.attname = 'proxy_id'
    WHERE t.oid = to_regclass('traffic_statistics')
      AND i.indisunique
      AND i.indisvalid
      AND i.indisready
      AND i.indnkeyatts = 1
      AND i.indkey[0] = a.attnum
      AND i.indpred IS NULL
      AND i.indexprs IS NULL
) AS traffic_proxy_id_unique_ready \gset

\if :traffic_proxy_id_unique_ready
\echo 'traffic_statistics(proxy_id) already has a valid unique index'
\else
BEGIN;
LOCK TABLE traffic_statistics IN SHARE ROW EXCLUSIVE MODE;

WITH duplicate_totals AS (
    SELECT proxy_id,
           MIN(id) AS keep_id,
           SUM(download_total) AS download_total,
           SUM(upload_total) AS upload_total,
           MIN(created_at) AS created_at,
           MAX(updated_at) AS updated_at
    FROM traffic_statistics
    GROUP BY proxy_id
    HAVING COUNT(*) > 1
)
UPDATE traffic_statistics AS traffic
SET download_total = totals.download_total,
    upload_total = totals.upload_total,
    created_at = totals.created_at,
    updated_at = totals.updated_at
FROM duplicate_totals AS totals
WHERE traffic.id = totals.keep_id;

WITH keepers AS (
    SELECT proxy_id, MIN(id) AS keep_id
    FROM traffic_statistics
    GROUP BY proxy_id
    HAVING COUNT(*) > 1
)
DELETE FROM traffic_statistics AS traffic
USING keepers
WHERE traffic.proxy_id = keepers.proxy_id
  AND traffic.id <> keepers.keep_id;

COMMIT;

DROP INDEX CONCURRENTLY IF EXISTS idx_traffic_statistics_proxy_id_unique;
CREATE UNIQUE INDEX CONCURRENTLY idx_traffic_statistics_proxy_id_unique
    ON traffic_statistics (proxy_id);
\endif

SELECT EXISTS (
    SELECT 1
    FROM pg_index AS i
    JOIN pg_class AS idx ON idx.oid = i.indexrelid
    JOIN pg_class AS t ON t.oid = i.indrelid
    WHERE t.oid = to_regclass('traffic_statistics')
      AND idx.relname = 'idx_traffic_proxy_id'
      AND NOT i.indisunique
) AS traffic_proxy_id_old_plain_index_exists \gset

\if :traffic_proxy_id_old_plain_index_exists
DROP INDEX CONCURRENTLY idx_traffic_proxy_id;
\endif

SELECT EXISTS (
    SELECT 1
    FROM pg_index AS i
    JOIN pg_class AS t ON t.oid = i.indrelid
    JOIN pg_attribute AS a ON a.attrelid = t.oid AND a.attname = 'proxy_id'
    WHERE t.oid = to_regclass('traffic_statistics')
      AND i.indisunique
      AND i.indisvalid
      AND i.indisready
      AND i.indnkeyatts = 1
      AND i.indkey[0] = a.attnum
      AND i.indpred IS NULL
      AND i.indexprs IS NULL
) AS traffic_proxy_id_unique_ready \gset

\if :traffic_proxy_id_unique_ready
\echo 'traffic_statistics(proxy_id) unique index is ready'
\else
\echo 'traffic_statistics(proxy_id) unique index validation failed'
\quit 1
\endif
