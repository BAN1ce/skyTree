---
topic: SkyTree engineering conventions
audience: ai-agent,developer
doc_type: reference,explanation
dependencies:
  - Go style
keywords:
  - conventions
  - logging
  - error-handling
  - layering
---

# SkyTree Conventions

<!-- maintained-by: human+ai -->
<!-- ai-generated-start -->

## 1. Layering and Package Boundaries

- `cmd/`: process bootstrap only, avoid business logic.
- `app/`: composition root and lifecycle management.
- `internal/`: concrete business implementation.
- `pkg/`: contracts/reusable infra abstractions.
- `config/`: config schema + validation only.

Design preference: define explicit structs/interfaces for stable schemas; use `map` only when keys are truly dynamic or unknown at compile time.

## 2. Dependency Injection and Startup

- Use constructor injection and explicit dependencies.
- Register new long-running components in `app.registerComponents`.
- Ensure close path exists in `App.Close()` resource/component reverse-order shutdown.

## 3. Error Handling

- Return wrapped errors with context (`fmt.Errorf("...: %w", err)`).
- Avoid swallow-and-continue for critical init failures.
- Distinguish startup failure vs runtime failure channels.

## 4. Logging and Privacy

- Use structured logging (`zerolog`) and avoid dumping sensitive values.
- Never log credentials, tokens, private keys, auth secrets.
- For personal identifiers, ensure proper privacy handling in runtime logs.
- User-facing API errors should stay concise and generic.

## 5. API and Contract Evolution

- Keep existing HTTP/gRPC contracts stable by default.
- Add versioned route or additive fields instead of breaking changes.
- New config fields require:
  - `yaml/env` tags,
  - validation updates,
  - config examples/tests updates.

## 6. Concurrency and Lifecycle

- All long-running goroutines must be cancelable via context.
- Use bounded channels or explicit back-pressure for async paths.
- Component startup should define readiness semantics (critical vs one-shot).

## 7. Testing Convention

- Prefer package-local focused tests plus integration slice tests.
- Race-sensitive paths should be covered by `test-race-core`.
- Config safety must be validated with `test-configs`.

## 8. Documentation Convention For AI

- Keep docs under `docs/00-07` as single source for AI onboarding.
- Update metadata footer when docs are refreshed.
- When architecture changes, update at least:
  - `02-architecture.md`
  - `03-workflows.md`
  - `04-data-and-api.md`

<!-- ai-generated-end -->

<!-- PKB-metadata
last_updated: 2026-05-19
commit: 1cec350
updated_by: human+ai
doc_type: reference,explanation
-->
