package summary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

var ErrInvalidResponse = errors.New("LLM returned an invalid response")

type Config struct {
	BaseURL string
	Model   string
	APIKey  string
	Timeout time.Duration
}

type Request struct {
	Language string   `json:"language"`
	Kind     string   `json:"kind"`
	Facts    []string `json:"facts"`
}

type Client struct {
	openai openai.Client
	model  string
}

func NewClient(config Config) (*Client, error) {
	if strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.Model) == "" || config.Timeout <= 0 {
		return nil, fmt.Errorf("summary client configuration is incomplete")
	}
	apiKey := config.APIKey
	if apiKey == "" {
		apiKey = "not-needed"
	}
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(strings.TrimRight(config.BaseURL, "/")),
		option.WithHTTPClient(&http.Client{Timeout: config.Timeout}),
		option.WithMaxRetries(0),
	)
	return &Client{openai: client, model: config.Model}, nil
}

func (c *Client) Generate(ctx context.Context, request Request) (string, error) {
	facts, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encode summary facts: %w", err)
	}
	content, err := c.complete(ctx,
		"Return one JSON object with a concise summary field. Base every claim only on the supplied facts.",
		string(facts), 800)
	if err != nil {
		return "", err
	}
	var result struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil || strings.TrimSpace(result.Summary) == "" {
		return "", ErrInvalidResponse
	}
	return strings.TrimSpace(result.Summary), nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	content, err := c.complete(ctx,
		"Return JSON only.",
		`Reply with {"ok":true}.`, 16)
	if err != nil {
		return err
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil || !result.OK {
		return ErrInvalidResponse
	}
	return nil
}

func (c *Client) complete(ctx context.Context, instruction, input string, maxTokens int64) (string, error) {
	jsonFormat := shared.NewResponseFormatJSONObjectParam()
	completion, err := c.openai.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.DeveloperMessage(instruction),
			openai.UserMessage(input),
		},
		Model:       shared.ChatModel(c.model),
		MaxTokens:   openai.Int(maxTokens),
		Temperature: openai.Float(0.2),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &jsonFormat,
		},
	})
	if err != nil {
		return "", fmt.Errorf("create chat completion: %w", err)
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return "", ErrInvalidResponse
	}
	return completion.Choices[0].Message.Content, nil
}
