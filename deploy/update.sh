#!/bin/bash
set -e

echo "🚀 [1/4] Mengambil pembaruan terbaru dari Git..."
git pull origin main

echo "🔨 [2/4] Mengompilasi binary Linux..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o storeku-linux ./cmd/server

echo "📦 [3/4] Memasang binary baru ke /var/www/digimest..."
sudo install -m 755 storeku-linux /var/www/digimest/storeku-linux

# Update folder web jika ada pembaruan file frontend
if [ -d "web" ]; then
    sudo cp -r web /var/www/digimest/
fi

# Update .env jika ada perubahan konfigurasi
if [ -f ".env" ]; then
    echo "⚙️ Memperbarui konfigurasi .env ke /var/www/digimest/.env..."
    sudo cp .env /var/www/digimest/.env
fi

echo "🔄 [4/4] Memuat ulang service digimest..."
sudo systemctl restart digimest

echo "✅ Selesai! Status service:"
sudo systemctl status digimest --no-pager
