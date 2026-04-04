# TypeScript gateway (legacy)

This directory holds the **original Node/Fastify** Claudia gateway used by **`docker compose`** for the **`claudia`** service (`Dockerfile`).

**Status:** The **Go** implementation in **`cmd/claudia/`** is the **recommended** runtime for new deployments and matches the same YAML configuration. This TypeScript stack remains in the repo for the existing Compose image and for comparison until the Docker image is switched to Go (follow-up).

See [docs/operator-migration-to-go.md](../docs/operator-migration-to-go.md) and [docs/go-bifrost-migration-plan.md](../docs/go-bifrost-migration-plan.md) Phase 6.
