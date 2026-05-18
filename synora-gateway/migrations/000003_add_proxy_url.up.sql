-- Add proxy_url to channels table for egress proxy routing
ALTER TABLE channels ADD COLUMN IF NOT EXISTS proxy_url TEXT;
