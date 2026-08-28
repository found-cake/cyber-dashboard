package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/found-cake/cyber-dashboard/internal/severity"
)

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
	// DamageUSD is stated incident damage in whole US dollars, or 0 when absent.
	DamageUSD       int64 `json:"damage_usd"`
	DataVolumeBytes int64 `json:"data_volume_bytes"`
	// PatchAvailable is "yes", "no", or "" when unspecified; severity distinguishes all three.
	PatchAvailable string `json:"patch_available"`
	ZeroDay        bool   `json:"zero_day"`
}

type articleAnalysisResponse struct {
	Summary        string        `json:"summary"`
	AttackMethods  attackMethods `json:"attack_method"`
	ThreatActor    string        `json:"threat_actor"`
	ActorCountry   actorCountry  `json:"actor_country"`
	TargetSector   string        `json:"target_sector"`
	VictimCount    int           `json:"victim_count"`
	DamageUSD      damageAmount  `json:"damage_usd"`
	DataVolume     dataVolume    `json:"data_volume"`
	PatchAvailable patchState    `json:"patch_available"`
	ZeroDay        bool          `json:"zero_day"`
}

var requiredArticleAnalysisFields = [...]string{
	"summary", "attack_method", "threat_actor", "actor_country", "target_sector",
	"victim_count", "damage_usd", "data_volume", "patch_available", "zero_day",
}

type actorCountry string
type attackMethods []string
type damageAmount int64
type patchState string

func (r *articleAnalysisResponse) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, field := range requiredArticleAnalysisFields {
		if _, exists := fields[field]; !exists {
			return fmt.Errorf("missing required article analysis field %q", field)
		}
	}
	type response articleAnalysisResponse
	var decoded response
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = articleAnalysisResponse(decoded)
	return nil
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
		return ArticleAnalysis{}, invalidResponse()
	}
	analysis := ArticleAnalysis{
		Summary: response.Summary, AttackMethod: response.AttackMethods.String(), ThreatActor: response.ThreatActor,
		ActorCountry: string(response.ActorCountry), TargetSector: response.TargetSector,
		VictimCount: response.VictimCount, DamageUSD: int64(response.DamageUSD),
		DataVolumeBytes: response.DataVolume.bytes, PatchAvailable: string(response.PatchAvailable), ZeroDay: response.ZeroDay,
	}
	analysis.Summary = strings.TrimSpace(analysis.Summary)
	analysis.AttackMethod = strings.TrimSpace(analysis.AttackMethod)
	analysis.ThreatActor = strings.TrimSpace(analysis.ThreatActor)
	analysis.ActorCountry = strings.TrimSpace(analysis.ActorCountry)
	analysis.TargetSector = strings.TrimSpace(analysis.TargetSector)
	if analysis.ThreatActor == "" {
		analysis.ThreatActor = unknownActor
	}
	if analysis.Summary == "" || analysis.AttackMethod == "" || analysis.TargetSector == "" ||
		analysis.VictimCount < 0 || analysis.DamageUSD < 0 || analysis.DataVolumeBytes < 0 {
		return ArticleAnalysis{}, invalidResponse()
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

// UnmarshalJSON normalizes patch_available to yes, no, or unspecified. Bare false means
// unspecified because absence of a claim is not evidence that no patch exists.
func (p *patchState) UnmarshalJSON(data []byte) error {
	var flag bool
	if err := json.Unmarshal(data, &flag); err == nil {
		*p = ""
		if flag {
			*p = severity.PatchAvailable
		}
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		*p = ""
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case severity.PatchAvailable, "true", "available", "patched", "fixed":
		*p = severity.PatchAvailable
	case severity.PatchUnavailable, "false", "unavailable", "unpatched", "none":
		*p = severity.PatchUnavailable
	default:
		*p = ""
	}
	return nil
}

// damageScales are longest-first so abbreviations cannot preempt full magnitude words.
var damageScales = []struct {
	suffix string
	factor float64
}{
	{"trillion", 1e12}, {"billion", 1e9}, {"million", 1e6}, {"thousand", 1e3},
	{"tn", 1e12}, {"bn", 1e9}, {"t", 1e12}, {"b", 1e9}, {"m", 1e6}, {"k", 1e3},
}

// UnmarshalJSON accepts numeric or prose money values; text without a figure means no stated damage.
func (d *damageAmount) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*d = 0
		return nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*d = damageAmount(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("decode damage amount: %w", err)
	}
	*d = damageAmount(parseDamageText(text))
	return nil
}

func parseDamageText(text string) float64 {
	normalized := strings.ToLower(strings.TrimSpace(text))
	for _, unwanted := range []string{"us$", "usd", "$", ",", "_", "약", "달러", "dollars", "dollar", "+", "~"} {
		normalized = strings.ReplaceAll(normalized, unwanted, "")
	}
	normalized = strings.TrimSpace(normalized)
	digits := 0
	for digits < len(normalized) && (normalized[digits] == '.' || (normalized[digits] >= '0' && normalized[digits] <= '9')) {
		digits++
	}
	number, err := strconv.ParseFloat(normalized[:digits], 64)
	if err != nil {
		return 0
	}
	remainder := strings.TrimSpace(normalized[digits:])
	for _, scale := range damageScales {
		if strings.HasPrefix(remainder, scale.suffix) {
			return number * scale.factor
		}
	}
	return number
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
