package cloudinary

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/config"
)

type Client struct {
	cfg  config.Cloudinary
	http *http.Client
	base string
}

type UploadResult struct {
	URL       string `json:"url"`
	SecureURL string `json:"secure_url"`
	PublicID  string `json:"public_id"`
}

func New(cfg config.Cloudinary) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 60 * time.Second},
		base: "https://api.cloudinary.com",
	}
}

func (c *Client) Enabled() bool { return c.cfg.Enabled() }

func (c *Client) SetHTTPClient(h *http.Client) { c.http = h }
func (c *Client) SetBaseURL(u string)          { c.base = strings.TrimRight(u, "/") }

// Upload sends image bytes to Cloudinary (server-side signed upload) and returns the hosted URL.
func (c *Client) Upload(filename string, data []byte, folder string) (UploadResult, error) {
	if !c.Enabled() {
		return UploadResult{}, fmt.Errorf("cloudinary is not configured")
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	params := map[string]string{"timestamp": ts}
	if strings.TrimSpace(folder) != "" {
		params["folder"] = strings.TrimSpace(folder)
	}
	sig := sign(params, c.cfg.APISecret)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range params {
		_ = w.WriteField(k, v)
	}
	_ = w.WriteField("api_key", c.cfg.APIKey)
	_ = w.WriteField("signature", sig)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return UploadResult{}, err
	}
	if _, err := io.Copy(part, bytes.NewReader(data)); err != nil {
		return UploadResult{}, err
	}
	if err := w.Close(); err != nil {
		return UploadResult{}, err
	}

	cloud := strings.TrimSpace(c.cfg.CloudName)
	endpoint := fmt.Sprintf("%s/v1_1/%s/image/upload", c.base, cloud)
	req, err := http.NewRequest(http.MethodPost, endpoint, &buf)
	if err != nil {
		return UploadResult{}, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.http.Do(req)
	if err != nil {
		return UploadResult{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode >= 300 {
		return UploadResult{}, fmt.Errorf("cloudinary upload failed: %s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		SecureURL string `json:"secure_url"`
		URL       string `json:"url"`
		PublicID  string `json:"public_id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return UploadResult{}, fmt.Errorf("cloudinary upload: invalid response")
	}
	u := out.SecureURL
	if u == "" {
		u = out.URL
	}
	return UploadResult{URL: u, SecureURL: out.SecureURL, PublicID: out.PublicID}, nil
}

func sign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if params[k] != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	h := sha1.Sum([]byte(strings.Join(parts, "&") + secret))
	return hex.EncodeToString(h[:])
}
