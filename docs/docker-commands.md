# Docker command cookbook

All commands assume the repository root (where `docker-compose.yml` lives).

## Lifecycle

```bash
# Build images and start in background
docker compose up -d --build

# Stop containers (keep volumes)
docker compose down

# Stop and remove named volumes (deletes Qdrant data volume)
docker compose down -v
```

## Logs and status

```bash
docker compose ps
docker compose logs -f claudia
docker compose logs -f litellm
docker compose logs -f --tail=200 claudia litellm
```

## Shell inside containers

```bash
docker compose exec claudia sh
docker compose exec litellm sh
```

## Rebuild after code changes

```bash
docker compose build claudia --no-cache
docker compose up -d claudia
```

## Health from the host

```bash
curl -sS http://localhost:3000/health
curl -sS http://localhost:4000/health
```

## Volumes

- **`qdrant_data`** — persistent Qdrant storage (see `docker compose volume ls`).

Inspect:

```bash
docker volume inspect qdrant_data
```
