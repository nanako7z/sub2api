-- Add cache_hit_requests column to dashboard aggregation tables
-- for platform-level cache hit rate trend tracking.

ALTER TABLE usage_dashboard_hourly
    ADD COLUMN IF NOT EXISTS cache_hit_requests BIGINT NOT NULL DEFAULT 0;

ALTER TABLE usage_dashboard_daily
    ADD COLUMN IF NOT EXISTS cache_hit_requests BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN usage_dashboard_hourly.cache_hit_requests IS 'Number of requests where cache_read_tokens > 0 in this hour bucket.';
COMMENT ON COLUMN usage_dashboard_daily.cache_hit_requests IS 'Number of requests where cache_read_tokens > 0 in this day bucket.';
