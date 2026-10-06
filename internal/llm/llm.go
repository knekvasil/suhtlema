package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Message is one chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Timings mirrors llama-server's reported timings.
type Timings struct {
	PromptN            int     `json:"prompt_n"`
	PromptMS           float64 `json:"prompt_ms"`
	PromptPerSecond    float64 `json:"prompt_per_second"`
	PredictedN         int     `json:"predicted_n"`
	PredictedMS        float64 `json:"predicted_ms"`
	PredictedPerSecond float64 `json:"predicted_per_second"`
}

// Client talks to a running llama-server's OpenAI-compatible endpoint.
type Client struct {
	Endpoint string
	HTTP     *http.Client
}

// New returns a chat client.
func New(endpoint string) *Client {
	return &Client{
		Endpoint: endpoint,
		HTTP:     &http.Client{Timeout: 120 * time.Second},
	}
}

type chatRequest struct {
	Messages           []Message      `json:"messages"`
	MaxTokens          int            `json:"max_tokens"`
	Temperature        float64        `json:"temperature"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
	CachePrompt        bool           `json:"cache_prompt"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Timings Timings `json:"timings"`
}

// Chat sends a conversation and returns the assistant reply plus timings.
func (c *Client) Chat(ctx context.Context, messages []Message, maxTokens int) (string, Timings, time.Duration, error) {
	reqBody, err := json.Marshal(chatRequest{
		Messages:           messages,
		MaxTokens:          maxTokens,
		Temperature:        0.7,
		ChatTemplateKwargs: map[string]any{"enable_thinking": false},
		CachePrompt:        true,
	})
	if err != nil {
		return "", Timings{}, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", Timings{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", Timings{}, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", Timings{}, 0, fmt.Errorf("llama-server returned %s", resp.Status)
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", Timings{}, 0, err
	}
	if len(out.Choices) == 0 {
		return "", Timings{}, 0, fmt.Errorf("no choices in response")
	}
	return out.Choices[0].Message.Content, out.Timings, time.Since(start), nil
}
