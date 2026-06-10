# Deployment & public exposure

The Go API serves **plain HTTP on `:8080`** and is internal-only by default. To
reach it from a frontend or mobile app over **HTTPS**, put a TLS terminator in
front of it. Two paths, both delivered here:

- **Tunnel** — instant public HTTPS URL for testing on real devices. Zero cost,
  zero server.
- **Reverse proxy on a VPS** — stable, always-on HTTPS with a real domain.

No application code changes are needed for either: TLS terminates at the
proxy/tunnel, and the API already (a) streams file downloads through itself
(no internal MinIO URLs leak to clients), (b) uses `Authorization: Bearer` auth
so wildcard CORS is safe, and (c) exposes an unauthenticated `GET /healthz` for
upstream health checks.

---

## A. Quick tunnel (test on a real device now)

Best for: pointing a phone or a teammate's browser at your laptop in seconds.

```sh
make up                 # dev stack on :8080
brew install cloudflared # one-time (macOS)
make tunnel             # prints https://<random>.trycloudflare.com
```

Then:

```sh
curl https://<random>.trycloudflare.com/healthz          # {"status":"ok"}
API_BASE=https://<random>.trycloudflare.com bash scripts/check_minio_api.sh
```

Point the mobile app / frontend `API_BASE` at that URL.

**ngrok alternative** (already referenced in `docs/arhitecture.md`):

```sh
ngrok http 8080
```

Caveats: the free quick-tunnel URL **changes every run**, and **your machine must
stay on**. For a stable URL, use a VPS (section C) or a named cloudflared/ngrok
tunnel with an account.

---

## B. Local production simulation (Caddy on :80, no domain)

Best for: verifying the production topology (Caddy → compiled API, db/minio
internal) on your machine before touching a server.

```sh
cp .env.prod.example .env.prod      # leave SITE_ADDRESS empty
make prod-up                        # build + start the prod stack
make prod-logs                      # watch: migrate exits 0, api+caddy healthy
curl http://localhost/healthz       # 200, proxied through Caddy
```

End-to-end through Caddy:

```sh
API_BASE=http://localhost bash scripts/check_minio_api.sh
# or edit scripts/verify_e2e.py  B = "http://localhost"  and run it
```

Tear down with `make prod-down`.

---

## C. VPS deployment (always-on HTTPS) — recommended for a defense

Best for: a stable URL the committee can hit any time.

1. **Provision** a small VPS (e.g. Hetzner CX22 / DigitalOcean ~€4–5/mo),
   install Docker + Compose, open firewall ports **80** and **443**.
2. **DNS**: point an `A` record (e.g. `api.yourdomain.com`) at the VPS IP.
3. **Copy the repo** to the VPS and create `.env.prod`:
   ```sh
   cp .env.prod.example .env.prod
   # set SITE_ADDRESS=api.yourdomain.com
   # set strong JWT_SECRET, INTERNAL_API_TOKEN, DB_PASSWORD, S3_SECRET_KEY
   ```
4. **Launch**:
   ```sh
   make prod-up
   ```
   Caddy automatically requests and renews a Let's Encrypt certificate for
   `SITE_ADDRESS`.
5. **Verify**:
   ```sh
   curl -v https://api.yourdomain.com/healthz   # 200 + valid cert chain
   ```

### No domain yet?

Leave `SITE_ADDRESS` empty and Caddy serves plain HTTP on `:80`
(`http://<vps-ip>/healthz`). Add the domain later by setting `SITE_ADDRESS` and
re-running `make prod-up` — issued certs persist in the `caddy_data` volume.

---

## Tunnel vs VPS

| | Quick tunnel (cloudflared/ngrok) | VPS + Caddy |
|---|---|---|
| Cost | Free | ~€4–5/mo |
| Setup time | Seconds | ~30 min first time |
| URL stability | Ephemeral (free tier) | Stable + your domain |
| Uptime | Only while your machine runs | Always-on |
| HTTPS | Provided by the tunnel | Auto Let's Encrypt |
| Best for | Device testing, demos | Thesis defense, real clients |

Recommendation: use the **tunnel now** for device testing; stand up the **VPS**
for the always-on endpoint you show during the defense. This repo supports both.

---

## Topology & security notes

```
public internet ──► caddy :80/:443 (TLS) ──► api:8080 ──► db:5432 / minio:9000
                         (only public surface)        (internal compose network)
```

- **Only Caddy is public.** `docker-compose.prod.yml` does **not** publish the db
  (5432), MinIO (9000/9001), or the API (8080) to the host — unlike the dev
  `docker-compose.yml`, which exposes them for convenience.
- **Rotate secrets.** The dev defaults (`dev-only-secret-change-me`,
  `minioadmin`, …) must be replaced in `.env.prod`. `.env.prod` is gitignored.
- **`/api/*` callbacks.** `GET /api/schema` and `POST /api/analytics/query` are
  AI-service callbacks guarded by a shared secret (`X-Internal-Token`). If your
  AI service runs on the same host (not a remote caller), uncomment the block in
  `Caddyfile` to forbid them from the public internet entirely.
- **Upload size.** Caddy caps request bodies at 35 MB, just above the app's
  32 MiB upload limit (`maxUploadBytes`).

## nginx alternative

Caddy was chosen for one-line automatic HTTPS. If your environment standardizes
on nginx, replace the `caddy` service with an `nginx` + `certbot` pair: nginx
`proxy_pass http://api:8080;` with `client_max_body_size 35m;`, and a certbot
companion container (or host certbot) managing `/etc/letsencrypt`. The API side
is identical — it stays plain HTTP on `:8080`.

## Migrations

The one-shot `migrate` service applies `project/migrations/core/*.up.sql` then
`project/migrations/analysis/*.up.sql` in filename order on first boot. It is
**idempotent**: it checks whether the DB is already initialized (the `users`
table exists) and exits 0 without re-applying, so `api` starts cleanly on every
`make prod-up`. It is a first-boot initializer, not a tracked migration runner —
when you add migrations beyond the initial set, introduce a real runner
(e.g. golang-migrate) with a `schema_migrations` table so incremental migrations
apply in order.
