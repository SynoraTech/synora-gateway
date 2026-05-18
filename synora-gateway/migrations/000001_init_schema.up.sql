-- Users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'user', -- admin, user
    tier VARCHAR(50) DEFAULT 'standard', -- flagship, standard, self
    dedicated_channel_group_id INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- API Keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100),
    key_hash VARCHAR(255) UNIQUE NOT NULL, -- sk-synora-... hash
    key_prefix VARCHAR(10) NOT NULL, -- for recognition
    status VARCHAR(20) DEFAULT 'active', -- active, paused, revoked
    daily_spend_limit DECIMAL(10, 4) DEFAULT 100.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP WITH TIME ZONE
);

-- Channels table
CREATE TABLE IF NOT EXISTS channels (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL, -- openai, anthropic, gemini, azure, aws, etc.
    endpoint TEXT NOT NULL,
    credential_ref TEXT NOT NULL, -- Reference to encrypted file/vault
    region VARCHAR(50),
    weight INTEGER DEFAULT 1,
    priority INTEGER DEFAULT 1, -- Lower is higher
    customer_tier_allowed VARCHAR(50) DEFAULT 'standard', -- flagship, standard, all
    health_score INTEGER DEFAULT 100,
    state VARCHAR(20) DEFAULT 'active', -- active, degraded, circuit_open, disabled
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Pricing table
CREATE TABLE IF NOT EXISTS pricing (
    id SERIAL PRIMARY KEY,
    model_name VARCHAR(100) NOT NULL, -- logical model name
    provider VARCHAR(50) NOT NULL,
    input_price_per_1k DECIMAL(10, 6) NOT NULL,
    output_price_per_1k DECIMAL(10, 6) NOT NULL,
    currency VARCHAR(10) DEFAULT 'USD',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(model_name, provider)
);

-- Wallets table
CREATE TABLE IF NOT EXISTS wallets (
    id SERIAL PRIMARY KEY,
    user_id INTEGER UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance DECIMAL(15, 4) DEFAULT 0.0,
    currency VARCHAR(10) DEFAULT 'CNY',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Wallet Transactions (Audit trail for balance changes)
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id SERIAL PRIMARY KEY,
    wallet_id INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    amount DECIMAL(15, 4) NOT NULL,
    type VARCHAR(20) NOT NULL, -- recharge, consume, refund
    request_id VARCHAR(100), -- link to ClickHouse log if consume
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_channels_provider ON channels(provider);
CREATE INDEX idx_pricing_model ON pricing(model_name);
