CREATE TABLE IF NOT EXISTS call_logs (
    request_id      String,
    user_id         UInt64,
    api_key_id      UInt64,
    customer_tier   LowCardinality(String),  -- self/standard/flagship
    model_logical   LowCardinality(String),  -- logical model name
    channel_id      UInt32,
    provider        LowCardinality(String),
    region          LowCardinality(String),
    ts_start        DateTime64(3),
    ts_first_token  DateTime64(3),
    ts_end          DateTime64(3),
    status_code     UInt16,
    is_failover     UInt8,
    failover_chain  Array(UInt32),  -- failover path
    input_tokens    UInt32,
    output_tokens   UInt32,
    cost_usd        Decimal(10,6),
    revenue_cny     Decimal(10,4),
    margin_cny      Decimal(10,4),
    error_type      LowCardinality(String),
    error_message   String,
    moderation_hit  UInt8,
    client_ip       IPv6,           -- client IP for compliance
    user_agent      String
) ENGINE = MergeTree
PARTITION BY toYYYYMM(ts_start)
ORDER BY (user_id, ts_start, request_id)
TTL ts_start + INTERVAL 90 DAY;
