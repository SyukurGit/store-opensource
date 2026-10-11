# ==============================================================
# Script Build Linux VPS Binary (Windows PowerShell)
# ==============================================================
$ErrorActionPreference = "Stop"

Write-Host "==============================================================" -ForegroundColor Cyan
Write-Host "Mengompilasi Digimest Backend untuk Linux VPS (amd64)..." -ForegroundColor Cyan
Write-Host "==============================================================" -ForegroundColor Cyan

# Pure Go SQLite (modernc.org/sqlite) tidak membutuhkan GCC / CGO
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"

# Kompilasi dengan stripping debug info agar ukuran file lebih ramping (-s -w)
go build -ldflags="-s -w" -o deploy/storeku-linux .\cmd\server

if (Test-Path "deploy\storeku-linux") {
    $item = Get-Item "deploy\storeku-linux"
    $sizeMB = [math]::Round($item.Length / 1MB, 2)
    Write-Host "[OK] Kompilasi Sukses!" -ForegroundColor Green
    Write-Host "Binary Output : deploy\storeku-linux ($sizeMB MB)" -ForegroundColor Green
    Write-Host ""
    Write-Host "Langkah Deploy ke VPS:" -ForegroundColor Yellow
    Write-Host "1. Upload 'deploy/storeku-linux' dan file '.env' ke VPS folder '/var/www/digimest-backend/'"
    Write-Host "2. Di VPS jalankan: chmod +x /var/www/digimest-backend/storeku-linux"
    Write-Host "3. Pasang systemd service: cp deploy/storeku.service /etc/systemd/system/"
    Write-Host "4. Jalankan: systemctl daemon-reload; systemctl enable --now storeku"
    Write-Host "5. Cek status: systemctl status storeku"
} else {
    Write-Host "[ERROR] Gagal menghasilkan binary deploy\storeku-linux" -ForegroundColor Red
}
