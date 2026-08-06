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

type ArticleRequest struct {
	Language string `json:"language"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Body     string `json:"body"`
}

type ArticleAnalysis struct {
	Summary      string `json:"summary"`
	AttackMethod string `json:"attack_method"`
	ThreatActor  string `json:"threat_actor"`
	ActorCountry string `json:"actor_country"`
	TargetSector string `json:"target_sector"`
	VictimCount  int    `json:"victim_count"`
	ZeroDay      bool   `json:"zero_day"`
}

type articleAnalysisResponse struct {
	Summary       string        `json:"summary"`
	AttackMethods attackMethods `json:"attack_method"`
	ThreatActor   string        `json:"threat_actor"`
	ActorCountry  actorCountry  `json:"actor_country"`
	TargetSector  string        `json:"target_sector"`
	VictimCount   int           `json:"victim_count"`
	ZeroDay       bool          `json:"zero_day"`
}

type actorCountry string
type attackMethods []string

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
	content, err := c.complete(ctx, generateSystemPrompt(request.Language), string(facts))
	if err != nil {
		return "", err
	}
	var result struct {
		Summary        string `json:"summary"`
		ConciseSummary string `json:"concise_summary"`
	}
	if err := json.Unmarshal([]byte(normalizeJSONContent(content)), &result); err != nil {
		return "", invalidResponse(content)
	}
	if strings.TrimSpace(result.Summary) == "" {
		result.Summary = result.ConciseSummary
	}
	if strings.TrimSpace(result.Summary) == "" {
		return "", invalidResponse(content)
	}
	return strings.TrimSpace(result.Summary), nil
}

func (c *Client) AnalyzeArticle(ctx context.Context, request ArticleRequest) (ArticleAnalysis, error) {
	input, err := json.Marshal(request)
	if err != nil {
		return ArticleAnalysis{}, fmt.Errorf("encode article: %w", err)
	}
	content, err := c.complete(ctx, AnalyzeArticleSystemPrompt(request.Language), string(input))
	if err != nil {
		return ArticleAnalysis{}, err
	}
	var response articleAnalysisResponse
	if err := json.Unmarshal([]byte(normalizeJSONContent(content)), &response); err != nil {
		return ArticleAnalysis{}, invalidResponse(content)
	}
	analysis := ArticleAnalysis{
		Summary: response.Summary, AttackMethod: response.AttackMethods.String(), ThreatActor: response.ThreatActor,
		ActorCountry: string(response.ActorCountry), TargetSector: response.TargetSector,
		VictimCount: response.VictimCount, ZeroDay: response.ZeroDay,
	}
	analysis.Summary = strings.TrimSpace(analysis.Summary)
	analysis.AttackMethod = strings.TrimSpace(analysis.AttackMethod)
	analysis.ThreatActor = strings.TrimSpace(analysis.ThreatActor)
	analysis.ActorCountry = strings.TrimSpace(analysis.ActorCountry)
	analysis.TargetSector = strings.TrimSpace(analysis.TargetSector)
	if analysis.ThreatActor == "" {
		analysis.ThreatActor = "Unknown"
		if strings.EqualFold(strings.TrimSpace(request.Language), "ko") {
			analysis.ThreatActor = "미확인"
		}
	}
	if analysis.Summary == "" || analysis.AttackMethod == "" || analysis.TargetSector == "" || analysis.VictimCount < 0 {
		return ArticleAnalysis{}, invalidResponse(content)
	}
	return analysis, nil
}

func (m *attackMethods) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*m = attackMethods{value}
		return nil
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("decode attack methods: %w", err)
	}
	*m = values
	return nil
}

func (m attackMethods) String() string {
	values := make([]string, 0, len(m))
	for _, value := range m {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return strings.Join(values, ", ")
}

func (c *actorCountry) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "0" || trimmed == "null" {
		*c = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode actor country: %w", err)
	}
	*c = actorCountry(value)
	return nil
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
		return invalidResponse(content)
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

func invalidResponse(content string) error {
	return fmt.Errorf("%w: response content %q", ErrInvalidResponse, content)
}
