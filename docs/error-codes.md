# API error-code catalog

All errors use the standard API envelope and include a request ID. Clients should branch on `error.code`, not message text.

| Code | HTTP | Meaning |
|---|---:|---|
| `VALIDATION_ERROR`, `BAD_REQUEST` | 400 | Input is malformed or violates a request rule. |
| `UNAUTHORIZED`, `INVALID_TOKEN`, `TOKEN_EXPIRED` | 401 | Authentication is absent, invalid, or expired. |
| `FORBIDDEN`, `EMAIL_NOT_VERIFIED`, `REVIEW_NOT_ELIGIBLE` | 403 | The account lacks permission, verification, or exact-version eligibility. |
| `NOT_FOUND` | 404 | The resource is absent or intentionally undisclosed. |
| `CONFLICT_ERROR`, `EMAIL_TAKEN`, `USERNAME_TAKEN`, `SIMILAR_DISHES_FOUND` | 409 | Current state conflicts with the command. |
| `TOO_MANY_REQUESTS` | 429 | A shared route/network/account policy rejected the request. |
| `INTERNAL_SERVER_ERROR` | 500 | Unexpected server failure; use the request ID for investigation. |
| `SERVICE_UNAVAILABLE`, `NOT_READY` | 503 | A dependency or required migration is unavailable. |
| `DISH_TAXONOMY_REQUIRED`, `INVALID_COUNTRY_CODE`, `RECIPE_INCOMPLETE`, `STEPS_INCOMPLETE`, `MEDIA_NOT_READY` | 400/409 | Domain validation described by the code failed. |
| `INVALID_RESET`, `RESET_OTP_EXPIRED`, `USED_OTP` | 400/401 | Password-reset credential is invalid, expired, or consumed. |

Authentication responses deliberately avoid disclosing whether an email address exists. Inaccessible private, deleted, hidden, and removed content returns `NOT_FOUND`.
