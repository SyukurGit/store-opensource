# 🚀 Panduan Hosting Simpel: Digimest via Cloudflare Tunnel

Panduan ini dirancang untuk **Developer** maupun **AI Coding Agent** agar memiliki pemahaman arsitektur yang searah, ringkas, dan modern. Tidak ada lapisan rumit (*zero complex layers*): tanpa Nginx, tanpa port-forwarding, dan tanpa konfigurasi SSL manual (Certbot/Let's Encrypt).

---

## 🌐 Arsitektur 1-Layer (Cloudflare Tunnels)

Seluruh lalu lintas dari internet diamankan oleh Cloudflare dan diteruskan secara aman ke server melalui koneksi keluar (*outbound encrypted tunnel*) menggunakan daemon `cloudflared`.

```
[ Pengunjung / Browser ]
           │ HTTPS
           ▼
[ Cloudflare Edge Network (DDoS / CDN / Auto-SSL) ]
           │ Encrypted Tunnel (Outbound)
           ▼
   [ cloudflared agent di Server / VPS ]
           │ HTTP (localhost:8080)
           ▼
   [ Digimest Go Backend (:8080) ] ────► [ SQLite: storeku.db ]
           │
     (Melayani REST API & Web Frontend)
```

### Keunggulan Setup Ini:
1. **Tanpa Buka Port Publik:** Firewall server dapat menutup port 80 dan 443. Hanya butuh koneksi internet keluar (*outbound*).
2. **Tanpa Setup Nginx:** Go router (Chi) sudah menangani routing REST API, file statis, Clean URLs, dan proteksi header secara mandiri.
3. **Auto SSL:** Sertifikat HTTPS dikelola otomatis 100% oleh Cloudflare Edge.
4. **Ringan & Hemat Resource:** Hanya 2 proses yang berjalan di server: binary Go (`storeku-linux`) dan daemon `cloudflared`.

---

## 🛠️ Pilihan Model Deployment

Pilihlah salah satu model sesuai preferensi infrastruktur Anda:

### 🌟 Model 1: Full-Stack Single Binary (Direkomendasikan - Paling Simpel)
Go backend menyajikan REST API (`/api/*`) sekaligus file statis frontend dari folder `web/` di port `8080`.
* **Domain:** `digimest.tech` langsung diarahkan ke `http://localhost:8080`.
* **Kelebihan:** 1 server, 1 folder, 0 konfigurasi CORS cross-domain, tidak perlu deploy terpisah.

### 🌐 Model 2: Terpisah (Cloudflare Pages + Cloudflare Tunnel API)
* **Frontend:** Di-deploy di **Cloudflare Pages** (gratis) mengarah ke domain `digimest.tech`.
* **Backend:** Berjalan di VPS dihubungkan via Cloudflare Tunnel ke subdomain `api.digimest.tech`.
* **Koneksi:** Frontend memanggil `https://api.digimest.tech/api/*` via file `web/js/api-config.js`.

---

## 📋 Langkah Eksekusi (Step-by-Step)

### Langkah 1: Kompilasi Binary untuk Linux (dari Komputer Lokal)
Jalankan perintah Go cross-compile berikut di terminal root project:
```bash
# Windows PowerShell
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"
go build -ldflags="-s -w" -o deploy/storeku-linux ./cmd/server
$env:GOOS = "windows"

# Atau via bash / Linux:
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o deploy/storeku-linux ./cmd/server
```
> *Catatan: Binary Go ini menggunakan modernc SQLite (pure Go), tidak memerlukan CGO atau GCC compiler.*

---

### Langkah 2: Persiapan di Server / VPS
1. Buat folder kerja aplikasi di VPS:
   ```bash
   mkdir -p /var/www/digimest
   cd /var/www/digimest
   ```
2. Upload file berikut dari lokal ke `/var/www/digimest/`:
   * `deploy/storeku-linux` (file binary Go)
   * `.env` (file konfigurasi kredensial)
   * Folder `web/` (mirror statis yang sudah disinkronkan dari `scratch_fe/`)
3. Berikan izin eksekusi pada binary:
   ```bash
   chmod +x /var/www/digimest/storeku-linux
   ```

---

### Langkah 3: Jalankan Backend sebagai Systemd Service
Agar aplikasi berjalan otomatis di background dan hidup kembali saat server restart:

1. Buat file systemd service:
   ```bash
   sudo nano /etc/systemd/system/digimest.service
   ```
2. Isi konfigurasinya:
   ```ini
   [Unit]
   Description=Digimest Digital Goods Backend Service
   After=network.target

   [Service]
   Type=simple
   User=root
   WorkingDirectory=/var/www/digimest
   ExecStart=/var/www/digimest/storeku-linux
   Restart=always
   RestartSec=5
   Environment=PORT=8080

   [Install]
   WantedBy=multi-user.target
   ```
3. Aktifkan dan jalankan service:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now digimest
   sudo systemctl status digimest
   ```
4. Verifikasi lokal server:
   ```bash
   curl http://localhost:8080/health
   # Respon yang diharapkan: {"service":"digimest-backend","status":"healthy","success":true,"version":"1.0.0"}
   ```

---

### Langkah 4: Setup Cloudflare Tunnel (`cloudflared`)

#### A. Install `cloudflared` di Server
```bash
# Untuk Ubuntu / Debian:
curl -L --output cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
sudo dpkg -i cloudflared.deb
```

#### B. Login & Buat Tunnel
1. Autentikasi server dengan akun Cloudflare Anda:
   ```bash
   cloudflared tunnel login
   ```
   *(Buka URL yang muncul di browser dan pilih domain `digimest.tech`)*

2. Buat tunnel baru:
   ```bash
   cloudflared tunnel create digimest-tunnel
   ```
   *(Catat `Tunnel ID` yang dihasilkan, contoh: `a1b2c3d4-xxxx-xxxx-xxxx-xxxxxxxxxxxx`)*

#### C. Konfigurasi Tunnel Ingress
Buat file konfigurasi di `~/.cloudflared/config.yml`:
```bash
nano ~/.cloudflared/config.yml
```

**Jika menggunakan Model 1 (Full-stack di domain utama `digimest.tech`):**
```yaml
tunnel: <TUNNEL_ID_ANDA>
credentials-file: /root/.cloudflared/<TUNNEL_ID_ANDA>.json

ingress:
  - hostname: digimest.tech
    service: http://localhost:8080
  - hostname: www.digimest.tech
    service: http://localhost:8080
  - service: http_status:404
```

**Jika menggunakan Model 2 (API Subdomain `api.digimest.tech`):**
```yaml
tunnel: <TUNNEL_ID_ANDA>
credentials-file: /root/.cloudflared/<TUNNEL_ID_ANDA>.json

ingress:
  - hostname: api.digimest.tech
    service: http://localhost:8080
  - service: http_status:404
```

#### D. Route DNS Cloudflare
Hubungkan domain ke tunnel secara otomatis:
```bash
# Untuk domain utama (Model 1):
cloudflared tunnel route dns digimest-tunnel digimest.tech

# Atau untuk subdomain API (Model 2):
cloudflared tunnel route dns digimest-tunnel api.digimest.tech
```

#### E. Jalankan `cloudflared` sebagai System Service
```bash
sudo cloudflared service install
sudo systemctl enable --now cloudflared
sudo systemctl status cloudflared
```

---

## 🎯 Verifikasi Akhir
Buka browser dan uji URL produksi:
1. **Health Check:** `https://digimest.tech/health` (atau `https://api.digimest.tech/health`)
2. **Katalog Produk:** `https://digimest.tech/`
3. **Klaim Lisensi:** `https://digimest.tech/claim`
4. **Panel Admin:** `https://digimest.tech/admin`

Sistem kini berjalan 24/7 secara aman, otomatis, dan siap melayani transaksi pembeli!
