-- Add mapping between logical models and physical channels
CREATE TABLE IF NOT EXISTS model_channels (
    model_name VARCHAR(100) NOT NULL, -- logical model name, e.g., 'gpt-4'
    channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    PRIMARY KEY (model_name, channel_id)
);

CREATE INDEX idx_model_channels_model ON model_channels(model_name);
