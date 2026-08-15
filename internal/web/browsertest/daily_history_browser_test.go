//go:build browser

package browsertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/found-cake/cyber-dashboard/api"
)

const historicalDay = "2026-08-01"

func TestDailyHistory_opensStoredDayOutsideCollectionWindow_withoutAllowingRecollection(t *testing.T) {
	server := newDailyHistoryServer(t)
	browser := newBrowserContext(t, 20*time.Second)

	var calendarState struct {
		Disabled bool   `json:"disabled"`
		Day      string `json:"day"`
	}
	if err := chromedp.Run(browser,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(`Date.now = () => Date.parse("2026-08-13T00:00:00Z")`).Do(ctx)
			return err
		}),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`.calendar-day.has-data.is-expired`),
		chromedp.Evaluate(`(() => {
			const button = document.querySelector('.calendar-day.has-data.is-expired');
			return { disabled: button?.disabled ?? true, day: button?.dataset.day ?? "" };
		})()`, &calendarState),
	); err != nil {
		t.Fatalf("inspect historical calendar day: %v", err)
	}
	if calendarState.Disabled || calendarState.Day != historicalDay {
		t.Fatalf("historical stored day = %+v, want an enabled %s button", calendarState, historicalDay)
	}

	var result struct {
		Article             string `json:"article"`
		RecollectionBlocked bool   `json:"recollectionBlocked"`
		ModalCount          int    `json:"modalCount"`
	}
	if err := chromedp.Run(browser,
		chromedp.Evaluate("document.querySelector('[data-day=\""+historicalDay+"\"]').click()", nil),
		chromedp.WaitVisible(`.article-row`),
		chromedp.Evaluate(`({
			article: document.querySelector('.article-row h3').textContent,
			recollectionBlocked: document.querySelector('#recollect-day').disabled,
			modalCount: document.querySelectorAll('#modal-root > *').length
		})`, &result),
	); err != nil {
		t.Fatalf("open historical daily view: %v", err)
	}
	if result.Article != "Archived threat article" || !result.RecollectionBlocked || result.ModalCount != 0 {
		t.Fatalf("historical daily view = %+v, want article visible with collection blocked and no modal", result)
	}
}

func newDailyHistoryServer(t *testing.T) *httptest.Server {
	t.Helper()
	staticFiles := http.FileServerFS(os.DirFS("../../../static"))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/bootstrap", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Bootstrap{
			Settings:      api.SettingsResponse{Language: "en", TimezoneOffsetMinutes: 0},
			CollectedDays: []string{historicalDay},
		})
	})
	mux.HandleFunc("GET /api/dashboard", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Dashboard{Empty: true})
	})
	mux.HandleFunc("GET /api/daily/"+historicalDay, func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(t, writer, api.Daily{Day: historicalDay, Summary: "Archived daily summary", Articles: []api.Article{{
			Source: "Archive", Title: "Archived threat article", URL: "https://example.com/archive",
			AttackMethod: "Espionage", ThreatActor: "APT", Severity: "HIGH", PublishedAt: historicalDay,
		}}})
	})
	mux.Handle("/", staticFiles)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
