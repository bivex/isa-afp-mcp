package afpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
)

const (
	DefaultBaseURL   = "https://isa-afp.org"
	DefaultUserAgent = "isa-afp-mcp/1.0 (+https://github.com/isa-afp/isa-afp-mcp)"
	DefaultTimeout   = 30 * time.Second
)

// Client implements port.AFPClient for isa-afp.org HTTP REST endpoints.
type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

// Option configures Client.
type Option func(*Client)

// WithBaseURL overrides the base URL (useful for testing or local mirrors).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithUserAgent configures a custom User-Agent.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// New creates an AFPClient with HTTP connection pooling.
func New(opts ...Option) *Client {
	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false, // Automatic gzip/deflate support
	}

	c := &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   DefaultTimeout,
		},
		userAgent: DefaultUserAgent,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

var _ port.AFPClient = (*Client)(nil)

func (c *Client) doRequest(ctx context.Context, endpoint string) (*http.Response, error) {
	url := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request %s: %w", url, err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json, application/gzip, */*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request %s: %w", url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("request %s returned status %d", url, resp.StatusCode)
	}

	return resp, nil
}

// FetchEntries retrieves all AFP entries metadata.
func (c *Client) FetchEntries(ctx context.Context) ([]entry.Entry, error) {
	resp, err := c.doRequest(ctx, "/entries/index.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var entries []entry.Entry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode entries json: %w", err)
	}
	return entries, nil
}

// FetchTopics retrieves the hierarchy of topics.
func (c *Client) FetchTopics(ctx context.Context) ([]entry.Topic, error) {
	resp, err := c.doRequest(ctx, "/topics/index.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var topics []entry.Topic
	if err := json.NewDecoder(resp.Body).Decode(&topics); err != nil {
		return nil, fmt.Errorf("decode topics json: %w", err)
	}
	return topics, nil
}

// FetchAuthors retrieves contributor profiles.
func (c *Client) FetchAuthors(ctx context.Context) ([]entry.Author, error) {
	resp, err := c.doRequest(ctx, "/authors/index.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var authors []entry.Author
	if err := json.NewDecoder(resp.Body).Decode(&authors); err != nil {
		return nil, fmt.Errorf("decode authors json: %w", err)
	}
	return authors, nil
}

// FetchKeywords retrieves the math/CS terms dictionary.
func (c *Client) FetchKeywords(ctx context.Context) ([]entry.Keyword, error) {
	resp, err := c.doRequest(ctx, "/data/keywords.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var keywords []entry.Keyword
	if err := json.NewDecoder(resp.Body).Decode(&keywords); err != nil {
		return nil, fmt.Errorf("decode keywords json: %w", err)
	}
	return keywords, nil
}

// DownloadPackageArchive downloads the verified package archive (.tar.gz) for an entry.
// Caller is responsible for closing the returned ReadCloser.
func (c *Client) DownloadPackageArchive(ctx context.Context, entryShortname string) (io.ReadCloser, error) {
	endpoint := fmt.Sprintf("/release/afp-%s-current.tar.gz", entryShortname)
	resp, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("download archive for %s: %w", entryShortname, err)
	}
	return resp.Body, nil
}
