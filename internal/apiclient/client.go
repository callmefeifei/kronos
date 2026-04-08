// Package apiclient provides a shared HTTP client for calling the Kronos REST API.
// It is used by CLI commands and the MCP proxy handler.
package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Client is an authenticated HTTP client for the Kronos API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New creates a Client that authenticates using a short-lived admin JWT
// derived from jwtSecret.
func New(baseURL, jwtSecret string) (*Client, error) {
	token, err := GenerateAdminJWT(jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("generate admin JWT: %w", err)
	}
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// GenerateAdminJWT creates a short-lived admin JWT for CLI / internal usage.
func GenerateAdminJWT(secret string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id":  0,
		"username": "cli-admin",
		"role":     "admin",
		"type":     "access",
		"iss":      "kronos",
		"iat":      now.Unix(),
		"exp":      now.Add(5 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Get performs a GET request and returns the parsed JSON response.
func (c *Client) Get(path string) (map[string]interface{}, error) {
	return c.Do("GET", path, nil)
}

// Post performs a POST request with the given JSON body.
func (c *Client) Post(path string, body []byte) (map[string]interface{}, error) {
	return c.Do("POST", path, body)
}

// Put performs a PUT request with the given JSON body.
func (c *Client) Put(path string, body []byte) (map[string]interface{}, error) {
	return c.Do("PUT", path, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(path string) (map[string]interface{}, error) {
	return c.Do("DELETE", path, nil)
}

// Do performs an HTTP request and returns the parsed JSON response.
func (c *Client) Do(method, path string, body []byte) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(data))
	}

	return result, nil
}

// --- Response parsing helpers ---

// CheckResponse returns an error if the API response code is non-zero.
func CheckResponse(resp map[string]interface{}) error {
	code, _ := resp["code"].(float64)
	if code != 0 {
		msg, _ := resp["message"].(string)
		return fmt.Errorf("API error (code %d): %s", int(code), msg)
	}
	return nil
}

// ExtractData returns the "data" field from a successful API response.
func ExtractData(resp map[string]interface{}) (interface{}, error) {
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return resp["data"], nil
}

// ExtractPagedItems returns the "items" slice from a paged API response.
func ExtractPagedItems(resp map[string]interface{}) ([]interface{}, error) {
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}
	items, ok := data["items"].([]interface{})
	if !ok {
		return []interface{}{}, nil
	}
	return items, nil
}
