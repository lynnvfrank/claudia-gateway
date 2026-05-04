# Log presentation layer — acceptance (v0.2.1)

Automated checks and manual acceptance for [`log-presentation-layer.plan.md`](log-presentation-layer.plan.md) phases implemented on this branch.

## Phase A — Shape detection + headline (UI)

**Automated**

- `go test ./internal/server/... -run 'UILogs|Logging'` — UI routes and logging middleware tests pass.

**Manual**

1. Sign in to `/ui/logs` (or Desktop → Logs).
2. Choose **View → Summary** on a gateway with traffic.
3. **Accept:** HTTP lines show a **colored status pill**, **method**, emphasized **path**, and **duration** without opening “All fields”.
4. Choose **View → Detailed**.
5. **Accept:** Every field still available (full props table per row); no loss vs prior behavior.

## Phase B — Correlation IDs + `service` tag

**Automated**

- `go test ./internal/platform/requestid/...` — request id middleware and validation.
- `go test ./internal/server/... -run 'LoggingMiddleware|ConversationID'` — access logs include `request_id` and `service=gateway`; chat conversation id helper behaves.

**Manual**

1. Call `POST /v1/chat/completions` with header `X-Claudia-Conversation-Id: my-thread-1` (valid charset per middleware).
2. **Accept:** Gateway log lines for that request include the same `conversation_id` and `request_id` on `chat completion request` / upstream relay lines.
3. Optional: send `X-Request-ID: abc` on any request; **Accept:** `http response` line echoes `request_id=abc` when valid.

## Phase C — Indexer run narrative

**Automated**

- `go test ./internal/indexer/... -run IndexRunID` — `GatewayClient` sends `X-Claudia-Index-Run-Id` when set.

**Manual**

1. Run `claudia-index` against a dev gateway; inspect stderr or `/ui/logs` (if indexer output is teed).
2. **Accept:** Lines include `indexer.run.start`, `indexer.run.progress` (initial scan), and `indexer.run.done`; `index_run_id` repeats on lines.
3. Ingest with the same run: **Accept:** Gateway `ingest complete` / failure logs include `index_run_id` when the client sent the header.

## Phase D — Conversations + Subsystems panels

**Manual**

1. Generate chat traffic (Phase B).
2. **View → Conversations**.
3. **Accept:** Cards grouped by **principal** + **conversation_id**; last events listed; **issues** badge when WARN/ERROR or HTTP ≥400 on correlated fields.
4. **View → Subsystems**.
5. **Accept:** **gateway**, **qdrant**, **bifrost**, **indexer** buckets show line counts and recent `msg` lines; error tint when failures present.

## Regression

- `go test ./... -short`
- Confirm `wrapResponse` access logs reflect real **statusCode** (not stuck at 200) for non-200 handlers.
