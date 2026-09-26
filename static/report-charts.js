(function (root, factory) {
  "use strict";
  const charts = factory();
  if (typeof module === "object" && module.exports) module.exports = charts;
  if (root) root.reportCharts = charts;
})(typeof window === "undefined" ? globalThis : window, function () {
  "use strict";

  const copy = {
    ko: {
      title: "이전 보고서 대비 변화", methods: "위협 유형 분포", actors: "위협 행위자 분포",
      current: "현재", previous: "이전", compared: "비교 보고서", weekly: "주간", monthly: "월간",
      currentTone: "현재", previousTone: "이전 · 기준선", difference: "이전 → 현재", sameScale: "모든 항목에 동일한 눈금 적용",
      missing: "비교 데이터 없음 · 이전 동일 유형 보고서와 해당 기간의 기사 데이터가 필요합니다.",
      unavailable: "해당 보고서 기간의 기사 데이터가 없어 그래프를 표시할 수 없습니다.",
      empty: "해당 기간의 분류 데이터가 없습니다.", counts: "기사 수", count: "건", share: "분류된 기사 비중 · 증감 단위 %p",
      increase: "▲ 증가", decrease: "▼ 감소", unchanged: "변화 없음", fresh: "신규", gone: "미관측", unclassified: "미분류",
      top: limit => `현재 상위 ${limit}개`, combinedTop: limit => `현재·이전 각각 상위 ${limit}개 · 중복 제외`
    },
    en: {
      title: "Changes from the previous report", methods: "Threat category breakdown", actors: "Threat actor breakdown",
      current: "Current", previous: "Previous", compared: "Compared with", weekly: "Weekly", monthly: "Monthly",
      currentTone: "Current", previousTone: "Previous · marker", difference: "Previous → current", sameScale: "Same scale for all entries",
      missing: "No comparison data · A previous report of the same type and articles from its period are required.",
      unavailable: "No article data is available for this report period to display charts.",
      empty: "No classified data in this period.", counts: "Article count", count: "articles", share: "Share of classified articles · change in pp",
      increase: "▲ Increase", decrease: "▼ Decrease", unchanged: "No change", fresh: "New", gone: "Not observed", unclassified: "Unclassified",
      top: limit => `Current top ${limit}`, combinedTop: limit => `Current + previous top ${limit} · unique entries`
    }
  };
  const escape = value => String(value).replace(/[&<>"']/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
  const total = rows => rows.reduce((sum, row) => sum + row.value, 0);
  const topLabels = (rows, limit) => [...rows].sort((a, b) => b.value - a.value || a.label.localeCompare(b.label)).slice(0, limit).map(row => row.label);

  function comparisonRows(current, previous, percent) {
    const limit = percent ? 7 : 5;
    const currentValues = new Map(current.map(row => [row.label, row.value]));
    const previousValues = new Map((previous || []).map(row => [row.label, row.value]));
    const currentTotal = total(current), previousTotal = total(previous || []);
    return [...new Set([...topLabels(current, limit), ...topLabels(previous || [], limit)])].map(label => {
      const count = currentValues.get(label) || 0, oldCount = previousValues.get(label) || 0;
      const value = percent ? count / (currentTotal || 1) * 100 : count;
      const oldValue = percent ? oldCount / (previousTotal || 1) * 100 : oldCount;
      return { label, count, oldCount, value, previous: previous === null ? null : oldValue,
        delta: previous === null ? null : Number((value - oldValue).toFixed(1)) };
    }).sort((a, b) => b.count - a.count || b.oldCount - a.oldCount || a.label.localeCompare(b.label));
  }

  function panel(rows, kind, language) {
    const words = copy[language] || copy.en;
    const percent = kind === "methods";
    const largest = Math.max(1, ...rows.flatMap(row => [row.value, row.previous || 0]));
    const step = percent ? 5 : Math.max(1, 10 ** Math.floor(Math.log10(largest)) / 2);
    const max = Math.ceil(largest / step) * step;
    const hasPrevious = rows.some(row => row.previous !== null);
    const number = value => percent ? `${value.toFixed(1)}%` : `${value} ${words.count}`;
    const items = rows.map((row, index) => {
      const difference = row.previous === null ? 0 : row.value - row.previous;
      const direction = difference > 0 ? "increase" : difference < 0 ? "decrease" : "unchanged";
      const sign = row.delta > 0 ? "+" : row.delta < 0 ? "−" : "";
      const magnitude = difference !== 0 && row.delta === 0 ? "&lt;0.1" : `${sign}${Math.abs(row.delta).toFixed(percent ? 1 : 0)}`;
      const delta = row.delta === null ? "" : `${words[direction]} ${magnitude} ${percent ? (language === "ko" ? "%p" : "pp") : words.count}`;
      const status = row.previous === null ? "" : row.oldCount === 0 && row.count > 0 ? words.fresh : row.count === 0 && row.oldCount > 0 ? words.gone : "";
      return `<li class="report-chart-row" style="--report-chart-current:var(--chart-${index % 5 + 1})"><div class="report-chart-label"><strong>${escape(row.label.trim() || words.unclassified)}</strong>${status ? `<span>${status}</span>` : ""}</div>
        <div class="report-chart-values"><span>${words.current} <b>${number(row.value)}</b>${percent ? ` · ${row.count} ${words.count}` : ""}</span>${row.previous === null ? "" : `<span>${words.previous} ${number(row.previous)}${percent ? ` · ${row.oldCount} ${words.count}` : ""}</span>`}</div>
        <div class="report-chart-bars" aria-hidden="true"><span class="report-chart-track${row.previous === null ? "" : " has-previous"}">${row.previous === null ? "" : `<i class="report-chart-previous" style="width:${row.previous / max * 100}%"></i>`}<i class="report-chart-current" style="width:${row.value / max * 100}%"></i>${row.previous === null ? "" : `<i class="report-chart-change is-${direction}" style="left:${Math.min(row.value, row.previous) / max * 100}%;width:${Math.abs(difference) / max * 100}%"></i><i class="report-chart-reference" style="left:${row.previous / max * 100}%"></i>`}</span></div>
        ${delta ? `<p class="report-chart-delta">${delta}</p>` : ""}</li>`;
    }).join("");
    return `<section class="report-chart-panel"><h3>${words[kind]}</h3><p class="report-chart-note">${(hasPrevious ? words.combinedTop : words.top)(percent ? 7 : 5)}<br>${percent ? words.share : words.counts}</p>
      <div class="report-chart-legend" aria-hidden="true"><span><i></i>${words.currentTone}</span>${hasPrevious ? `<span class="is-previous"><i></i>${words.previousTone}</span><span>${words.difference}</span>` : ""}</div>
      ${items ? `<div class="report-chart-axis"><span>0</span><span>${words.sameScale}</span><span>${percent ? `${max}%` : `${max} ${words.count}`}</span></div>` : ""}
      ${items ? `<ul class="report-chart-list">${items}</ul>` : `<p class="report-chart-note">${words.empty}</p>`}</section>`;
  }

  function render(report, language) {
    if (report.type !== "weekly" && report.type !== "monthly") return "";
    const words = copy[language] || copy.en;
    if (!report.charts) return `<section class="report-charts"><h2>${words.title}</h2><p class="report-chart-note">${words.unavailable}</p></section>`;
    const charts = report.charts, previous = charts.previous || null;
    const baseline = previous ? `${words.compared} · ${words[report.type]} · ${escape(previous.period_start)} – ${escape(previous.period_end)}` : words.missing;
    return `<section class="report-charts"><h2>${words.title}</h2><p class="report-chart-baseline">${baseline}</p><div class="report-chart-grid">
      ${panel(comparisonRows(charts.attack_methods, previous ? previous.attack_methods : null, true), "methods", language)}
      ${panel(comparisonRows(charts.threat_actors, previous ? previous.threat_actors : null, false), "actors", language)}
      </div></section>`;
  }

  const styles = `
    .report-charts { --report-chart-marker: var(--t1); margin-block: var(--space-6); color: var(--t1); font-size: 12px; }
    [data-theme="dark"] .report-charts { --report-chart-marker: var(--bg); }
    .report-chart-row, .report-chart-legend { --report-chart-previous: color-mix(in srgb, var(--report-chart-current) 35%, var(--on-accent)); }
    .report-charts h2 { margin: 0 0 var(--space-2); font-size: 14px; word-break: keep-all; }
    .report-chart-baseline, .report-chart-note { margin: 0 0 var(--space-4); color: var(--t2); font-size: 11px; line-height: 1.6; word-break: keep-all; overflow-wrap: anywhere; }
    .report-chart-grid { display: grid; grid-template-columns: repeat(auto-fit,minmax(min(100%,20rem),1fr)); gap: var(--space-4); }
    .report-chart-panel { min-width: 0; padding: var(--space-4); border: 1px solid var(--cb); border-radius: var(--radius-card); }
    .report-chart-panel h3 { margin: 0 0 var(--space-1); color: var(--t1); font-size: 14px; letter-spacing: normal; text-transform: none; word-break: keep-all; }
    .report-chart-list { display: grid; gap: var(--space-4); margin: 0; padding: 0; list-style: none; }
    .report-chart-row { min-width: 0; break-inside: avoid; page-break-inside: avoid; }
    .report-chart-label, .report-chart-values { display: flex; justify-content: space-between; flex-wrap: wrap; gap: var(--space-1) var(--space-2); line-height: 1.6; }
    .report-chart-label strong { min-width: 0; word-break: keep-all; overflow-wrap: anywhere; font-weight: 600; }
    .report-chart-label span { color: var(--t2); font-size: 11px; }
    .report-chart-values { margin-block: var(--space-1); color: var(--t2); font-size: 11px; font-variant-numeric: tabular-nums; }
    .report-chart-values b { color: var(--t1); }
    .report-chart-track { position: relative; display: block; height: var(--space-3); background: var(--divider); }
    .report-chart-track i { position: absolute; inset-inline-start: 0; inset-block-start: 0; height: 100%; border-radius: inherit; }
    .report-chart-current { background: var(--report-chart-current); }
    .report-chart-previous { background: var(--report-chart-previous); }
    .report-chart-track.has-previous .report-chart-current { inset-block-start: 25%; height: 50%; }
    .report-chart-change { overflow: hidden; color: var(--report-chart-marker); }
    .report-chart-change::before { content: ""; position: absolute; top: calc(50% - .5px); left: 0; width: 100%; border-top: 1px solid currentColor; }
    .report-chart-change::after { content: ""; position: absolute; top: 50%; right: 0; transform: translateY(-50%); border-block: 4px solid transparent; border-left: 4px solid currentColor; }
    .report-chart-change.is-decrease { transform: scaleX(-1); }
    .report-chart-change.is-unchanged::before, .report-chart-change.is-unchanged::after { content: none; }
    .report-chart-track .report-chart-reference { width: 2px; transform: translateX(-50%); background: var(--report-chart-marker); }
    .report-chart-axis { display: flex; justify-content: space-between; gap: var(--space-2); margin-bottom: var(--space-2); color: var(--t2); font-size: 10px; font-variant-numeric: tabular-nums; }
    .report-chart-delta { margin: var(--space-1) 0 0; color: var(--t1); font-size: 11px; font-weight: 600; font-variant-numeric: tabular-nums; }
    .report-chart-legend { --report-chart-current: var(--t2); display: flex; flex-wrap: wrap; gap: var(--space-2) var(--space-4); margin-bottom: var(--space-2); color: var(--t2); font-size: 11px; }
    .report-chart-legend span { display: inline-flex; align-items: center; gap: var(--space-1); }
    .report-chart-legend i { width: var(--space-4); height: var(--space-2); background: var(--report-chart-current); }
    .report-chart-legend .is-previous i { background: var(--report-chart-previous); border-inline-end: 2px solid var(--report-chart-marker); }
    @media print { .report-chart-list { display: block; } .report-chart-panel { --space-1: calc(var(--space-2) / 4); --space-3: var(--space-2); --space-4: var(--space-2); break-inside: avoid; page-break-inside: avoid; } .report-chart-label, .report-chart-values, .report-chart-delta { line-height: 1.3; } .report-chart-values { margin-block: 0; } .report-chart-row { display: grid; grid-template-columns: minmax(0,1fr) auto; column-gap: var(--space-2); margin-bottom: calc(var(--space-2) / 2); } .report-chart-values, .report-chart-bars { grid-column: 1 / -1; } .report-chart-delta { grid-column: 2; grid-row: 1; margin: 0; text-align: end; } .report-charts { print-color-adjust: exact; -webkit-print-color-adjust: exact; } .report-charts h2, .report-chart-baseline, .report-chart-panel h3, .report-chart-note, .report-chart-legend { break-after: avoid; } }
  `;
  return { render, comparisonRows, styles };
});
