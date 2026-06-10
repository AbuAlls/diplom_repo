.PHONY: up down ps logs build test integration-test api-sh db-psql minio-open minio-ready api-minio-check front tunnel prod-up prod-down prod-logs

up:
	docker compose up -d --build

down:
	docker compose down

ps:
	docker compose ps

logs:
	docker compose logs -f --tail=200

build:
	docker compose build

test:
	docker compose run --rm api sh -lc "cd project && /usr/local/go/bin/go test ./..."

# Runs Postgres-backed integration tests (real SQL) against the running db container.
# Requires 'make up' first.  Covers GroupRepo, DocumentRepo SERIALIZABLE,
# PlanRepo.ListByOwners, and RecognitionQueueRepo.
integration-test:
	docker compose run --rm \
		-e POSTGRES_TEST_DSN="postgres://app:app@db:5432/app?sslmode=disable" \
		api sh -lc "cd project && /usr/local/go/bin/go test -tags integration -v ./internal/adapters/pgcore/..."

api-sh:
	docker compose exec api sh

# psql inside db container (no local psql needed)
db-psql:
	docker compose exec db psql -U app -d app

# Open MinIO Console in browser manually:
# http://localhost:9001  (login: minioadmin / minioadmin)
minio-open:
	@echo "MinIO Console: http://localhost:9001"

minio-ready:
	curl -v http://localhost:9000/minio/health/ready

api-minio-check:
	bash scripts/check_minio_api.sh

# Serve the prototype frontend over HTTP so Babel can fetch the .jsx files and
# the browser can reach the API. Open http://localhost:5500/index.html
front:
	python3 "diploma front end/serve.py"

# --- Public exposure -------------------------------------------------------

# Quick public HTTPS URL for testing on real devices. Requires the dev stack
# running ('make up') and cloudflared installed (brew install cloudflared).
# Prints a https://*.trycloudflare.com URL; point the mobile/front app at it.
# ngrok alternative:  ngrok http 8080
tunnel:
	cloudflared tunnel --url http://localhost:8080

# Production topology (Caddy + compiled API; db/minio internal). Needs .env.prod
# (cp .env.prod.example .env.prod). See docs/deployment.md.
prod-up:
	docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build

prod-down:
	docker compose -f docker-compose.prod.yml --env-file .env.prod down

prod-logs:
	docker compose -f docker-compose.prod.yml --env-file .env.prod logs -f --tail=200
