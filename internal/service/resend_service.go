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
		fromEmail = "StoreKu <orders@syukurapi.online>"
	}

	claimURL := fmt.Sprintf("%s/claim.html?code=%s", strings.TrimRight(s.cfg.StoreBaseURL, "/"), claimCode)
	formattedTotal := fmt.Sprintf("Rp %s", formatNumber(totalAmount))

	subject := fmt.Sprintf("Pembayaran Berhasil — Kode Klaim: %s", claimCode)
	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Pembayaran Berhasil</title>
</head>
<body style="margin: 0; padding: 0; background-color: #0b0f19; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #f8fafc;">
  <table width="100%%" border="0" cellspacing="0" cellpadding="0" style="background-color: #0b0f19; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table width="100%%" border="0" cellspacing="0" cellpadding="0" style="max-width: 540px; background-color: #161e31; border: 1px solid rgba(255,255,255,0.1); border-radius: 16px; overflow: hidden; padding: 32px 28px; text-align: left; box-shadow: 0 10px 25px rgba(0,0,0,0.5);">
          
          <!-- Logo / Header -->
          <tr>
            <td>
              <div style="font-size: 20px; font-weight: 800; color: #ffffff; letter-spacing: -0.5px; margin-bottom: 4px;">
                ⚡ StoreKu Digital
              </div>
              <div style="font-size: 13px; color: #94a3b8; margin-bottom: 24px;">
                Toko Produk & Lisensi Digital Instan
              </div>
            </td>
          </tr>

          <!-- Banner Lunas -->
          <tr>
            <td>
              <div style="background-color: rgba(16, 185, 129, 0.12); border: 1px solid rgba(16, 185, 129, 0.3); border-radius: 10px; padding: 14px 16px; margin-bottom: 24px; color: #34d399; font-size: 14px; font-weight: 700;">
                ✓ Pembayaran Berhasil Diverifikasi!
              </div>
            </td>
          </tr>

          <!-- Info Produk & Total Belanja -->
          <tr>
            <td>
              <table width="100%%" border="0" cellspacing="0" cellpadding="0" style="margin-bottom: 20px; font-size: 14px; color: #cbd5e1;">
                <tr>
                  <td style="padding: 6px 0; color: #94a3b8;">Nomor Invoice</td>
                  <td style="padding: 6px 0; font-family: monospace; font-weight: 700; color: #ffffff; text-align: right;">%s</td>
                </tr>
                <tr>
                  <td style="padding: 6px 0; color: #94a3b8;">Produk</td>
                  <td style="padding: 6px 0; font-weight: 600; color: #ffffff; text-align: right;">%s (%d item)</td>
                </tr>
                <tr>
                  <td style="padding: 10px 0; border-top: 1px dashed rgba(255,255,255,0.1); font-weight: 700; color: #ffffff; font-size: 15px;">TOTAL BELANJA</td>
                  <td style="padding: 10px 0; border-top: 1px dashed rgba(255,255,255,0.1); font-family: monospace; font-weight: 800; color: #10b981; font-size: 18px; text-align: right;">%s</td>
                </tr>
              </table>
            </td>
          </tr>

          <!-- KODE KLAIM HIGHLIGHT -->
          <tr>
            <td>
              <div style="background-color: #090d16; border: 1px solid rgba(168, 85, 247, 0.4); border-radius: 12px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <div style="font-size: 12px; font-weight: 700; text-transform: uppercase; color: #c084fc; letter-spacing: 1px; margin-bottom: 6px;">
                  KODE KLAIM RAHASIA ANDA
                </div>
                <div style="font-family: monospace; font-size: 28px; font-weight: 900; color: #a855f7; letter-spacing: 2px;">
                  %s
                </div>
                <div style="font-size: 12px; color: #94a3b8; margin-top: 6px;">
                  Gunakan kode di atas untuk mengambil lisensi/akun Anda kapan saja.
                </div>
              </div>
            </td>
          </tr>

          <!-- Tombol CTA -->
          <tr>
            <td align="center" style="padding-bottom: 24px;">
              <a href="%s" target="_blank" style="display: inline-block; background-color: #6366f1; color: #ffffff; text-decoration: none; font-weight: 700; font-size: 14px; padding: 12px 28px; border-radius: 8px; box-shadow: 0 4px 12px rgba(99, 102, 241, 0.4);">
                Buka & Ambil Lisensi Sekarang ➔
              </a>
            </td>
          </tr>

          <!-- Footer Note -->
          <tr>
            <td style="border-top: 1px solid rgba(255,255,255,0.08); padding-top: 16px; font-size: 12px; color: #64748b; line-height: 1.5;">
              Email ini dikirim otomatis oleh StoreKu via domain resmi <strong>syukurapi.online</strong>.<br>
              Jika Anda tidak merasa melakukan transaksi ini, abaikan email ini.
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>
`, invoice, productName, qty, formattedTotal, claimCode, claimURL)

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
