# M6 — Cook Mode, sessions, XP, streaks, and analytics

## Delivered

- Resumable Cook Sessions bound to an exact immutable Recipe Version, with one active session per user/version.
- Idempotent Step visits and persisted start, pause, resume, and reset timer state based on target timestamps.
- Explicit abandonment and transactional completion after every version Step has been visited.
- Optional owned, processed Cook Session photos; timers and photos are never completion requirements.
- Configurable 50 base XP, 10 photo XP, 25 first-Dish XP, and five-rewarded-session daily cap defaults.
- Per-Recipe/local-date rewards, first-Dish decisions, daily caps, and zero-XP outcomes recorded in append-only ledgers.
- Daily streak advancement using snapshotted IANA timezone/local date, plus a worker projection that expires stale cached streaks.
- Allowlisted, pseudonymous/versioned client analytics and transactional server-authoritative activation/completion events.
- Staff product metrics for activation, Cook Mode conversion, Review-eligibility placeholder, matured cohorts, and seven-day repeat cooking.
- Analytics identity severance during account anonymization.

## API routes

- `POST /api/v1/cook-sessions`
- `GET /api/v1/cook-sessions`
- `GET /api/v1/cook-sessions/active`
- `GET /api/v1/cook-sessions/{id}`
- `POST /api/v1/cook-sessions/{id}/steps/{stepId}/visit`
- `PUT /api/v1/cook-sessions/{id}/steps/{stepId}/timer`
- `POST /api/v1/cook-sessions/{id}/abandon`
- `POST /api/v1/cook-sessions/{id}/complete`
- `POST /api/v1/analytics/events`
- `GET /api/v1/staff/metrics/product`

## Verification

- Concurrent completion retries create one completed state, XP set, streak mutation, activation event, and completion event.
- Timer deadlines and Step visits restore correctly after replacing the service instance.
- Tests cover missing Steps, same-Recipe/day, same-Dish, first-Dish, photo bonus, sixth rewarded session, zero-XP ledger decisions, same-day repeats, missed days, timezone changes, DST boundaries, and stale-streak projection.
- Product metrics reconcile both zero-return and positive seven-day-return fixtures directly from Cook Sessions.
- Migration `000013` has a real PostgreSQL up/down/reapply lifecycle test.

## X thread draft

1/ I’m building Cooked because I don’t know how to cook—and following a long recipe on my phone usually turns into scrolling with messy hands, losing my place, and guessing whether I’m actually making progress.

This milestone turns a recipe into a guided cooking session. 🧵

2/ Cook Mode now remembers the exact version of the recipe I started, which Steps I’ve visited, and every timer I created. I can close the app, switch devices, or restart the backend and continue from the same state.

Timers use persisted deadlines, not a browser tab that has to stay awake.

3/ Finishing is deliberate: I must visit every Step and explicitly say “I finished cooking.” A timer ending does not complete the meal for me, and adding a photo is optional.

That keeps progress useful without pretending the software can verify what happened in my kitchen.

4/ To make practice feel rewarding, Cooked now awards XP: 50 for a qualifying cook, +10 for a photo, and +25 the first time I cook any recipe for a Dish.

The limits matter too: one reward per Recipe each local day and no more than five XP-bearing sessions per day.

5/ Every award—and every zero-XP cap decision—is written to an immutable ledger. Completion, XP, streaks, and analytics happen in one transaction, so even concurrent retries cannot duplicate rewards.

6/ Daily streaks respect the cook’s IANA timezone. I tested DST changes, timezone updates, same-day repeats, consecutive days, and missed days. Historical completion dates never shift when a profile timezone changes.

7/ I also added first-party product metrics built from server-authoritative sessions: activation, Cook Mode conversion, Review eligibility, and seven-day repeat cooking.

No email addresses or arbitrary free text are accepted into analytics properties.

8/ Cooked is becoming less like a recipe archive and more like the patient cooking companion I need: one Step at a time, able to remember where I stopped, and giving me a reason to come back tomorrow.
