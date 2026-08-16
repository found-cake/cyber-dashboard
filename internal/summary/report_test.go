package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/found-cake/cyber-dashboard/api"
)

type reportCall struct {
	Facts       []string                `json:"facts"`
	Sections    []string                `json:"sections"`
	Threats     []ReportThreatCandidate `json:"threats"`
	ThreatLimit int                     `json:"threat_limit"`
}

func recordReportCalls(t *testing.T, replies []string) (*Service, *[]reportCall) {
	t.Helper()
	calls := []reportCall{}
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode chat request: %v", err)
		}
		var input reportCall
		if len(body.Messages) < 2 {
			t.Errorf("messages = %d, want at least 2", len(body.Messages))
		} else if err := json.Unmarshal([]byte(body.Messages[1].Content), &input); err != nil {
			t.Errorf("decode report input: %v", err)
		}
		calls = append(calls, input)
		reply := replies[min(len(calls)-1, len(replies)-1)]
		encoded, _ := json.Marshal(reply)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test-model","choices":[{"index":0,"message":{"role":"assistant","content":%s},"finish_reason":"stop"}]}`, encoded)
	}))
	t.Cleanup(upstream.Close)
	service := NewService(staticSettings{value: api.Settings{
		LLMBaseURL: upstream.URL + "/v1", LLMModel: "test-model", LLMAPIKey: "key", LLMTimeout: 5,
	}})
	return service, &calls
}

func reportThreats(count int) []ReportThreatCandidate {
	values := make([]ReportThreatCandidate, count)
	for index := range values {
		values[index] = ReportThreatCandidate{
			ID: fmt.Sprintf("threat-%d", index+1), Title: fmt.Sprintf("Incident %d", index+1),
			Summary: "Distinct incident", Severity: "CRITICAL", PublishedAt: "2026-08-15", SourceCount: 1,
		}
	}
	return values
}

func TestServiceGenerateReportSelectsThreatsInExistingMergePass_whenFactsNeedThreeBatches(t *testing.T) {
	// Given report facts that already require three batch calls and one merge call.
	service, calls := recordReportCalls(t, []string{
		`{"summary":"part-1"}`,
		`{"summary":"part-2"}`,
		`{"summary":"part-3"}`,
		`{"summary":"merged","top_threat_groups":[{"representative_id":"threat-1","member_ids":["threat-1","threat-2"],"translated_title":"Merged incident"},{"representative_id":"threat-3","member_ids":["threat-3"],"translated_title":"Separate incident"}]}`,
	})

	// When the report summary and top threats are generated together.
	got, err := service.GenerateReport(context.Background(), ReportRequest{
		Language: "en", Kind: "weekly report", Facts: articleFacts(14), Threats: reportThreats(6), ThreatLimit: 3,
	})

	// Then the pre-existing merge pass carries all candidates and returns the bounded groups.
	if err != nil {
		t.Fatalf("generate report: %v", err)
	}
	if got.Summary != "merged" || len(got.ThreatGroups) != 2 {
		t.Fatalf("report result = %+v", got)
	}
	if len(*calls) != 4 || len((*calls)[3].Sections) != 3 || len((*calls)[3].Threats) != 6 || (*calls)[3].ThreatLimit != 3 {
		t.Fatalf("report calls = %+v", *calls)
	}
}

func TestServiceGenerateReportMergesSections_whenDailySummariesNeedTwoBatches(t *testing.T) {
	// Given daily summaries that require two batch calls before a report merge.
	service, calls := recordReportCalls(t, []string{
		`{"summary":"part-1"}`,
		`{"summary":"part-2"}`,
		`{"summary":"merged","top_threat_groups":[{"representative_id":"threat-2","member_ids":["threat-2"],"translated_title":"번역된 주요 위협"}]}`,
	})

	// When the report is generated.
	got, err := service.GenerateReport(context.Background(), ReportRequest{
		Language: "ko", Kind: "weekly report", Facts: articleFacts(8), Threats: reportThreats(6), ThreatLimit: 3,
	})

	// Then the merge produces one coherent report and performs threat selection once.
	if err != nil {
		t.Fatalf("generate report: %v", err)
	}
	if got.Summary != "merged" || len(got.ThreatGroups) != 1 || got.ThreatGroups[0].TranslatedTitle != "번역된 주요 위협" || len(*calls) != 3 {
		t.Fatalf("result = %+v, calls = %+v", got, *calls)
	}
	if len((*calls)[0].Threats) != 0 || len((*calls)[1].Threats) != 0 || len((*calls)[2].Threats) != 6 {
		t.Fatalf("candidate calls = %+v", *calls)
	}
}

func TestServiceGenerateReportRejectsPartialReport_whenSectionMergeFails(t *testing.T) {
	// Given two valid batch summaries followed by invalid merge responses.
	service, calls := recordReportCalls(t, []string{
		`{"summary":"part-1"}`,
		`{"summary":"part-2"}`,
		`not-json`,
		`still-not-json`,
	})

	// When the period-level merge cannot produce a valid report.
	got, err := service.GenerateReport(context.Background(), ReportRequest{
		Language: "en", Kind: "weekly report", Facts: articleFacts(8), Threats: reportThreats(4), ThreatLimit: 3,
	})

	// Then no concatenated partial report is returned or stored by a caller.
	if err == nil || got.Summary != "" {
		t.Fatalf("report result = %+v, error = %v", got, err)
	}
	if len(*calls) != 4 {
		t.Fatalf("report calls = %d, want two batches and two merge attempts", len(*calls))
	}
}

func TestServiceGenerateReportDiscardsThreatSelection_whenModelReturnsInvalidIDs(t *testing.T) {
	tests := []struct {
		name   string
		reply  string
		groups int
	}{
		{
			name:  "unknown id",
			reply: `{"summary":"usable","top_threat_groups":[{"representative_id":"missing","member_ids":["missing"],"translated_title":"Unknown incident"}]}`,
		},
		{
			name:  "candidate used twice",
			reply: `{"summary":"usable","top_threat_groups":[{"representative_id":"threat-1","member_ids":["threat-1"],"translated_title":"First incident"},{"representative_id":"threat-2","member_ids":["threat-1","threat-2"],"translated_title":"Second incident"}]}`,
		},
		{
			name:  "missing translated title",
			reply: `{"summary":"usable","top_threat_groups":[{"representative_id":"threat-1","member_ids":["threat-1"]}]}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given an otherwise usable report response with an invalid candidate selection.
			service, _ := recordReportCalls(t, []string{test.reply})

			// When the response crosses the LLM boundary.
			got, err := service.GenerateReport(context.Background(), ReportRequest{
				Language: "en", Kind: "weekly report", Facts: []string{"fact"}, Threats: reportThreats(3), ThreatLimit: 3,
			})

			// Then the prose remains usable and callers can apply the deterministic static fallback.
			if err != nil || got.Summary != "usable" || len(got.ThreatGroups) != 0 {
				t.Fatalf("report result = %+v, error = %v", got, err)
			}
		})
	}
}
