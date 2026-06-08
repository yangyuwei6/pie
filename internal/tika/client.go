package tika

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

type Client struct {
	serverURL  string
	httpClient *http.Client
}

func NewClient(serverURL string) *Client {
	return &Client{
		serverURL:  strings.TrimRight(serverURL, "/"),
		httpClient: http.DefaultClient,
	}
}

func (c *Client) ExtractText(reader io.Reader, fileName string) (string, error) {
	req, err := http.NewRequest(http.MethodPut, c.serverURL+"/tika", reader)
	if err != nil {
		return "", fmt.Errorf("create tika request: %w", err)
	}

	req.Header.Set("Accept", "text/plain")
	req.Header.Set("Content-Type", detectMimeType(fileName))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call tika: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("tika returned status %d: %s", resp.StatusCode, string(body))
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return "", fmt.Errorf("read tika response: %w", err)
	}

	return buf.String(), nil
}

func detectMimeType(fileName string) string {
	ext := filepath.Ext(fileName)
	if ext == "" {
		return "application/octet-stream"
	}

	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}
