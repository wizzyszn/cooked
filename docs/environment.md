# Environment and configuration reference

`.env.example` is the canonical key list. Secrets belong in the deployment secret store or mode-0600 systemd environment file and must never be committed.

| Area | Keys | Notes |
|---|---|---|
| Server | `ENV`, `PORT`, timeout values, `TRUSTED_PROXIES` | Configure only actual reverse-proxy addresses. |
| PostgreSQL | `DB_*` | TLS is required outside trusted private networking; pool sizes must remain below server capacity. |
| JWT | `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, TTL values | Use separate random secrets and rotate through a controlled session-invalidating release. |
| OAuth | `GOOGLE_OAUTH_*` | Return URLs are exact-match allowlisted. |
| Email | `BREVO_*` | Optional activity email remains preference-gated; auth mail is transactional. |
| Objects | `S3_*` | Bucket stays private; only short-lived signed URLs are issued. |
| Worker | `WORKER_*` | Run independently under systemd with restart-on-failure. |
| Rewards/trends | `COOK_*`, `TREND_*`, `STREAK_REMINDER_HOUR` | Defaults are recorded in `.env.example`. |
| Abuse limits | `RATE_LIMIT_*` | Per-minute shared PostgreSQL limits by network and, after authentication, account. |

Production validation: load configuration with `go run ./cmd/migrate version`, verify no error, then check `/health/ready` before sending traffic.
