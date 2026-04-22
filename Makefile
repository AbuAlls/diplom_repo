.PHONY: up down ps logs build test api-sh db-psql nats-sub nats-pub minio-open

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
	docker compose run --rm api go test ./...

api-sh:
	docker compose exec api sh

# psql inside db container (no local psql needed)
db-psql:
	docker compose exec db psql -U app -d app

# Quick NATS debugging (no local nats tools needed)
# nats-sub:
# 	docker compose exec nats sh -lc "nats --version >/dev/null 2>&1 || echo 'no nats cli in image'; echo ' use http://localhost:8222 for monitoring'"

# Open MinIO Console in browser manually:
# http://localhost:9001  (login: minioadmin / minioadmin)
minio-open:
	@echo "MinIO Console: http://localhost:9001"

minio-ready:
	curl -v http://localhost:9000/minio/health/ready

nats-ready:
	curl http://localhost:8222/varz