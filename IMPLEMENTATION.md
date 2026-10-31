# Implementation plan and status

1. Inspect and preserve the existing Go health endpoint, Compose service and database volume. **Complete.**
2. Add Goose migrations, the shared catalogue, authenticated user data and pgx persistence. **Complete.**
3. Implement cookie sessions, CSRF, nutrition snapshots, versioned corrections and private APIs. **Complete.**
4. Build and connect the five responsive React screens. **Complete.**
5. Verify calculation units, PostgreSQL isolation/persistence and desktop/mobile acceptance journeys. **Complete.**
6. Add local start/stop commands, configuration notes and explicit MVP limitations. **Complete.**

No public deployment or system-wide Go installation change is needed. See README.md for tests, data provenance and operating instructions.

## Personalized onboarding extension

1. Inspect existing schema, account/history counts and Docker volume; preserve existing data. **Complete; no reset needed.**
2. Add migration 005, versioned profile storage and auditable calculation/decision snapshots. **Complete.**
3. Implement and document the independent recommendation service, eligibility checks, unit normalization and conservative activity/goal policies. **Complete.**
4. Add the guided wizard, result review, explicit goal choices and recalculation from Profile / Goals. **Complete.**
5. Verify unit, actual legacy migration, authorization, stale-write and desktop/mobile journeys. **Complete.**

See ONBOARDING.md for exact formulas, source references, example outputs and limitations.
