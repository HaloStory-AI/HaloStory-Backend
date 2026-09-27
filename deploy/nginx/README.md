# Nginx + SSL Deploy Guide — HaloStory Backend

Domain produksi: `backend-hsai.rafiq8k.my.id` — app Go `127.0.0.1:3000`.

## 1. Prasyarat VPS

- Ubuntu 22.04 / 24.04 (atau Debian)
- Domain `backend-hsai.rafiq8k.my.id` sudah pointing A record ke IP VPS
- App jalan di `127.0.0.1:3000` via `docker compose up -d` — cek `curl http://localhost:3000/api/ping` -> `{"message":"pong","status":"ok"}`
- Port 80 & 443 terbuka

```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow OpenSSH
sudo ufw enable
```

## 2. Install Nginx + Certbot

```bash
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
sudo nginx -t && sudo systemctl enable --now nginx
```

## 3. Pasang Config

```bash
# dari root repo di VPS
sudo cp deploy/nginx/halostory.conf /etc/nginx/sites-available/halostory

sudo ln -s /etc/nginx/sites-available/halostory /etc/nginx/sites-enabled/halostory
# hapus default kalau bentrok
sudo rm -f /etc/nginx/sites-enabled/default

sudo nginx -t
sudo systemctl reload nginx
```

## 4. Sesuaikan .env di VPS

```bash
nano .env
# WAJIB https domain, bukan localhost — dipakai ImageURL & OpenRouter referer
APP_URL=https://backend-hsai.rafiq8k.my.id
APP_ENV=production
APP_DEBUG=false
GIN_MODE=release
```

```bash
docker compose pull && docker compose up -d --remove-orphans
curl http://localhost:3000/api/ping  # harus pong
```

## 5. Generate SSL (Let's Encrypt)

Opsi A — otomatis via nginx plugin (paling mudah):

```bash
sudo certbot --nginx -d backend-hsai.rafiq8k.my.id
# pilih redirect (2) kalau ditanya — akan isi ssl_certificate otomatis
```

Opsi B — webroot (pakai `location /.well-known/acme-challenge/` di conf):

```bash
sudo mkdir -p /var/www/certbot
sudo certbot certonly --webroot -w /var/www/certbot -d backend-hsai.rafiq8k.my.id
sudo nginx -t && sudo systemctl reload nginx
```

Tes renewal:

```bash
sudo certbot renew --dry-run
```

Auto-renew via systemd timer: `systemctl list-timers | grep certbot`

## 6. Verifikasi

```bash
curl -i http://backend-hsai.rafiq8k.my.id/api/ping       # harus 301 ke https
curl -i https://backend-hsai.rafiq8k.my.id/api/ping      # harus {"message":"pong","status":"ok"}
curl -i https://backend-hsai.rafiq8k.my.id/up            # maintenance check (200 / 503)
curl -i https://backend-hsai.rafiq8k.my.id/api/image?path=Bot/xxx/file  # image proxy S3

# SSE stream — harus header text/event-stream, jangan putus
curl -i -H "Authorization: Bearer <JWT>" https://backend-hsai.rafiq8k.my.id/api/chat/stream/<uuid>

# cek SSL grade
# https://www.ssllabs.com/ssltest/analyze.html?d=backend-hsai.rafiq8k.my.id
```

## 7. Catatan Konfigurasi

- `client_max_body_size 50M` — untuk `POST /api/chat/upload` (5 MB), `POST /api/user/backup`, `POST /admin/upload`.
- `proxy_buffering off` hanya untuk `~ ^/api/chat/stream/` (SSE), sisanya `on`.
- Rate limit: `10r/m` untuk `/api/login`, `/api/register`, `/api/forgot-password`; `5r/m` untuk `/api/email/send-verification-otp`.
- General proxy `location /` forward semua `/api/*`, `/admin*`, `/`, `/up`, `/metrics`, `NoRoute 404.html`.

## Troubleshooting

- `nginx -t` error `no such file ssl_dhparams` -> jalankan certbot dulu, atau biarkan 4 baris `ssl_certificate` tetap comment sampai cert terbit.
- `502 Bad Gateway` -> app belum ready: `docker compose ps` & `docker compose logs app --tail 50`
- `CORS error` -> `APP_URL` di .env harus `https://backend-hsai.rafiq8k.my.id`
- SSE putus -> pastikan `proxy_buffering off` untuk `/api/chat/stream/` dan `proxy_read_timeout 3600s`
