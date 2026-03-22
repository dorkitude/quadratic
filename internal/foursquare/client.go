package foursquare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"quadratic/internal/config"
)

const authBaseURL = "https://foursquare.com/oauth2"
const apiBaseURL = "https://api.foursquare.com/v2"

type Client struct {
	httpClient *http.Client
	cfg        *config.Config
}

type Checkin struct {
	ID         string                 `json:"id"`
	CreatedAt  int64                  `json:"createdAt"`
	Type       string                 `json:"type"`
	Shout      string                 `json:"shout,omitempty"`
	TimeZone   string                 `json:"timeZone,omitempty"`
	Venue      map[string]any         `json:"venue,omitempty"`
	Raw        map[string]any         `json:"-"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type CheckinsPage struct {
	Items []Checkin
	Count int
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Login(ctx context.Context) (string, error) {
	if c.cfg.ClientID == "" || c.cfg.ClientSecret == "" || c.cfg.RedirectURL == "" {
		return "", errors.New("client_id, client_secret, and redirect_url must be set in config or environment")
	}

	callbackURL, err := url.Parse(c.cfg.RedirectURL)
	if err != nil {
		return "", fmt.Errorf("parse redirect_url: %w", err)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, "Login complete. You can close this window.\n")
		select {
		case codeCh <- code:
		default:
		}
	})

	listener, err := net.Listen("tcp", callbackURL.Host)
	if err != nil {
		return "", fmt.Errorf("listen for OAuth callback: %w", err)
	}
	defer listener.Close()

	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()
	defer server.Shutdown(context.Background())

	authURL := c.authURL()
	_ = openBrowser(authURL)
	fmt.Printf("Open this URL to authorize:\n%s\n", authURL)

	var code string
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case err := <-errCh:
		return "", err
	case code = <-codeCh:
	}

	return c.exchangeCode(ctx, code)
}

func (c *Client) authURL() string {
	values := url.Values{}
	values.Set("client_id", c.cfg.ClientID)
	values.Set("response_type", "code")
	values.Set("redirect_uri", c.cfg.RedirectURL)
	return authBaseURL + "/authenticate?" + values.Encode()
}

func (c *Client) exchangeCode(ctx context.Context, code string) (string, error) {
	values := url.Values{}
	values.Set("client_id", c.cfg.ClientID)
	values.Set("client_secret", c.cfg.ClientSecret)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", c.cfg.RedirectURL)
	values.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authBaseURL+"/access_token?"+values.Encode(), nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("exchange code: status %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode access token response: %w", err)
	}
	if payload.AccessToken == "" {
		return "", errors.New("access token missing in response")
	}
	return payload.AccessToken, nil
}

func (c *Client) FetchCheckins(ctx context.Context, limit, offset int) (*CheckinsPage, error) {
	if c.cfg.AccessToken == "" {
		return nil, errors.New("missing access token: run `quadratic login` first")
	}

	reqURL := fmt.Sprintf("%s/users/self/checkins?v=20260322&limit=%d&offset=%d", apiBaseURL, limit, offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get checkins: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("get checkins: status %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Response struct {
			Checkins struct {
				Count int              `json:"count"`
				Items []map[string]any `json:"items"`
			} `json:"checkins"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode checkins response: %w", err)
	}

	page := &CheckinsPage{
		Count: payload.Response.Checkins.Count,
		Items: make([]Checkin, 0, len(payload.Response.Checkins.Items)),
	}
	for _, item := range payload.Response.Checkins.Items {
		checkin := Checkin{Raw: item}
		if id, ok := item["id"].(string); ok {
			checkin.ID = id
		}
		if createdAt, ok := item["createdAt"].(float64); ok {
			checkin.CreatedAt = int64(createdAt)
		}
		if typeValue, ok := item["type"].(string); ok {
			checkin.Type = typeValue
		}
		if shout, ok := item["shout"].(string); ok {
			checkin.Shout = shout
		}
		if tz, ok := item["timeZone"].(string); ok {
			checkin.TimeZone = tz
		}
		if venue, ok := item["venue"].(map[string]any); ok {
			checkin.Venue = venue
		}
		page.Items = append(page.Items, checkin)
	}
	return page, nil
}

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		return nil
	}
	return cmd.Start()
}
