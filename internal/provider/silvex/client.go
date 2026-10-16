package silvex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"storeku/internal/config"
	"storeku/internal/provider"
)

// Client handles low-level HTTP communication with Silvex AI Reseller API.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewClient creates a new Silvex API client.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) getBaseURL() string {
	baseURL := strings.TrimRight(c.cfg.SilvexBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://silvex-ai.com/api/public/reseller"
	}
	return baseURL
}

// GetBalance queries the current USD wallet balance from Silvex AI.
func (c *Client) GetBalance(ctx context.Context) (float64, error) {
	if c.cfg.SilvexAPIKey == "" {
		return 0, fmt.Errorf("SILVEX_API_KEY is not configured")
	}

	url := fmt.Sprintf("%s/balance", c.getBaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("x-api-key", c.cfg.SilvexAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to silvex: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("silvex error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var balResp BalanceResponse
	if err := json.Unmarshal(body, &balResp); err == nil && (balResp.OK || balResp.Balance > 0) {
		return balResp.Balance, nil
	}

	// Try parsing as simple float or general map
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err == nil {
		if b, ok := raw["balance"].(float64); ok {
			return b, nil
		}
	}

	return 0, fmt.Errorf("unexpected balance response: %s", string(body))
}

// GetProducts queries the product catalog from Silvex AI.
func (c *Client) GetProducts(ctx context.Context) ([]RawProduct, error) {
	if c.cfg.SilvexAPIKey == "" {
		return nil, fmt.Errorf("SILVEX_API_KEY is not configured")
	}

	url := fmt.Sprintf("%s/products", c.getBaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-api-key", c.cfg.SilvexAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch silvex products: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("silvex returned status %d: %s", resp.StatusCode, string(body))
	}

	// 1. Try decoding as direct slice: []RawProduct
	var list []RawProduct
	if err := json.Unmarshal(body, &list); err == nil && len(list) > 0 {
		return list, nil
	}

	// 2. Try decoding as wrapper: {"ok": true, "products": [...]} or {"data": [...]}
	var wrapper struct {
		OK       bool         `json:"ok"`
		Products []RawProduct `json:"products"`
		Data     []RawProduct `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil {
		if len(wrapper.Products) > 0 {
			return wrapper.Products, nil
		}
		if len(wrapper.Data) > 0 {
			return wrapper.Data, nil
		}
	}

	return nil, fmt.Errorf("failed to parse silvex products: %s", string(body))
}

// CreateOrder places an order to Silvex AI.
func (c *Client) CreateOrder(ctx context.Context, productID string, qty int) (*OrderResponse, error) {
	if c.cfg.SilvexAPIKey == "" {
		return nil, fmt.Errorf("SILVEX_API_KEY is not configured")
	}

	url := fmt.Sprintf("%s/order", c.getBaseURL())
	payload := OrderRequest{
		ProductID: productID,
		Qty:       qty,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.cfg.SilvexAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send order to silvex: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ordResp OrderResponse
	if err := json.Unmarshal(body, &ordResp); err != nil {
		return nil, fmt.Errorf("failed to parse silvex order response: %w (body: %s)", err, string(body))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errMsg := ordResp.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("silvex order rejected: %s", errMsg)
	}

	return &ordResp, nil
}

// CheckConnection tests upstream Silvex AI credentials and retrieves balance.
func (c *Client) CheckConnection(ctx context.Context) (*provider.HealthResult, error) {
	apiKey := strings.TrimSpace(c.cfg.SilvexAPIKey)
	baseURL := c.getBaseURL()

	maskedKey := "(belum diatur di .env)"
	if apiKey != "" {
		if len(apiKey) <= 6 {
			maskedKey = "***"
		} else {
			maskedKey = apiKey[:3] + "..." + apiKey[len(apiKey)-3:]
		}
	}

	result := &provider.HealthResult{
		Success:      false,
		Configured:   apiKey != "",
		ProviderID:   "SILVEX",
		ProviderName: "Silvex AI Reseller",
		Currency:     "USD",
		HTTPStatus:   0,
		Balance:      0,
		TotalOrders:  "-",
	}

	if apiKey == "" {
		result.Message = "SILVEX_API_KEY belum dikonfigurasi di file .env VPS."
		return result, fmt.Errorf("%s", result.Message)
	}

	url := fmt.Sprintf("%s/balance", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		result.Message = fmt.Sprintf("Gagal membuat request: %v", err)
		return result, err
	}

	req.Header.Set("x-api-key", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		result.Message = fmt.Sprintf("Gagal menghubungi server Silvex AI: %v", err)
		return result, err
	}
	defer resp.Body.Close()

	result.HTTPStatus = resp.StatusCode

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Message = fmt.Sprintf("Gagal membaca respon Silvex AI: %v", err)
		return result, err
	}
	result.RawResponse = string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		result.Message = fmt.Sprintf("Silvex AI menolak request (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
		return result, fmt.Errorf("%s", result.Message)
	}

	var balResp BalanceResponse
	if err := json.Unmarshal(bodyBytes, &balResp); err == nil && (balResp.OK || balResp.Balance > 0) {
		result.Balance = balResp.Balance
	} else {
		var raw map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &raw); err == nil {
			if b, ok := raw["balance"].(float64); ok {
				result.Balance = b
			}
		}
	}

	result.Success = true
	result.Message = fmt.Sprintf("Koneksi Silvex AI sukses! Kunci: %s | Saldo wallet: $%.2f USD", maskedKey, result.Balance)

	return result, nil
}
