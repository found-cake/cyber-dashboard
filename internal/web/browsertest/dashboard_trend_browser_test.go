//go:build browser

package browsertest

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

type trendGeometry struct {
	Buckets  int     `json:"buckets"`
	BarWidth float64 `json:"barWidth"`
	FirstBar float64 `json:"firstBar"`
	LastBar  float64 `json:"lastBar"`
	PlotEnd  float64 `json:"plotEnd"`
	Line     int     `json:"line"`
}

// Read from the rendered SVG, so the assertion is on what a viewer sees.
const readTrendGeometry = `(() => {
	const chart = document.querySelector("#collection-trend svg");
	const columns = [...chart.querySelectorAll(":scope > g:not(.trend-grid)")];
	const slots = columns.map(column => column.querySelector("rect"));
	const bars = columns.map(column => column.querySelector("rect + rect")).filter(Boolean);
	return {
		buckets: columns.length,
		barWidth: Number(bars[0].getAttribute("width")),
		firstBar: Number(slots[0].getAttribute("x")),
		lastBar: Number(slots[slots.length - 1].getAttribute("x")),
		plotEnd: Number(chart.querySelector(".trend-grid line").getAttribute("x2")),
		line: chart.querySelector("polyline").getAttribute("points").split(" ").length
	};
})()`

func TestTrendGeometryHoldsAcrossWideRangesAndOpensUpDaily(t *testing.T) {
	// Given a dashboard whose bucket count changes with the selected range.
	server, _ := newDashboardPartialRenderServer(t)
	browser := newBrowserContext(t, 20*time.Second)
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#collection-trend svg`),
	); err != nil {
		t.Fatalf("load dashboard: %v", err)
	}

	measured := map[string]trendGeometry{}
	for _, test := range []struct{ days, wantBuckets int }{{30, 10}, {7, 7}, {90, 10}} {
		// When each range is selected.
		var geometry trendGeometry
		if err := chromedp.Run(browser,
			chromedp.Evaluate(fmt.Sprintf(`(() => {
				const select = document.querySelector("#dashboard-range");
				select.value = "%d";
				select.dispatchEvent(new Event("change", { bubbles: true }));
			})()`, test.days), nil),
			chromedp.Poll(fmt.Sprintf(`document.querySelectorAll("#collection-trend svg > g:not(.trend-grid)").length === %d`, test.wantBuckets), nil),
			chromedp.Evaluate(readTrendGeometry, &geometry),
		); err != nil {
			t.Fatalf("select the %d-day range: %v", test.days, err)
		}

		// Then it draws one slot per bucket, and the line follows the same points.
		if geometry.Buckets != test.wantBuckets || geometry.Line != test.wantBuckets {
			t.Fatalf("%d-day chart = %+v, want %d buckets", test.days, geometry, test.wantBuckets)
		}
		measured[fmt.Sprint(test.days)] = geometry
	}
	var wideLabel string
	if err := chromedp.Run(browser, chromedp.Evaluate(`document.querySelector("#collection-trend [data-bucket]").getAttribute("aria-label")`, &wideLabel)); err != nil {
		t.Fatalf("inspect the 90-day trend label: %v", err)
	}
	if !strings.Contains(wideLabel, "Jun 1, 2026 – Jun 9, 2026") {
		t.Fatalf("90-day first bucket label = %q", wideLabel)
	}

	// Then the two wide ranges are drawn identically; switching moves nothing but the bars.
	if measured["30"] != measured["90"] {
		t.Fatalf("wide ranges disagree on geometry: 30-day %+v, 90-day %+v", measured["30"], measured["90"])
	}
	if measured["30"].LastBar+measured["30"].BarWidth > measured["30"].PlotEnd {
		t.Fatalf("wide range overflows its plot: %+v", measured["30"])
	}

	// And the daily range opens up instead of leaving the shared grid three slots short.
	if measured["7"].BarWidth <= measured["30"].BarWidth {
		t.Fatalf("daily bar width = %v, want wider than the wide ranges' %v", measured["7"].BarWidth, measured["30"].BarWidth)
	}
	if measured["7"].LastBar+measured["7"].BarWidth > measured["7"].PlotEnd {
		t.Fatalf("daily range overflows its plot: %+v", measured["7"])
	}
	// Every range reaches the right edge: a trailing gap wider than one bar means empty slots.
	for days, geometry := range measured {
		if gap := geometry.PlotEnd - (geometry.LastBar + geometry.BarWidth); gap > geometry.BarWidth {
			t.Fatalf("%s-day range leaves a %v gap after its last bucket: %+v", days, gap, geometry)
		}
	}
}

func TestTrendPointerAndKeyboardReportTheBucketFigures(t *testing.T) {
	// Given the dashboard's default range, whose third bucket holds a known set of figures.
	server, _ := newDashboardPartialRenderServer(t)
	browser := newBrowserContext(t, 20*time.Second)
	var hovered struct {
		Hidden  bool    `json:"hidden"`
		Text    string  `json:"text"`
		Start   float64 `json:"start"`
		Overlap bool    `json:"overlap"`
	}
	if err := chromedp.Run(browser,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(server.URL),
		chromedp.WaitVisible(`#collection-trend svg`),

		// When the pointer enters that bucket.
		chromedp.Evaluate(`(() => {
			const slot = document.querySelectorAll("#collection-trend [data-bucket]")[2];
			slot.dispatchEvent(new MouseEvent("mouseover", { bubbles: true }));
		})()`, nil),
		chromedp.Evaluate(`(() => {
			const chart = document.querySelector("#collection-trend");
			const tip = chart.querySelector(".chart-tip");
			const slot = chart.querySelectorAll("[data-bucket]")[2];
			const tipBox = tip.getBoundingClientRect(), slotBox = slot.getBoundingClientRect();
			return {
				hidden: tip.hidden,
				text: tip.textContent,
				start: parseFloat(tip.style.insetInlineStart),
				overlap: tipBox.left < slotBox.right && tipBox.right > slotBox.left
			};
		})()`, &hovered),
	); err != nil {
		t.Fatalf("hover a trend bucket: %v", err)
	}

	// Then the panel names the bucket's span and every exact figure behind its bars.
	if hovered.Hidden {
		t.Fatal("hovering a bucket left the tooltip hidden")
	}
	for _, want := range []string{"Jun 7, 2026 – Jun 9, 2026", "Total8", "Critical1", "High2", "Medium3"} {
		if !strings.Contains(hovered.Text, want) {
			t.Fatalf("tooltip text = %q, want it to contain %q", hovered.Text, want)
		}
	}
	// And it steps aside instead of covering the column being read.
	if hovered.Overlap {
		t.Fatalf("tooltip at %v covers the hovered bucket", hovered.Start)
	}

	// When the pointer leaves the chart.
	var hiddenAfterLeave bool
	if err := chromedp.Run(browser,
		chromedp.Evaluate(`(() => {
			document.querySelector("#collection-trend").dispatchEvent(
				new MouseEvent("mouseout", { bubbles: true, relatedTarget: document.body }));
			return document.querySelector("#collection-trend .chart-tip").hidden;
		})()`, &hiddenAfterLeave),
	); err != nil {
		t.Fatalf("leave the chart: %v", err)
	}

	// Then the panel goes away with it.
	if !hiddenAfterLeave {
		t.Fatal("tooltip stayed visible after the pointer left the chart")
	}

	var focused struct {
		Bucket bool   `json:"bucket"`
		Text   string `json:"text"`
		Hidden bool   `json:"hidden"`
	}
	if err := chromedp.Run(browser,
		chromedp.Evaluate(`document.querySelector('#collection-trend [data-bucket="3"]').focus()`, nil),
		chromedp.KeyEvent(kb.Tab),
		chromedp.Evaluate(`({
			bucket: document.activeElement === document.querySelector('#collection-trend [data-bucket="4"]'),
			text: document.querySelector('#collection-trend .chart-tip').textContent,
			hidden: document.querySelector('#collection-trend .chart-tip').hidden
		})`, &focused),
	); err != nil {
		t.Fatalf("focus a trend bucket: %v", err)
	}
	if !focused.Bucket || focused.Hidden || !strings.Contains(focused.Text, "Total10") {
		t.Fatalf("focused trend bucket = %+v", focused)
	}
	var hiddenAfterEscape bool
	if err := chromedp.Run(browser, chromedp.Evaluate(`(() => {
		document.activeElement.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
		return document.querySelector("#collection-trend .chart-tip").hidden;
	})()`, &hiddenAfterEscape)); err != nil {
		t.Fatalf("dismiss the trend tooltip: %v", err)
	}
	if !hiddenAfterEscape {
		t.Fatal("trend tooltip stayed visible after Escape")
	}
}
