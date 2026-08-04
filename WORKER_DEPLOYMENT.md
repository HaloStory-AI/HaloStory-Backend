# Worker Deployment Guide

Panduan untuk deploy HaloStory Worker di VPS terpisah (overflow).

## Arsitektur

```
VPS 1 (Primary):
  ├── halostory-app       (:3000)     — REST API
  ├── halostory-worker    (:3001)     — AI chat worker
  ├── halostory-scheduler             — Cron jobs
  ├── MySQL              (:3306)     — Database
  └── Redis              (:6379)     — Queue + Cache

VPS 2 (Overflow Worker):
  └── halostory-worker    (:3001)     — Consume dari Redis VPS 1
```

Worker di VPS 2 consume job dari Redis yang sama di VPS 1. Tambah worker = tambah throughput.

## Prerequisites di VPS 1

### 1. Redis: Aktifkan Password

Edit `/etc/redis/redis.conf`:

```
requirepass your-strong-redis-password
```

Restart Redis:

```bash
sudo systemctl restart redis
```

### 2. MySQL: Buat User Khusus Worker

```sql
CREATE USER 'worker'@'<VPS2_IP>' IDENTIFIED BY '<strong-password>';
GRANT SELECT, INSERT, UPDATE ON halostory.* TO 'worker'@'<VPS2_IP>';
FLUSH PRIVILEGES;
```

> Catatan: Worker butuh SELECT (read bots, users, settings), INSERT/UPDATE (save chat results, XP updates). Jika error karena insufficient privileges, tambah permission yang dibutuhkan.

### 3. Firewall: Buka Port ke VPS 2

**UFW:**

```bash
sudo ufw allow from <VPS2_IP> to any port 3306
sudo ufw allow from <VPS2_IP> to any port 6379
```

**iptables:**

```bash
iptables -A INPUT -s <VPS2_IP> -p tcp --dport 3306 -j ACCEPT
iptables -A INPUT -s <VPS2_IP> -p tcp --dport 6379 -j ACCEPT
```

**Cloud Provider (DigitalOcean, AWS, dll):**
Buat firewall rule di dashboard, allow TCP 3306 dan 6379 dari IP VPS 2.

## Setup di VPS 2

### 1. Install Docker

```bash
curl -fsSL https://get.docker.com | sh
```

### 2. Clone & Deploy

```bash
git clone https://github.com/your-username/halostory_go_backend.git /opt/halostory-worker
cd /opt/halostory-worker
cp .env.worker.example .env
nano .env  # Isi DB_HOST, REDIS_HOST, passwords
docker compose -f docker-compose.worker.yml up -d --build
```

### 3. Atau Pakai Script Otomatis

```bash
bash <(curl -s https://raw.githubusercontent.com/your-username/halostory_go_backend/main/scripts/deploy-worker.sh)
```

## Konfigurasi

### Environment Variables (Worker Only)

| Variable | Required | Default | Description |
|---|---|---|---|
| `DB_HOST` | ✅ | — | IP VPS 1 (MySQL) |
| `DB_PORT` | ❌ | 3306 | MySQL port |
| `DB_DATABASE` | ❌ | halostory | Database name |
| `DB_USERNAME` | ❌ | halostory | MySQL username |
| `DB_PASSWORD` | ✅ | — | MySQL password |
| `REDIS_HOST` | ✅ | — | IP VPS 1 (Redis) |
| `REDIS_PORT` | ❌ | 6379 | Redis port |
| `REDIS_PASSWORD` | ❌ | — | Redis password |
| `WORKER_CONCURRENCY` | ❌ | 30 | Jumlah goroutine |

### Rekomendasi Concurrency

| VPS Spec | WORKER_CONCURRENCY |
|---|---|
| 2 vCPU / 4GB RAM | 10-15 |
| 4 vCPU / 8GB RAM | 25-30 |
| 8 vCPU / 16GB RAM | 50-60 |

## Monitoring

### Cek Status Worker

```bash
docker compose -f docker-compose.worker.yml ps
```

### Lihat Logs

```bash
docker compose -f docker-compose.worker.yml logs -f
```

### Health Check

```bash
curl http://localhost:3001/health
```

Response:
```json
{"status":"ok"}
```

## Troubleshooting

### Worker tidak connect ke Redis

- Pastikan Redis password benar di `.env`
- Pastikan firewall port 6379 terbuka dari VPS 2
- Test: `redis-cli -h <VPS1_IP> -a <password> ping`

### Worker tidak connect ke MySQL

- Pastikan MySQL user dibuat untuk IP VPS 2
- Pastikan firewall port 3306 terbuka
- Test: `mysql -h <VPS1_IP> -u worker -p halostory`

### Worker crash restart terus

- Cek logs: `docker compose -f docker-compose.worker.yml logs --tail 50`
- Kemungkinan: insufficient MySQL permissions, network issue, atau OOM

## Scaling

Untuk tambah worker capacity, tinggal deploy ke VPS tambahan:

1. Deploy VPS 3, 4, dst dengan cara yang sama
2. Semua worker consume dari Redis queue yang sama
3. Redis BLPOP otomatis load-balance ke worker yang available
4. Tidak perlu config tambahan — auto-discovery lewat queue

## Rollback

Untuk stop worker di VPS 2:

```bash
docker compose -f docker-compose.worker.yml down
```

Worker di VPS 1 tetap berjalan normal. Tidak ada impact ke service.
