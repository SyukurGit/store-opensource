package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"storeku/internal/config"
)

type ResendService struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewResendService(cfg *config.Config) *ResendService {
	return &ResendService{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type ResendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

type ResendEmailResponse struct {
	ID    string `json:"id,omitempty"`
	Error struct {
		Message string `json:"message"`
		Name    string `json:"name"`
	} `json:"error,omitempty"`
}

// SendClaimEmail sends payment confirmation with total amount and claim code via Resend
func (s *ResendService) SendClaimEmail(toEmail, invoice, claimCode string, totalAmount int64, productName string, qty int) error {
	toEmail = strings.TrimSpace(toEmail)
	if toEmail == "" {
		return fmt.Errorf("email penerima kosong")
	}

	apiKey := strings.TrimSpace(s.cfg.ResendAPIKey)
	if apiKey == "" {
		log.Printf("[RESEND] RESEND_API_KEY belum dikonfigurasi di .env. Pengiriman email ke %s dilewati.", toEmail)
		return nil
	}

	fromEmail := s.cfg.ResendFromEmail
	if fromEmail == "" {
		fromEmail = "Digimest <orders@digimest.tech>"
	}

	claimURL := fmt.Sprintf("%s/claim?code=%s", strings.TrimRight(s.cfg.StoreBaseURL, "/"), claimCode)
	formattedTotal := fmt.Sprintf("Rp %s", formatNumber(totalAmount))

	subject := fmt.Sprintf("✓ Pembayaran Berhasil — Kode Ambil Pesanan: %s", claimCode)
	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Pembayaran Berhasil — Digimest</title>
</head>
<body style="margin: 0; padding: 0; background-color: #0b0f19; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #f8fafc;">
  <table width="100%%" border="0" cellspacing="0" cellpadding="0" style="background-color: #0b0f19; padding: 36px 12px;">
    <tr>
      <td align="center">
        <table width="100%%" border="0" cellspacing="0" cellpadding="0" style="max-width: 560px; background-color: #131b2e; border: 1px solid rgba(255,255,255,0.12); border-radius: 18px; overflow: hidden; padding: 32px 26px; text-align: left; box-shadow: 0 12px 30px rgba(0,0,0,0.5);">
          
          <!-- Logo / Header -->
          <tr>
            <td>
              <div style="font-size: 22px; font-weight: 800; color: #ffffff; letter-spacing: -0.5px; margin-bottom: 4px;">
                ⚡ Digimest Digital
              </div>
              <div style="font-size: 13px; color: #94a3b8; margin-bottom: 22px;">
                Pusat Produk & Lisensi Digital Instan • Digimest.tech
              </div>
            </td>
          </tr>

          <!-- Banner Lunas -->
          <tr>
            <td>
              <div style="background-color: rgba(16, 185, 129, 0.12); border: 1px solid rgba(16, 185, 129, 0.35); border-radius: 10px; padding: 14px 16px; margin-bottom: 22px; color: #34d399; font-size: 14px; font-weight: 700;">
                ✓ Pembayaran QRIS Berhasil Diverifikasi!
              </div>
            </td>
          </tr>

          <!-- Info Produk & Total Belanja -->
          <tr>
            <td>
              <table width="100%%" border="0" cellspacing="0" cellpadding="0" style="margin-bottom: 20px; font-size: 14px; color: #cbd5e1; background: rgba(0,0,0,0.25); border-radius: 10px; padding: 12px 14px;">
                <tr>
                  <td style="padding: 6px 0; color: #94a3b8;">Nomor Invoice</td>
                  <td style="padding: 6px 0; font-family: monospace; font-weight: 700; color: #ffffff; text-align: right;">%s</td>
                </tr>
                <tr>
                  <td style="padding: 6px 0; color: #94a3b8;">Produk</td>
                  <td style="padding: 6px 0; font-weight: 600; color: #60a5fa; text-align: right;">%s (%d item)</td>
                </tr>
                <tr>
                  <td style="padding: 10px 0 4px 0; border-top: 1px dashed rgba(255,255,255,0.12); font-weight: 700; color: #ffffff; font-size: 14px;">TOTAL LUNAS</td>
                  <td style="padding: 10px 0 4px 0; border-top: 1px dashed rgba(255,255,255,0.12); font-family: monospace; font-weight: 800; color: #10b981; font-size: 17px; text-align: right;">%s</td>
                </tr>
              </table>
            </td>
          </tr>

          <!-- KODE AMBIL PESANAN HIGHLIGHT -->
          <tr>
            <td>
              <div style="background-color: #090e1a; border: 1px solid rgba(168, 85, 247, 0.45); border-radius: 12px; padding: 20px; text-align: center; margin-bottom: 20px;">
                <div style="font-size: 12px; font-weight: 700; text-transform: uppercase; color: #c084fc; letter-spacing: 1px; margin-bottom: 8px;">
                  🔑 KODE AMBIL PESANAN ANDA
                </div>
                <div style="display: inline-block; font-family: 'JetBrains Mono', monospace; font-size: 26px; font-weight: 900; color: #ffffff; letter-spacing: 2px; background: rgba(168, 85, 247, 0.15); padding: 8px 18px; border-radius: 8px; border: 1px dashed rgba(168, 85, 247, 0.6);">
                  %s
                </div>
                <div style="font-size: 12px; color: #94a3b8; margin-top: 10px; line-height: 1.4;">
                  Salin kode di atas jika Anda ingin mengambil pesanan secara manual di menu Klaim Produk Digimest.
                </div>
              </div>
            </td>
          </tr>

          <!-- DIRECT LINK CTA BOX -->
          <tr>
            <td>
              <div style="background-color: rgba(37, 99, 235, 0.08); border: 1px solid rgba(59, 130, 246, 0.3); border-radius: 12px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <div style="font-size: 13px; font-weight: 700; color: #93c5fd; margin-bottom: 6px;">
                  🔗 ATAU KLIK LINK DI BAWAH INI UNTUK AMBIL PRODUK ANDA:
                </div>
                <div style="font-size: 12px; color: #cbd5e1; margin-bottom: 16px;">
                  Anda dapat langsung membuka lisensi tanpa perlu memasukkan kode lagi:
                </div>
                <div>
                  <a href="%s" target="_blank" style="display: inline-block; background-color: #2563eb; color: #ffffff; text-decoration: none; font-weight: 700; font-size: 14px; padding: 12px 28px; border-radius: 8px; box-shadow: 0 4px 14px rgba(37, 99, 235, 0.4);">
                    Buka & Ambil Produk Sekarang ➔
                  </a>
                </div>
                <div style="margin-top: 12px; font-size: 11px; color: #94a3b8; word-break: break-all;">
                  Tautan langsung: <a href="%s" target="_blank" style="color: #60a5fa; text-decoration: underline;">%s</a>
                </div>
              </div>
            </td>
          </tr>

          <!-- Panduan 2 Langkah -->
          <tr>
            <td style="background: rgba(255,255,255,0.03); border-radius: 10px; padding: 14px 16px; margin-bottom: 20px;">
              <div style="font-size: 12px; font-weight: 700; color: #e2e8f0; margin-bottom: 6px;">
                💡 Cara Menggunakan:
              </div>
              <div style="font-size: 12px; color: #94a3b8; line-height: 1.6;">
                1. Klik link langsung di atas, lisensi pesanan Anda akan langsung terbuka seketika.<br>
                2. Atau kunjungi situs <strong>Digimest.tech</strong>, pilih menu <strong>Klaim Produk</strong>, lalu masukkan Kode Ambil Pesanan Anda: <strong>%s</strong>.
              </div>
            </td>
          </tr>

          <!-- Footer Note -->
          <tr>
            <td style="border-top: 1px solid rgba(255,255,255,0.08); padding-top: 16px; margin-top: 20px; font-size: 12px; color: #64748b; line-height: 1.5;">
              Butuh bantuan atau garansi produk? Hubungi CS Telegram 24/7 kami: <a href="https://t.me/Digimestsupport" target="_blank" style="color: #38bdf8; text-decoration: underline;">@Digimestsupport</a><br>
              Email ini dikirim otomatis oleh Digimest via domain resmi <strong>Digimest.tech</strong>.
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>
`, invoice, productName, qty, formattedTotal, claimCode, claimURL, claimURL, claimURL, claimCode)

	payload := ResendEmailRequest{
		From:    fromEmail,
		To:      []string{toEmail},
		Subject: subject,
		HTML:    htmlContent,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode resend payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create resend request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call resend api: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var resendResp ResendEmailResponse
	_ = json.Unmarshal(respBody, &resendResp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := resendResp.Error.Message
		if errMsg == "" {
			errMsg = string(respBody)
		}
		return fmt.Errorf("resend returned status %d: %s", resp.StatusCode, errMsg)
	}

	log.Printf("[RESEND SUCCESS] Email dikirim ke %s (ID: %s)", toEmail, resendResp.ID)
	return nil
}

func formatNumber(n int64) string {
	in := fmt.Sprintf("%d", n)
	var out []string
	for i, c := range in {
		if (len(in)-i)%3 == 0 && i != 0 {
			out = append(out, ".")
		}
		out = append(out, string(c))
	}
	return strings.Join(out, "")
}
