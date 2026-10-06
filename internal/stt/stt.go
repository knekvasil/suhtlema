package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// Client talks to a running whisper-server.
type Client struct {
	Endpoint string
	Language string
	HTTP     *http.Client
}

// New returns a Spanish STT client.
func New(endpoint string) *Client {
	return &Client{
		Endpoint: endpoint,
		Language: "es",
		HTTP:     &http.Client{Timeout: 120 * time.Second},
	}
}

// Transcribe sends a 16 kHz WAV file and returns the recognised text.
func (c *Client) Transcribe(ctx context.Context, wavPath string) (string, time.Duration, error) {
	f, err := os.Open(wavPath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", wavPath)
	if err != nil {
		return "", 0, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", 0, err
	}
	_ = mw.WriteField("language", c.Language)
	_ = mw.WriteField("response_format", "json")
	_ = mw.WriteField("prompt", "Hola, ¿qué tal? Ayer fui, entonces, gracias, bueno, pues. Sí, no, ¿eh?")
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/inference", &body)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	start := time.Now()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("whisper-server returned %s", resp.Status)
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", 0, err
	}
	return out.Text, time.Since(start), nil
}
