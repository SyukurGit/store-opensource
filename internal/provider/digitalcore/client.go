package digitalcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"storeku/internal/config"
	"storeku/internal/provider"
)

// RawProduct matches the structure returned by DigitalCore /api/public/products.
type RawProduct struct {
	ID          interface{} `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Logo        string      `json:"logo"`
	Region      string      `json:"region"`
	Status      string      `json:"status"`
	Stock       int         `json:"stock"`
	StockCapped bool        `json:"stockCapped"`
	Warranty    string      `json:"warranty"`
	Desc        string      `json:"desc"`
	PriceFrom   float64     `json:"priceFrom"`
	Tiers       []struct {
		MinQty int     `json:"minQty"`
		MaxQty *int    `json:"maxQty"`
		Price  float64 `json:"price"`
	} `json:"tiers"`
}

// BuyResponse matches the response of POST /api/user/buy.
type BuyResponse struct {
	Success     bool     `json:"success"`
	OrderID     string   `json:"order_id"`
	ViewURL     string   `json:"view_url"`
	TotalAmount float64  `json:"total_amount"`
	Quantity    int      `json:"quantity"`
	Items       []string `json:"items"`
	Msg         string   `json:"msg"`
}

// UserMeResponse matches GET /api/user/me.
type UserMeResponse struct {
	Balance     float64 `json:"balance"`
	TotalOrders string  `json:"total_orders"`
}

// Client handles low-level HTTP communication with DigitalCore API.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewClient creates a new DigitalCore API client.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) getBaseURL() string {
	baseURL := strings.TrimRight(c.cfg.DigitalCoreBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://digitalcore.top"
	}
	return baseURL
}

// FetchRawProducts gets the active catalog from DigitalCore.
func (c *Client) FetchRawProducts(ctx context.Context) ([]RawProduct, error) {
	url := fmt.Sprintf("%s/api/public/products", c.getBaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.cfg.DigitalCoreAPIKey != "" {
		req.Header.Set("Api-Key", c.cfg.DigitalCoreAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("digitalcore returned status %d: %s", resp.StatusCode, string(body))
	}

	var rawProds []RawProduct
	if err := json.NewDecoder(resp.Body).Decode(&rawProds); err != nil {
		return nil, fmt.Errorf("failed to decode products: %w", err)
	}

	return rawProds, nil
}

// BuyProduct executes a purchase transaction on DigitalCore.
func (c *Client) BuyProduct(ctx context.Context, slugOrID string, count int) (*BuyResponse, error) {
	if c.cfg.DigitalCoreAPIKey == "" {
		return nil, fmt.Errorf("DIGITALCORE_API_KEY is not configured")
	}

	targetSlug := strings.TrimSpace(slugOrID)
	targetSlug = strings.ReplaceAll(targetSlug, " ", "_")
	targetSlug = strings.ReplaceAll(targetSlug, "-", "_")

	buyURL := fmt.Sprintf("%s/api/user/buy?id=%s&count=%d", c.getBaseURL(), url.QueryEscape(targetSlug), count)
	payload := map[string]interface{}{
		"id":    targetSlug,
		"count": count,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buyURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.cfg.DigitalCoreAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error connecting to digitalcore: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var buyResp BuyResponse
	if err := json.Unmarshal(body, &buyResp); err != nil {
		return nil, fmt.Errorf("failed to parse buy response: %w (body: %s)", err, string(body))
	}

	if resp.StatusCode != http.StatusOK || !buyResp.Success {
		errMsg := buyResp.Msg
		if errMsg == "" {
			errMsg = fmt.Sprintf("status code %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("digitalcore buy failed: %s", errMsg)
	}

	return &buyResp, nil
}

// CheckConnection tests upstream DigitalCore credentials and account status.
func (c *Client) CheckConnection(ctx context.Context) (*provider.HealthResult, error) {
	apiKey := strings.TrimSpace(c.cfg.DigitalCoreAPIKey)
	baseURL := c.getBaseURL()

	result := &provider.HealthResult{
		Success:      false,
		Configured:   apiKey != "",
		ProviderID:   "DIGITALCORE",
		ProviderName: "DigitalCore Upstream",
		Currency:     "USD",
		HTTPStatus:   0,
		Balance:      0,
		TotalOrders:  "0",
	}

	if apiKey == "" {
		result.Message = "DIGITALCORE_API_KEY belum dikonfigurasi di file .env VPS."
		return result, fmt.Errorf("%s", result.Message)
	}

	checkURL := fmt.Sprintf("%s/api/user/me", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		result.Message = fmt.Sprintf("Gagal membuat request: %v", err)
		return result, err
	}

	req.Header.Set("Api-Key", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		result.Message = fmt.Sprintf("Gagal menghubungi server DigitalCore: %v", err)
		return result, err
	}
	defer resp.Body.Close()

	result.HTTPStatus = resp.StatusCode

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Message = fmt.Sprintf("Gagal membaca respon DigitalCore: %v", err)
		return result, err
	}
	result.RawResponse = string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		result.Message = fmt.Sprintf("DigitalCore menolak request (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
		return result, fmt.Errorf("%s", result.Message)
	}

	balance, totalOrders, err := parseUserMe(bodyBytes)
	if err != nil {
		result.Message = fmt.Sprintf("HTTP 200 OK tetapi gagal parsing format JSON: %v", err)
		return result, err
	}

	result.Success = true
	result.Balance = balance
	result.TotalOrders = totalOrders
	result.Message = fmt.Sprintf("Koneksi DigitalCore sukses! Saldo bot: $%.2f USD | Total order: %s", balance, totalOrders)

	return result, nil
}

// GetUserAccount queries current balance and total orders.
func (c *Client) GetUserAccount(ctx context.Context) (*UserMeResponse, error) {
	if c.cfg.DigitalCoreAPIKey == "" {
		return &UserMeResponse{Balance: 0, TotalOrders: "0"}, nil
	}

	checkURL := fmt.Sprintf("%s/api/user/me", c.getBaseURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Api-Key", c.cfg.DigitalCoreAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("digitalcore /me error %d: %s", resp.StatusCode, string(body))
	}

	balance, totalOrders, err := parseUserMe(body)
	if err != nil {
		return nil, err
	}

	return &UserMeResponse{
		Balance:     balance,
		TotalOrders: totalOrders,
	}, nil
}

func parseUserMe(bodyBytes []byte) (balance float64, totalOrders string, err error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return 0, "0", fmt.Errorf("failed to decode JSON response: %w", err)
	}

	lookupMaps := []map[string]interface{}{raw}
	for _, nestedKey := range []string{"data", "user", "account", "result"} {
		if nested, ok := raw[nestedKey].(map[string]interface{}); ok {
			lookupMaps = append(lookupMaps, nested)
		}
	}

	balanceFound := false
	balanceKeys := []string{"balance", "wallet", "saldo", "credit", "amount"}
	for _, m := range lookupMaps {
		for _, k := range balanceKeys {
			if v, ok := m[k]; ok {
				if f, ok := extractFloat(v); ok {
					balance = f
					balanceFound = true
					break
				}
			}
		}
		if balanceFound {
			break
		}
	}

	orderKeys := []string{"total_orders", "totalOrders", "orders", "order_count", "total"}
	for _, m := range lookupMaps {
		for _, k := range orderKeys {
			if v, ok := m[k]; ok {
				totalOrders = extractString(v)
				break
			}
		}
		if totalOrders != "" && totalOrders != "0" {
			break
		}
	}
	if totalOrders == "" {
		totalOrders = "0"
	}

	return balance, totalOrders, nil
}

func extractFloat(val interface{}) (float64, bool) {
	if val == nil {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	case string:
		clean := strings.TrimSpace(v)
		clean = strings.TrimPrefix(clean, "$")
		clean = strings.TrimSpace(clean)
		if f, err := strconv.ParseFloat(clean, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func extractString(val interface{}) string {
	if val == nil {
		return "0"
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}
