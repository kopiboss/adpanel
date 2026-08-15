#!/bin/bash
# Setup AdPanel di VPS yang sudah ada Caddy + MySQL
# Script ini TIDAK install Nginx/Certbot — Caddy sudah handle SSL
set -e

echo "=== AdPanel Setup (existing VPS) ==="
echo ""

# Cek prerequisites
command -v mysql >/dev/null 2>&1 || { echo "ERROR: MySQL tidak ditemukan"; exit 1; }
command -v caddy >/dev/null 2>&1 || { echo "ERROR: Caddy tidak ditemukan"; exit 1; }

read -p "Subdomain untuk AdPanel (contoh: panel.ktools.id): " DOMAIN
read -p "DB password untuk user adpanel (buat baru): " DB_PASS
read -p "MySQL root password: " MYSQL_ROOT_PASS

echo ""
echo "=== Membuat database MySQL ==="
mysql -u root -p"$MYSQL_ROOT_PASS" << SQL
CREATE DATABASE IF NOT EXISTS adpanel CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'adpanel'@'localhost' IDENTIFIED BY '$DB_PASS';
GRANT ALL PRIVILEGES ON adpanel.* TO 'adpanel'@'localhost';
FLUSH PRIVILEGES;
SQL
echo "✓ Database adpanel dibuat"

echo ""
echo "=== Membuat direktori aplikasi ==="
mkdir -p /opt/adpanel/templates
mkdir -p /opt/adpanel/static
echo "✓ /opt/adpanel siap"

echo ""
echo "=== Membuat file .env ==="
cat > /opt/adpanel/.env << ENV
APP_PORT=8080
APP_SECRET=$(openssl rand -hex 32)
APP_URL=https://$DOMAIN

ADMIN_EMAIL=admin@$DOMAIN
ADMIN_PASSWORD=GANTI_INI_SEKARANG

DB_HOST=localhost
DB_PORT=3306
DB_USER=adpanel
DB_PASS=$DB_PASS
DB_NAME=adpanel

ENCRYPTION_KEY=$(openssl rand -hex 32)

GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=https://$DOMAIN/auth/google/callback

TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
ENV
chmod 600 /opt/adpanel/.env
echo "✓ File .env dibuat"

echo ""
echo "=== Install systemd service ==="
cat > /etc/systemd/system/adpanel.service << SERVICE
[Unit]
Description=AdPanel - Meta Ads Management Platform
After=network.target mysql.service
Requires=mysql.service

[Service]
Type=simple
User=ubuntu
Group=ubuntu
WorkingDirectory=/opt/adpanel
ExecStart=/opt/adpanel/adpanel
EnvironmentFile=/opt/adpanel/.env
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=adpanel

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable adpanel
echo "✓ Service adpanel terdaftar"

echo ""
echo "=== Tambah config ke Caddyfile ==="
CADDY_BLOCK="
$DOMAIN {
    reverse_proxy localhost:8080 {
        header_up Host {host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
        header_up X-Real-IP {remote_host}
    }

    request_body {
        max_size 1100MB
    }

    encode gzip
}"

# Backup Caddyfile dulu
cp /etc/caddy/Caddyfile /etc/caddy/Caddyfile.backup.$(date +%Y%m%d_%H%M%S)
echo "$CADDY_BLOCK" >> /etc/caddy/Caddyfile

# Validasi Caddyfile sebelum reload
caddy validate --config /etc/caddy/Caddyfile && systemctl reload caddy
echo "✓ Caddy dikonfigurasi untuk $DOMAIN"

echo ""
echo "=== Ownership ==="
chown -R ubuntu:ubuntu /opt/adpanel
echo "✓ Permission diset ke user ubuntu"

echo ""
echo "============================================"
echo "Setup selesai! Langkah selanjutnya:"
echo ""
echo "1. Edit /opt/adpanel/.env — ganti ADMIN_PASSWORD"
echo "2. Copy binary: scp adpanel ubuntu@VPS:/opt/adpanel/"
echo "3. Copy templates: rsync -avz templates/ ubuntu@VPS:/opt/adpanel/templates/"
echo "4. Jalankan migrasi DB:"
echo "   mysql -u adpanel -p adpanel < /opt/adpanel/database/migrations/001_init.sql"
echo "5. Start service: sudo systemctl start adpanel"
echo "6. Cek log: journalctl -u adpanel -f"
echo "============================================"
