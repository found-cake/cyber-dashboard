package summary

import (
	"context"
	"testing"
)

func TestClientAnalyzeArticleNormalizesDataVolumeForSeverity(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int64
	}{
		{name: "decimal gigabytes", value: `"500 GB"`, want: 500_000_000_000},
		{name: "range uses conservative lower bound", value: `"2 - 3 TB"`, want: 2_000_000_000_000},
		{name: "binary unit", value: `"1.5 GiB"`, want: 1_610_612_736},
		{name: "English long unit", value: `"3 gigabytes"`, want: 3_000_000_000},
		{name: "Korean unit", value: `"1.7 테라바이트"`, want: 1_700_000_000_000},
		{name: "grouped thousands", value: `"1,700 GB"`, want: 1_700_000_000_000},
		{name: "surrounding whitespace", value: `" 500 MB "`, want: 500_000_000},
		{name: "numeric JSON has no unit", value: `1700000000000`, want: 0},
		{name: "fractional numeric JSON has no unit", value: `1.5`, want: 0},
		{name: "negative numeric JSON", value: `-1`, want: 0},
		{name: "unitless string", value: `"500"`, want: 0},
		{name: "leading approximation", value: `"about 1 TB"`, want: 0},
		{name: "multiple prose quantities", value: `"250 million records 1 TB"`, want: 0},
		{name: "multiple byte quantities", value: `"500 GB leaked 2 MB"`, want: 0},
		{name: "malformed thousands separator", value: `"1,5 TB"`, want: 0},
		{name: "incomplete range", value: `"2- TB"`, want: 0},
		{name: "reversed range", value: `"2-1 TB"`, want: 0},
		{name: "fractional reversed range", value: `"1.9-1.1 B"`, want: 0},
		{name: "overflowing range upper bound", value: `"2-999999999999999999999 TB"`, want: 0},
		{name: "fractional overflow", value: `"9223372036854775807.1 B"`, want: 0},
		{name: "trailing approximation", value: `"1 TB+"`, want: 0},
		{name: "record count is not byte volume", value: `"250 million records"`, want: 0},
		{name: "overflow", value: `"999999999999999999999 PB"`, want: 0},
		{name: "null", value: `null`, want: 0},
		{name: "boolean", value: `true`, want: 0},
		{name: "object", value: `{}`, want: 0},
		{name: "array", value: `[]`, want: 0},
		{name: "not stated", value: `"unknown"`, want: 0},
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

			// Then only a complete byte-sized quantity becomes a deterministic byte count.
			if err != nil {
				t.Fatalf("analyze article: %v", err)
			}
			if analysis.DataVolumeBytes != test.want {
				t.Fatalf("data volume = %d, want %d", analysis.DataVolumeBytes, test.want)
			}
		})
	}
}
