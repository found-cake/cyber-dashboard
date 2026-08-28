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

	// section selects batch-summary prompting and stays outside the serialized model payload.
	section bool
}

// mergeRequest carries the section summaries that the merge pass rewrites into one digest.
type mergeRequest struct {
	Language string   `json:"language"`
	Kind     string   `json:"kind"`
	Sections []string `json:"sections"`
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
		option.WithHTTPClient(&http.Client{Timeout: config.Timeout, CheckRedirect: checkSameOriginRedirect}),
		option.WithMaxRetries(0),
	)
	return &Client{openai: client, model: config.Model}, nil
}

func (c *Client) Generate(ctx context.Context, request Request) (string, error) {
	facts, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encode summary facts: %w", err)
	}
	instruction := generateSystemPrompt(request.Language)
	if request.section {
		instruction = generateSectionSystemPrompt(request.Language)
	}
	content, err := c.complete(ctx, instruction, string(facts))
	if err != nil {
		return "", err
	}
	return decodeSummary(content)
}

// merge rewrites independently written section summaries into one readable digest.
func (c *Client) merge(ctx context.Context, request mergeRequest) (string, error) {
	sections, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("encode summary sections: %w", err)
	}
	content, err := c.complete(ctx, mergeSystemPrompt(request.Language), string(sections))
	if err != nil {
		return "", err
	}
	return decodeSummary(content)
}

func decodeSummary(content string) (string, error) {
	var result struct {
		Summary        string `json:"summary"`
		ConciseSummary string `json:"concise_summary"`
	}
	if err := json.Unmarshal([]byte(normalizeJSONContent(content)), &result); err != nil {
		return "", invalidResponse()
	}
	if strings.TrimSpace(result.Summary) == "" {
		result.Summary = result.ConciseSummary
	}
	if strings.TrimSpace(result.Summary) == "" {
		return "", invalidResponse()
	}
	return strings.TrimSpace(result.Summary), nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	content, err := c.complete(ctx,
		testConnectionSystemPrompt,
		testConnectionUserPrompt)
	if err != nil {
		return err
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal([]byte(normalizeJSONContent(content)), &result); err != nil || !result.OK {
		return invalidResponse()
	}
	return nil
}

func (c *Client) complete(ctx context.Context, instruction, input string) (string, error) {
	completion, err := c.openai.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(instruction),
			openai.UserMessage(input),
		},
		Model:       shared.ChatModel(c.model),
		Temperature: openai.Float(0.2),
	})
	if err != nil {
		return "", fmt.Errorf("create chat completion: %w", err)
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return "", ErrInvalidResponse
	}
	return completion.Choices[0].Message.Content, nil
}

func normalizeJSONContent(content string) string {
	return stripJSONLineComments(stripJSONCodeFence(content))
}

func invalidResponse() error {
	return ErrInvalidResponse
}
