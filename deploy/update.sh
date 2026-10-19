#!/bin/bash
set -e

echo "🚀 [1/4] Mengambil pembaruan terbaru dari Git..."
git pull origin main

echo "🔨 [2/4] Mengompilasi binary Linux..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o storeku-linux ./cmd/server

echo "📦 [3/4] Memasang binary baru ke /var/www/digimest..."
sudo install -m 755 storeku-linux /var/www/digimest/storeku-linux

# Sinkronisasi .env: Jika ada file .env lokal, otomatis perbarui ke /var/www/digimest/.env
if [ -f ".env" ]; then
    echo "⚙️ Menyinkronkan .env terbaru ke /var/www/digimest/.env..."
    sudo cp .env /var/www/digimest/.env
elif [ ! -f "/var/www/digimest/.env" ]; then
    if [ -f ".env.example" ]; then
        echo "⚙️ Inisialisasi awal konfigurasi .env dari .env.example..."
        sudo cp .env.example /var/www/digimest/.env
    fi
else
    echo "🔒 File /var/www/digimest/.env live dipertahankan."
fi

# Bersihkan direktori web statis lama dari VPS jika masih ada (Frontend sudah terpisah di Cloudflare Pages)
if [ -d "/var/www/digimest/web" ]; then
    echo "🧹 Membersihkan direktori web statis lama dari VPS..."
    sudo rm -rf /var/www/digimest/web
fi

echo "🔄 [4/4] Memuat ulang service digimest..."
sudo systemctl restart digimest

echo "✅ Selesai! Status service:"
sudo systemctl status digimest --no-pager
