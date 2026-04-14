CREATE TABLE webhook_delivery_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type      VARCHAR(50) NOT NULL,
    payload         JSONB NOT NULL,
    response_status INTEGER,
    response_body   TEXT,
    attempt         INTEGER NOT NULL DEFAULT 1,
    success         BOOLEAN NOT NULL DEFAULT false,
    error_message   TEXT,
    delivered_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_delivery_sub ON webhook_delivery_log(subscription_id);
CREATE INDEX idx_webhook_delivery_time ON webhook_delivery_log(delivered_at);
