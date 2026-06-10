# VPS setup runbook — from buying a domain to a live HTTPS API

A complete, copy-paste guide to put this backend on a virtual server with a real
domain and automatic HTTPS. Assumes you start with nothing but this repo on
GitHub (`https://github.com/AbuAlls/diplom_repo.git`).

End state:

```
phone / frontend ──HTTPS──► https://api.yourdomain.com ──► Caddy ──► api:8080 ──► Postgres / MinIO
```

Total time: ~30–45 min the first time. Cost: ~€4–5/month + ~€10/year domain.

---

## Step 0 — What you'll need

- A credit card or PayPal (for the VPS provider + domain registrar).
- The GitHub repo (already have it).
- A terminal on your Mac (you already use one).
- ~45 minutes.

Pick names now so the commands below are concrete. This guide uses placeholders —
replace everywhere:

| Placeholder | Example | Meaning |
|---|---|---|
| `YOURDOMAIN` | `mydiplom.ru` | the domain you'll buy |
| `api.YOURDOMAIN` | `api.mydiplom.ru` | the subdomain for the API |
| `SERVER_IP` | `203.0.113.42` | the VPS public IP (you get this in Step 2) |

---

## Step 1 — Buy a domain

Any registrar works. Cheap/common options: **Namecheap**, **Cloudflare
Registrar**, **reg.ru** (RU), **Porkkun**.

1. Search for a domain, add to cart, pay.
2. You do **not** need their hosting, email, or "web builder" add-ons — just the
   domain.
3. Keep the registrar's DNS control panel open; you'll add one record in Step 6.

You can also do Step 2 first and buy the domain later — the server works on its
raw IP without a domain (just no HTTPS yet).

---

## Step 2 — Create the virtual server (VPS)

Recommended: **Hetzner Cloud** (cheapest, EU) or **DigitalOcean** (simplest UI).

### Hetzner

1. Sign up at https://console.hetzner.cloud → create a project.
2. **Add Server**:
   - Location: closest to you (e.g. Falkenstein/Nuremberg for EU).
   - Image: **Ubuntu 24.04**.
   - Type: **CX22** (2 vCPU / 4 GB) — plenty for this MVP. (CX11/CAX11 also fine.)
   - **SSH key**: see Step 3 — add your public key here so you can log in without
     a password. (If you skip this, Hetzner emails you a root password.)
   - Name it e.g. `diplom-api`.
3. Create. After ~30s you get a **public IPv4** — this is your `SERVER_IP`.

### DigitalOcean

1. https://cloud.digitalocean.com → **Create → Droplet**.
2. Ubuntu 24.04, Basic plan, **$6/mo** (1 GB) or **$12/mo** (2 GB, recommended).
3. Add your SSH key (Step 3), create. Copy the droplet's IP = `SERVER_IP`.

---

## Step 3 — Create an SSH key (if you don't have one)

On your **Mac**, check first:

```sh
ls ~/.ssh/id_ed25519.pub
```

If it prints "No such file", create one:

```sh
ssh-keygen -t ed25519 -C "diplom-vps"
# press Enter 3x to accept defaults (no passphrase is fine for a school project)
```

Show the public key and paste it into the provider's "SSH keys" box (Step 2):

```sh
cat ~/.ssh/id_ed25519.pub
```

---

## Step 4 — First login & secure the server

From your Mac:

```sh
ssh root@SERVER_IP
```

(Type `yes` to accept the fingerprint the first time.)

Now, **on the server**, update and create a non-root user (good practice; root
login over SSH is risky):

```sh
apt update && apt -y upgrade

adduser deploy            # set a password when asked; Enter through the rest
usermod -aG sudo deploy

# Copy your SSH key so you can log in as 'deploy' too:
rsync --archive --chown=deploy:deploy ~/.ssh /home/deploy/
```

Open a **new** terminal on your Mac and confirm you can log in as the new user:

```sh
ssh deploy@SERVER_IP
```

From here on, work as `deploy` (use `sudo` for admin commands).

### Firewall

Allow SSH + the web ports, block everything else:

```sh
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw --force enable
sudo ufw status          # should list 22, 80, 443
```

> Note: the database (5432) and MinIO (9000/9001) are intentionally **not**
> opened — the prod compose keeps them on the internal Docker network only.

---

## Step 5 — Install Docker on the server

```sh
# Docker's official one-line installer:
curl -fsSL https://get.docker.com | sudo sh

# Let 'deploy' run docker without sudo:
sudo usermod -aG docker deploy

# Apply the group change (log out and back in, or):
newgrp docker

# Verify:
docker --version
docker compose version
```

---

## Step 5.5 — (On your Mac) commit & push the prod files

The deployment files exist locally but are **not yet on GitHub**, and you're on
the `codex` branch. The VPS clones from GitHub, so push them first.

On your **Mac**, in the repo:

```sh
cd /Users/nikitagusev/obsidian/diplom_repo

# Sanity check what's new:
git status

# Stage the deployment files (NOT .env.prod — it's gitignored, keep it that way):
git add Caddyfile Dockerfile.prod docker-compose.prod.yml .env.prod.example \
        .gitignore Makefile docs/deployment.md docs/vps-setup-runbook.md docs/arhitecture.md

git commit -m "Add production deployment: Caddy reverse proxy, prod compose, docs"

# Push. If your main branch is 'main', merge codex first or push the branch:
git push origin codex
```

> If you deploy from the `codex` branch, use `git clone -b codex …` in Step 6.
> If you merge into `main` on GitHub first, clone normally.

---

## Step 6 — Point the domain at the server (DNS)

In your **registrar's DNS panel**, add a single **A record**:

| Type | Name / Host | Value | TTL |
|---|---|---|---|
| A | `api` | `SERVER_IP` | default (auto) |

- `Name = api` → creates `api.YOURDOMAIN`.
- If you want the root domain too, add a second A record with `Name = @`.

Wait for DNS to propagate (usually 1–10 min). Check from your Mac:

```sh
dig +short api.YOURDOMAIN     # should print SERVER_IP
# or: nslookup api.YOURDOMAIN
```

Don't continue to the HTTPS step until this returns your `SERVER_IP` — Caddy needs
working DNS to obtain a certificate.

---

## Step 7 — Get the code onto the server & configure

Back in your **server** session (`ssh deploy@SERVER_IP`):

```sh
# Clone (use -b codex if you pushed to the codex branch):
git clone -b codex https://github.com/AbuAlls/diplom_repo.git
cd diplom_repo

# Create the real production env from the template:
cp .env.prod.example .env.prod
nano .env.prod
```

In `nano`, set these (Ctrl+O to save, Ctrl+X to exit):

```ini
# Your domain → tells Caddy to fetch HTTPS automatically:
SITE_ADDRESS=api.YOURDOMAIN

# Replace EVERY CHANGE_ME with a strong random value.
# Generate values quickly on the server with: openssl rand -hex 32
JWT_SECRET=<paste a long random string>
INTERNAL_API_TOKEN=<paste another random string>
DB_PASSWORD=<paste a random string>
S3_SECRET_KEY=<paste a random string>
S3_ACCESS_KEY=minioadmin            # can stay, or change it too
```

Tip — generate four secrets at once:

```sh
for n in JWT_SECRET INTERNAL_API_TOKEN DB_PASSWORD S3_SECRET_KEY; do
  echo "$n=$(openssl rand -hex 32)"
done
```

Paste those lines into `.env.prod` (replacing the `CHANGE_ME` ones).

---

## Step 8 — Launch

```sh
make prod-up
```

This builds the API image, starts Postgres + MinIO (internal), runs the one-shot
migrations, then starts the API and Caddy. Caddy automatically requests a
Let's Encrypt certificate for `SITE_ADDRESS`.

Watch it come up:

```sh
make prod-logs
```

Look for:
- `migrate ... migrations applied` (or "already initialized" on later runs),
- `api` becoming healthy,
- Caddy lines about obtaining a certificate for `api.YOURDOMAIN`.

Press Ctrl+C to stop tailing logs (the stack keeps running).

---

## Step 9 — Verify it's live over HTTPS

From your **Mac** (or any device):

```sh
curl https://api.YOURDOMAIN/healthz
# → {"status":"ok"}

# Full end-to-end flow (register → upload → confirm, plus groups) through HTTPS:
# (run from your Mac inside the repo)
API_BASE=https://api.YOURDOMAIN bash scripts/check_minio_api.sh
```

In a browser, open `https://api.YOURDOMAIN/healthz` — you should see the JSON and
a valid padlock (no certificate warning).

**Point your frontend / mobile app** at `https://api.YOURDOMAIN`.

---

## Step 10 — Day-2 operations

All commands run **on the server**, inside `~/diplom_repo`.

| Task | Command |
|---|---|
| View logs | `make prod-logs` |
| Restart everything | `make prod-down && make prod-up` |
| Stop (keep data) | `make prod-down` |
| Deploy new code | `git pull` then `make prod-up` (rebuilds the API) |
| Check containers | `docker compose -f docker-compose.prod.yml ps` |
| DB shell | `docker compose -f docker-compose.prod.yml exec db psql -U app -d app` |
| Disk usage | `df -h` and `docker system df` |
| Free space | `docker system prune -f` (removes unused images/layers) |

Data (Postgres rows, uploaded files, issued certs) lives in Docker **named
volumes** (`pgdata_prod`, `miniodata_prod`, `caddy_data`) and survives
`prod-down` / reboots. To wipe everything and start fresh:
`docker compose -f docker-compose.prod.yml down -v` (the `-v` deletes volumes —
**destroys all data**).

---

## Troubleshooting

**Certificate isn't issued / browser shows "not secure".**
- DNS must resolve first: `dig +short api.YOURDOMAIN` must equal `SERVER_IP`.
- Ports 80 **and** 443 must be open (`sudo ufw status`) — Let's Encrypt validates
  over port 80.
- Check Caddy logs: `docker compose -f docker-compose.prod.yml logs caddy`.
- Caddy's free-tier retries automatically; give it a minute after DNS is correct.

**`curl https://api.YOURDOMAIN/healthz` hangs or refuses.**
- Is the stack up? `docker compose -f docker-compose.prod.yml ps` — `api` and
  `caddy` should be `running`/`healthy`.
- Firewall: `sudo ufw status` shows 80, 443.

**`make prod-up` fails on the API build.**
- Check the build log; usually a transient network issue pulling Go modules —
  just re-run `make prod-up`.

**`migrate` service errors.**
- On a brand-new DB it should print "migrations applied". On a re-run it prints
  "already initialized; skipping". If it errors on first boot, read its log:
  `docker compose -f docker-compose.prod.yml logs migrate`.

**I don't have a domain yet but want to test.**
- Leave `SITE_ADDRESS=` empty in `.env.prod`. Caddy serves plain HTTP on port 80.
  Test with `curl http://SERVER_IP/healthz`. Add the domain later by setting
  `SITE_ADDRESS` and re-running `make prod-up` — the cert is fetched then.

---

## Quick reference card

```sh
# --- one time ---
ssh-keygen -t ed25519                 # Mac: make a key (if needed)
# buy domain; create Ubuntu 24.04 VPS with that key → note SERVER_IP
ssh root@SERVER_IP                    # then create 'deploy' user, ufw, docker
# DNS: A record  api → SERVER_IP

# --- Mac: publish the deploy files ---
git add Caddyfile Dockerfile.prod docker-compose.prod.yml .env.prod.example Makefile .gitignore docs/
git commit -m "deploy"; git push origin codex

# --- server: deploy ---
git clone -b codex https://github.com/AbuAlls/diplom_repo.git && cd diplom_repo
cp .env.prod.example .env.prod && nano .env.prod   # SITE_ADDRESS + secrets
make prod-up
make prod-logs

# --- verify ---
curl https://api.YOURDOMAIN/healthz
```
