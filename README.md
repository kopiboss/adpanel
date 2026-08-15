# AdPanel

Platform SaaS dashboard manajemen Meta Ads. Multi-user, setiap user isolated, kelola kredensial & akun iklan Meta sendiri.

## Stack

- **Backend**: Go 1.22 + Gin
- **Database**: MySQL
- **Frontend**: Go HTML Templates + Tailwind CSS + Alpine.js + Chart.js
- **Auth**: Session-based + Google OAuth2
- **Notifikasi**: Telegram Bot
- **Scheduler**: cron tiap 30 menit

---

## Setup Awal

### 1. Buat Meta App & System User Access Token

1. Buka [Meta for Developers](https://developers.facebook.com)
2. Buat app baru → pilih "Business"
3. Di Business Settings → System Users → buat System User
4. Generate token dengan permission: `ads_management`, `ads_read`
5. Catat **App ID**, **App Secret**, dan **Access Token**

### 2. Buat Telegram Bot & Bot Token

1. Buka Telegram, chat dengan [@BotFather](https://t.me/BotFather)
2. Kirim `/newbot` dan ikuti instruksi
3. Catat **Bot Token** yang diberikan
4. Untuk Chat ID admin: kirim pesan ke bot, lalu buka `https://api.telegram.org/bot{TOKEN}/getUpdates`
5. Catat `chat.id` dari response

### 3. Setup Google OAuth di Google Cloud Console

1. Buka [Google Cloud Console](https://console.cloud.google.com)
2. Buat project baru
3. APIs & Services → Credentials → Create OAuth Client ID
4. Application Type: Web application
5. Authorized redirect URIs: `https://yourdomain.com/auth/google/callback`
6. Catat **Client ID** dan **Client Secret**

---

## Setup VPS dari Nol

```bash
# Upload script ke VPS
scp install_vps.sh user@your-vps:/tmp/

# Jalankan di VPS
ssh user@your-vps
chmod +x /tmp/install_vps.sh
sudo bash /tmp/install_vps.sh
```

Script akan otomatis:
- Install MySQL, Nginx, Certbot
- Buat database dan user
- Setup Nginx reverse proxy
- Setup SSL dengan Let's Encrypt
- Buat file `.env` template

---

## Setup Database

```bash
mysql -u adpanel -p adpanel < database/migrations/001_init.sql
```

---

## Konfigurasi .env

Salin `.env.example` ke `.env` dan isi semua nilai:

```bash
cp .env.example .env
nano .env
```

**Wajib diisi:**
- `APP_SECRET` — string acak, min 32 karakter
- `ADMIN_EMAIL` & `ADMIN_PASSWORD` — login super admin
- `DB_PASS` — password database MySQL
- `ENCRYPTION_KEY` — generate dengan `openssl rand -hex 32`

**Opsional (bisa diisi dari dashboard admin):**
- `TELEGRAM_BOT_TOKEN` & `TELEGRAM_CHAT_ID`
- `GOOGLE_CLIENT_ID` & `GOOGLE_CLIENT_SECRET`

---

## Build & Run Lokal

```bash
# Install dependencies
go mod download

# Run
go run .

# Atau build dulu
go build -o adpanel .
./adpanel
```

---

## Deploy ke VPS

### Setup GitHub Actions

Tambahkan secrets di GitHub repo (Settings → Secrets):

| Secret | Value |
|--------|-------|
| `VPS_HOST` | IP atau domain VPS |
| `VPS_USER` | Username SSH (biasanya `root` atau `ubuntu`) |
| `VPS_SSH_KEY` | Private key SSH (isi dengan `cat ~/.ssh/id_rsa`) |
| `VPS_PATH` | Path di VPS, contoh: `/opt/adpanel` |

### Deploy Pertama Kali (Manual)

```bash
# Build binary di local (untuk Linux amd64)
GOOS=linux GOARCH=amd64 go build -o adpanel .

# Upload ke VPS
scp adpanel user@your-vps:/opt/adpanel/
scp -r templates/ user@your-vps:/opt/adpanel/
scp -r static/ user@your-vps:/opt/adpanel/

# Install systemd service
scp adpanel.service user@your-vps:/etc/systemd/system/
ssh user@your-vps "systemctl daemon-reload && systemctl enable adpanel && systemctl start adpanel"
```

### Deploy Berikutnya

Push ke branch `main` → GitHub Actions otomatis build dan deploy.

---

## Systemd Commands

```bash
# Status
systemctl status adpanel

# Lihat log
journalctl -u adpanel -f

# Restart
systemctl restart adpanel

# Stop
systemctl stop adpanel
```

---

## Penggunaan

### Login Admin

- URL: `https://yourdomain.com/login`
- Email & password dari `.env` (`ADMIN_EMAIL` / `ADMIN_PASSWORD`)
- Admin dashboard: `https://yourdomain.com/admin`

### Alur User Biasa

1. Register di `/register`
2. Tunggu approve dari admin (via dashboard atau Telegram)
3. Login → Dashboard
4. Tambah Kredensial Meta (App ID, App Secret, Access Token)
5. Fetch & pilih Ad Account
6. Upload creative (gambar/video)
7. Buat kampanye dengan wizard 3 step
8. Pantau analytics

---

## Struktur Folder

```
adpanel/
├── main.go                    # Entry point
├── config/config.go           # Konfigurasi dari env
├── database/
│   ├── db.go                  # Koneksi MySQL
│   └── migrations/001_init.sql
├── models/                    # DB models & queries
├── services/                  # Business logic
│   ├── crypto.go              # AES-256-GCM encryption
│   ├── oauth.go               # Google OAuth2
│   ├── telegram.go            # Telegram bot
│   ├── meta_api.go            # Meta Graph API
│   ├── meta_upload.go         # Upload gambar/video
│   ├── meta_insights.go       # Fetch insights
│   └── sync_worker.go         # Background sync
├── handlers/                  # HTTP handlers
├── middleware/                # Auth & role middleware
├── templates/                 # HTML templates
└── static/                   # Static files
```

---

## Keamanan

- Password di-hash dengan bcrypt
- Kredensial Meta dienkripsi AES-256-GCM di database
- Semua API call ke Meta dari server-side
- Session-based authentication
- HTTPS via Nginx + Let's Encrypt
- Super Admin login dari `.env` (tidak ada di tabel users)
