# 🚀 Panduan Deployment: Cloudflare Pages (Frontend) & Linux VPS (Backend)

Arsitektur ini memisahkan aplikasi menjadi dua komponen independen:
1. **Frontend:** Statis murni di hosting gratis & cepat **Cloudflare Pages** (`digimest.tech`).
2. **Backend:** Layanan REST API Go mandiri di **VPS Linux** (`api.digimest.tech`).

---

## 📁 1. Langkah Deploy Frontend ke Cloudflare Pages

### A. Persiapan Folder
Folder yang di-upload ke Cloudflare Pages adalah folder **`web/`**.
Folder ini sudah dilengkapi dengan:
- `_redirects`: Meneruskan `/api/*` ke VPS secara otomatis dan mengatur Clean URLs (`/product`, `/about`, `/admin`, dll.).
- `_headers`: Kebijakan cache cepat untuk CSS, JS, logo.
- `js/api-config.js`: Modul konfigurasi API global.

### B. Upload ke Cloudflare Pages
1. Buka dashboard Cloudflare: **Workers & Pages** ➔ **Create application** ➔ **Pages**.
2. Pilih salah satu:
   - **Connect to Git:** Hubungkan ke repo GitHub Anda. Pada setting build:
     - **Build output directory:** `web`
     - **Build command:** *(Kosongkan / None)*
   - **Direct Upload:** Drag & drop langsung seluruh isi folder `web/` ke Cloudflare Pages.
3. Hubungkan custom domain `digimest.tech` ke project Pages Anda di menu **Custom domains**.

---

## 🐧 2. Langkah Deploy Backend ke VPS Linux

### A. Kompilasi Binary Linux dari Komputer Lokal
Jalankan script PowerShell di folder project:
```powershell
.\deploy\build-linux.ps1
```
Script ini akan menghasilkan file executable Linux: `deploy/storeku-linux` (Pure Go, tanpa dependensi C/GCC).

### B. Upload ke VPS
1. Buat folder di VPS Anda:
   ```bash
   mkdir -p /var/www/digimest-backend
   ```
2. Upload file binary `deploy/storeku-linux` dan file `.env` ke `/var/www/digimest-backend/`.
3. Berikan izin eksekusi pada binary:
   ```bash
   chmod +x /var/www/digimest-backend/storeku-linux
   ```

### C. Pasang Background Service (Systemd)
1. Salin file service:
   ```bash
   cp deploy/storeku.service /etc/systemd/system/storeku.service
   ```
2. Aktifkan dan jalankan service:
   ```bash
   systemctl daemon-reload
   systemctl enable --now storeku
   ```
3. Periksa status:
   ```bash
   systemctl status storeku
   ```
4. Untuk melihat live logs backend di VPS:
   ```bash
   journalctl -u storeku -f
   ```

### D. Konfigurasi Nginx & Domain `api.digimest.tech`
1. Salin konfigurasi Nginx:
   ```bash
   cp deploy/nginx-api.conf /etc/nginx/sites-available/api.digimest.tech
   ln -s /etc/nginx/sites-available/api.digimest.tech /etc/nginx/sites-enabled/
   ```
2. Pasang SSL gratis via Certbot (atau Cloudflare SSL):
   ```bash
   certbot --nginx -d api.digimest.tech
   ```
3. Restart Nginx:
   ```bash
   nginx -t && systemctl reload nginx
   ```

---

## 🩺 3. Uji Coba Health Check

Setelah backend di VPS menyala, Anda dapat menguji endpoint:
```bash
curl https://api.digimest.tech/health
```
Respon sukses:
```json
{
  "success": true,
  "status": "healthy",
  "service": "digimest-backend",
  "version": "1.0.0"
}
```

---

## ⚡ 4. Pengaturan DNS di Cloudflare
Pastikan di dashboard DNS Cloudflare domain Anda terdapat:
1. **`digimest.tech`** (CNAME ke project Pages Anda) ➔ **Proxied (Awan Oranye)**
2. **`api.digimest.tech`** (A record ke IP Public VPS Anda) ➔ **Proxied (Awan Oranye)**

Dengan awan oranye aktif, IP asli VPS Anda aman dari serangan DDoS dan seluruh lalu lintas dienkripsi SSL oleh Cloudflare!
