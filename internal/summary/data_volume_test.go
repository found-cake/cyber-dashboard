package summary

import (
	"context"
	"testing"
)

func TestClientAnalyzeArticleNormalizesDataVolumeForSeverity(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		{name: "decimal gigabytes", value: `"500 GB"`, want: 500_000_000_000},
		{name: "range uses conservative lower bound", value: `"2 - 3 TB"`, want: 2_000_000_000_000},
		{name: "Unicode range uses conservative lower bound", value: `"2\u00a0–\u00a03 TB"`, want: 2_000_000_000_000},
		{name: "binary unit", value: `"1.5 GiB"`, want: 1_610_612_736},
		{name: "binary bit unit", value: `"1 Gib"`, want: 134_217_728},
		{name: "gigabits are converted to bytes", value: `"1 Gb"`, want: 125_000_000},
		{name: "megabits are converted to bytes", value: `"1 Mb"`, want: 125_000},
		{name: "terabits are converted to bytes", value: `"1 Tb"`, want: 125_000_000_000},
		{name: "uppercase terabytes remain bytes", value: `"1 TB"`, want: 1_000_000_000_000},
		{name: "English long unit", value: `"3 gigabytes"`, want: 3_000_000_000},
		{name: "Korean unit", value: `"1.7 테라바이트"`, want: 1_700_000_000_000},
		{name: "grouped thousands", value: `"1,700 GB"`, want: 1_700_000_000_000},
		{name: "surrounding whitespace", value: `" 500 MB "`, want: 500_000_000},
		{name: "empty string means not stated", value: `""`, want: 0},
		{name: "numeric JSON has no unit", value: `1700000000000`, wantErr: true},
		{name: "fractional numeric JSON has no unit", value: `1.5`, wantErr: true},
		{name: "negative numeric JSON", value: `-1`, wantErr: true},
		{name: "unitless string", value: `"500"`, wantErr: true},
		{name: "leading approximation", value: `"about 1 TB"`, wantErr: true},
		{name: "multiple prose quantities", value: `"250 million records 1 TB"`, wantErr: true},
		{name: "multiple byte quantities", value: `"500 GB leaked 2 MB"`, wantErr: true},
		{name: "malformed thousands separator", value: `"1,5 TB"`, wantErr: true},
		{name: "incomplete range", value: `"2- TB"`, wantErr: true},
		{name: "reversed range", value: `"2-1 TB"`, wantErr: true},
		{name: "fractional reversed range", value: `"1.9-1.1 B"`, wantErr: true},
		{name: "overflowing range upper bound", value: `"2-999999999999999999999 TB"`, wantErr: true},
		{name: "fractional overflow", value: `"9223372036854775807.1 B"`, wantErr: true},
		{name: "trailing approximation", value: `"1 TB+"`, wantErr: true},
		{name: "record count is not byte volume", value: `"250 million records"`, wantErr: true},
		{name: "overflow", value: `"999999999999999999999 PB"`, wantErr: true},
		{name: "null", value: `null`, wantErr: true},
		{name: "boolean", value: `true`, wantErr: true},
		{name: "object", value: `{}`, wantErr: true},
		{name: "array", value: `[]`, wantErr: true},
		{name: "not stated", value: `"unknown"`, wantErr: true},
		{name: "lowercase SI prefix with byte suffix", value: `"1 mB"`, wantErr: true},
		{name: "lowercase peta prefix with byte suffix", value: `"1 pB"`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given an otherwise valid analysis with one model-style data-volume value.
			client := newArticleAnalysisClient(t, `{
				"summary":"Customer data was exfiltrated.",
				"attack_method":"Data Breach / Unauthorized Access",
				"threat_actor":"Unknown",
				"actor_country":"",
				"target_sector":"Technology",
				"victim_count":0,
				"damage_usd":0,
				"data_volume":`+test.value+`,
				"patch_available":"unknown",
				"zero_day":false
			}`)

			// When the response is parsed for severity evaluation.
			analysis, err := client.AnalyzeArticle(context.Background(), ArticleRequest{Language: "en", Title: "Data breach", Body: "Body"})

			// Then malformed model output is rejected, while a valid quantity becomes bytes.
			if test.wantErr {
				if err == nil {
					t.Fatal("expected invalid response error")
				}
				return
			}
			if err != nil {
				t.Fatalf("analyze article: %v", err)
			}
			if analysis.DataVolumeBytes != test.want {
				t.Fatalf("data volume = %d, want %d", analysis.DataVolumeBytes, test.want)
			}
		})
	}
}

func TestClientAnalyzeArticleRejectsMissingDataVolume(t *testing.T) {
	// Given a model response that omits the required data_volume field.
	client := newArticleAnalysisClient(t, `{
		"summary":"Customer data was exfiltrated.",
		"attack_method":"Data Breach / Unauthorized Access",
		"threat_actor":"Unknown",
		"actor_country":"",
		"target_sector":"Technology",
		"victim_count":0,
		"damage_usd":0,
		"patch_available":"unknown",
		"zero_day":false
	}`)

	// When the response is parsed for severity evaluation.
	_, err := client.AnalyzeArticle(context.Background(), ArticleRequest{Language: "en", Title: "Data breach", Body: "Body"})

	// Then a missing required field is rejected instead of silently becoming zero.
	if err == nil {
		t.Fatal("expected invalid response error")
	}
}
