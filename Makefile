.PHONY: up down ps logs build test api-sh db-psql minio-open minio-ready api-minio-check

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
