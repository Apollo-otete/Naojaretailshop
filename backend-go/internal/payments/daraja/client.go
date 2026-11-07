// Package daraja implements the Safaricom Daraja (M-Pesa) STK Push client (P1).
package daraja

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
)

type Client struct {
	cfg  config.Mpesa
	http *http.Client
	base string
}

func New(cfg config.Mpesa) *Client {
	base := "https://sandbox.safaricom.co.ke"
	if strings.EqualFold(cfg.Environment, "production") {
		base = "https://api.safaricom.co.ke"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}, base: base}
}

func (c *Client) Enabled() bool { return c.cfg.Enabled() }

type tokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"`
}

func (c *Client) token(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/oauth/v1/generate?grant_type=client_credentials", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.cfg.ConsumerKey, c.cfg.ConsumerSecret)
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("daraja oauth: %s: %s", res.Status, string(body))
	}
	var tr tokenResp
	if err := json.Unmarshal(body, &tr); err != nil || tr.AccessToken == "" {
		return "", fmt.Errorf("daraja oauth: invalid response")
	}
	return tr.AccessToken, nil
}

type STKRequest struct {
	AmountCents int64
	Phone       string // 2547XXXXXXXX
	Reference   string
	Description string
	CallbackURL string
}

type STKResponse struct {
	MerchantRequestID   string
	CheckoutRequestID   string
	ResponseCode        string
	ResponseDescription string
	CustomerMessage     string
	Raw                 []byte
}

func (c *Client) STKPush(ctx context.Context, in STKRequest) (STKResponse, error) {
	tok, err := c.token(ctx)
	if err != nil {
		return STKResponse{}, err
	}
	ts := time.Now().In(time.FixedZone("EAT", 3*3600)).Format("20060102150405")
	password := base64.StdEncoding.EncodeToString([]byte(c.cfg.Shortcode + c.cfg.Passkey + ts))
	amount := in.AmountCents / 100
	if amount < 1 {
		amount = 1
	}
	cb := in.CallbackURL
	if cb == "" {
		cb = c.cfg.CallbackURL
	}
	payload := map[string]any{
		"BusinessShortCode": c.cfg.Shortcode,
		"Password":          password,
		"Timestamp":         ts,
		"TransactionType":   "CustomerPayBillOnline",
		"Amount":            amount,
		"PartyA":            in.Phone,
		"PartyB":            c.cfg.Shortcode,
		"PhoneNumber":       in.Phone,
		"CallBackURL":       cb,
		"AccountReference":  in.Reference,
		"TransactionDesc":   truncate(in.Description, 13),
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/mpesa/stkpush/v1/processrequest", bytes.NewReader(b))
	if err != nil {
		return STKResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return STKResponse{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var out struct {
		MerchantRequestID   string `json:"MerchantRequestID"`
		CheckoutRequestID   string `json:"CheckoutRequestID"`
		ResponseCode        string `json:"ResponseCode"`
		ResponseDescription string `json:"ResponseDescription"`
		CustomerMessage     string `json:"CustomerMessage"`
	}
	_ = json.Unmarshal(body, &out)
	if res.StatusCode >= 300 || (out.ResponseCode != "" && out.ResponseCode != "0") {
		msg := out.ResponseDescription
		if msg == "" {
			msg = out.CustomerMessage
		}
		if msg == "" {
			msg = string(body)
		}
		return STKResponse{MerchantRequestID: out.MerchantRequestID, CheckoutRequestID: out.CheckoutRequestID, ResponseCode: out.ResponseCode, ResponseDescription: out.ResponseDescription, CustomerMessage: out.CustomerMessage, Raw: body}, fmt.Errorf("daraja stk: %s", msg)
	}
	return STKResponse{MerchantRequestID: out.MerchantRequestID, CheckoutRequestID: out.CheckoutRequestID, ResponseCode: out.ResponseCode, ResponseDescription: out.ResponseDescription, CustomerMessage: out.CustomerMessage, Raw: body}, nil
}

// CallbackResult is the normalized outcome of a Daraja STK callback.
type CallbackResult struct {
	MerchantRequestID string
	CheckoutRequestID string
	ResultCode        int
	ResultDesc        string
	AmountCents       *int64 // whole KES * 100 when present
	MpesaReceipt      string
	Phone             string
	Raw               []byte
}

func (r CallbackResult) Success() bool { return r.ResultCode == 0 && r.MpesaReceipt != "" }

// ParseCallback parses Safaricom's STK callback envelope into a normalized result.
// It never trusts amounts for settlement until correlated and checked against the order.
func ParseCallback(body []byte) (CallbackResult, error) {
	var env struct {
		Body struct {
			StkCallback struct {
				MerchantRequestID string `json:"MerchantRequestID"`
				CheckoutRequestID string `json:"CheckoutRequestID"`
				ResultCode        int    `json:"ResultCode"`
				ResultDesc        string `json:"ResultDesc"`
				CallbackMetadata  struct {
					Item []struct {
						Name  string      `json:"Name"`
						Value interface{} `json:"Value"`
					} `json:"Item"`
				} `json:"CallbackMetadata"`
			} `json:"stkCallback"`
		} `json:"Body"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return CallbackResult{}, err
	}
	cb := env.Body.StkCallback
	r := CallbackResult{
		MerchantRequestID: cb.MerchantRequestID,
		CheckoutRequestID: cb.CheckoutRequestID,
		ResultCode:        cb.ResultCode,
		ResultDesc:        cb.ResultDesc,
		Raw:               body,
	}
	for _, it := range cb.CallbackMetadata.Item {
		switch it.Name {
		case "MpesaReceiptNumber":
			if s, ok := it.Value.(string); ok {
				r.MpesaReceipt = s
			}
		case "Amount":
			if v, ok := asInt64(it.Value); ok {
				cents := v * 100
				r.AmountCents = &cents
			}
		case "PhoneNumber":
			switch v := it.Value.(type) {
			case string:
				r.Phone = v
			case float64:
				r.Phone = fmt.Sprintf("%.0f", v)
			case json.Number:
				r.Phone = v.String()
			}
		}
	}
	return r, nil
}

func asInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case string:
		// rare
		var i int64
		_, err := fmt.Sscan(n, &i)
		return i, err == nil
	default:
		return 0, false
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// BaseURL exposes the resolved API base (useful for tests with httptest).
func (c *Client) BaseURL() string              { return c.base }
func (c *Client) SetBaseURL(u string)          { c.base = strings.TrimRight(u, "/") }
func (c *Client) SetHTTPClient(h *http.Client) { c.http = h }

var _ = url.Values{}
