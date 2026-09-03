# Backend security review — M9

Reviewed 3 September 2026 against the FRD threat boundaries.

| Boundary | Evidence and disposition |
|---|---|
| Passwords and credentials | Argon2id passwords; reset, verification, refresh, OAuth state, PKCE verifier, and login codes use hashed or single-use records with expiries. Raw secrets are excluded from structured logs. |
| JWT and sessions | Separate access/refresh secrets and TTLs, refresh-family rotation/reuse handling, logout-all revocation, and current account-state loading on authenticated requests. |
| OAuth | PKCE, hashed state/nonce, exact return-URL allowlist, two-minute one-use exchange code; no Cooked tokens in redirects. |
| RBAC/IDOR | Current user is loaded server-side; verified and staff gates are route-specific. Recipe versions, Cook Sessions, Reviews, reports, notifications, roles, and media re-check ownership/access in services or database queries. |
| Media | Private bucket, expiring signatures, owner/purpose binding, byte/dimension/pixel limits, decode-versus-declared MIME verification, metadata-stripping re-encode, and quarantine before approval. |
| Integrity | Immutable published versions and append-only XP/streak/audit triggers; transactional publish, completion, Review aggregate, moderation threshold, and merge operations. |
| Abuse | Shared PostgreSQL fixed-window policies cover network traffic and account-scoped contribution commands. XP caps remain independent. |
| Privacy/logging | Route-template metrics avoid IDs/query strings; analytics properties are allowlisted and exclude free text/email; inaccessible content uses non-disclosing 404s. |

No open critical or high backend defect was identified in the reviewed scope. Remaining external controls: TLS/HSTS and request-size limits at the reverse proxy, secret-manager rotation, private metrics-network access, database encryption/backups, provider IAM, dependency monitoring, and frontend WCAG/browser verification.
