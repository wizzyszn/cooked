CREATE TABLE rate_limit_buckets (
    policy VARCHAR(64) NOT NULL,
    subject_type VARCHAR(16) NOT NULL CHECK(subject_type IN ('network','account')),
    subject_key VARCHAR(255) NOT NULL,
    window_started_at TIMESTAMPTZ NOT NULL,
    request_count INT NOT NULL CHECK(request_count > 0),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY(policy,subject_type,subject_key)
);
CREATE INDEX idx_rate_limit_expiry ON rate_limit_buckets(expires_at);
