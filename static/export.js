(() => {
  "use strict";

  const EXPORT_STYLES = `
    @page { size: A4; margin: 14mm; }
    :root { --ink: #1f2328; --muted: #57606a; --rule: #d0d7de; --accent: #0969da; --accent-bg: #ddf4ff; --critical: #cf222e; --critical-bg: #fff1f0; --high: #9a6700; --high-bg: #fff8c5; --surface: #f6f8fa; }
    *, *::before, *::after { box-sizing: border-box; }
    html { background: #eef0f7; }
    body { margin: 0; color: var(--ink); background: #eef0f7; font: 11pt/1.55 "NanumSquareRound", "Apple SD Gothic Neo", "Noto Sans KR", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    .paper { width: 182mm; min-height: 269mm; margin: 14mm auto; padding: 0; background: #fff; user-select: text; -webkit-user-select: text; }
    .document-header { padding-block-end: 10mm; border-block-end: 1px solid var(--rule); }
    .eyebrow { color: var(--accent); font-size: 8pt; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
    h1 { margin: 3mm 0 1mm; font-size: 22pt; line-height: 1.2; }
    h2 { margin: 0 0 3mm; font-size: 12pt; line-height: 1.25; }
    h3 { margin: 0 0 2mm; color: var(--muted); font-size: 8pt; letter-spacing: .08em; text-transform: uppercase; }
    p { margin: 0; }
    .subtitle { color: var(--muted); font-size: 10pt; }
    .generated { margin-block-start: 5mm; color: var(--muted); font-size: 8pt; }
    .metric-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 3mm; margin-block: 8mm; break-inside: avoid; }
    .metric-grid-daily { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .metric { padding: 4mm 3mm; border-block-start: 1.5px solid var(--metric-color, var(--rule)); background: var(--metric-bg, var(--surface)); text-align: center; break-inside: avoid; }
    .metric-total { --metric-color: var(--accent); --metric-bg: var(--accent-bg); }
    .metric-critical { --metric-color: var(--critical); --metric-bg: var(--critical-bg); }
    .metric-high { --metric-color: var(--high); --metric-bg: var(--high-bg); }
    .metric-medium { --metric-color: var(--muted); }
    .metric strong { display: block; color: var(--metric-color, var(--ink)); font-size: 17pt; line-height: 1; }
    .metric span { display: block; margin-block-start: 2mm; color: var(--metric-color, var(--muted)); font-size: 8pt; }
    .report-cover { break-inside: auto; page-break-inside: auto; }
    .report-summary > h3 { margin-block-end: 4mm; }
    .summary-overview { color: var(--ink); font-size: 11.5pt; line-height: 1.65; word-break: keep-all; overflow-wrap: anywhere; }
    .summary-group { margin-block-start: 6mm; break-inside: auto; }
    .summary-topic { margin: 0 0 2mm; color: var(--ink); font-size: 13pt; font-weight: 800; line-height: 1.3; letter-spacing: normal; text-transform: none; word-break: keep-all; overflow-wrap: anywhere; break-after: avoid; page-break-after: avoid; }
    .summary-item { display: block; margin: 0; color: var(--ink); font-size: 10.5pt; line-height: 1.55; word-break: keep-all; overflow-wrap: anywhere; break-inside: avoid; page-break-inside: avoid; orphans: 3; widows: 3; }
    .section { padding-block: 5mm; border-block-end: 1px solid var(--rule); break-inside: avoid; }
    .section.report-summary { break-before: avoid; break-inside: auto; page-break-inside: auto; }
    .section.daily-summary-section { break-before: avoid; break-inside: auto; page-break-inside: auto; }
    .daily-summary-section .section-body { word-break: keep-all; }
    .report-details { break-before: page; page-break-before: always; }
    .section:last-child { border-block-end: 0; }
    .section-body { color: var(--muted); white-space: pre-line; overflow-wrap: anywhere; }
    .chips { display: flex; flex-wrap: wrap; gap: 2mm; }
    .chip { display: inline-block; padding: 1.5mm 2.5mm; background: var(--surface); color: var(--muted); font-size: 8pt; }
    @media screen { body { padding-block: 14mm; } }
    @media screen and (max-width: 700px) { body { padding: 16px; } .paper { width: 100%; min-height: auto; } .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    @media print { html, body { background: #fff; } .paper { width: auto; min-height: auto; margin: 0; } }
  `;

  const text = value => String(value == null ? "" : value)
    .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;").replace(/'/g, "&#39;");

  const array = value => Array.isArray(value) ? value : [];

  function documentHTML(title, subtitle, copy, body) {
    return `<!doctype html><html lang="${text(copy.language)}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${text(title)} · Cyber Dashboard</title><style>${EXPORT_STYLES}</style></head><body><main class="paper"><header class="document-header"><div class="eyebrow">Cyber Dashboard</div><h1>${text(title)}</h1><p class="subtitle">${text(subtitle)}</p><p class="generated">${text(copy.generatedBy)}</p></header>${body}</main></body></html>`;
  }

  function metric(value, label, tone = "") {
    return `<div class="metric${tone ? ` metric-${tone}` : ""}"><strong>${text(value)}</strong><span>${text(label)}</span></div>`;
  }

  function summaryBlocks(value) {
    const overview = [];
    const groups = [];
    let group = null;
    let itemIndex = -1;
    for (const rawLine of String(value == null ? "" : value).split(/\r?\n/)) {
      const line = rawLine.trim();
      if (!line) continue;
      if (line.startsWith("■")) {
        group = { title: line.replace(/^■\s*/, "").trim(), items: [] };
        groups.push(group);
        itemIndex = -1;
        continue;
      }
      if (/^-\s+/.test(line)) {
        const item = line.replace(/^-\s+/, "").trim();
        if (group) {
          group.items.push(item);
          itemIndex = group.items.length - 1;
        } else {
          overview.push(item);
        }
        continue;
      }
      if (group && itemIndex >= 0) group.items[itemIndex] = `${group.items[itemIndex]} ${line}`.trim();
      else overview.push(line);
    }
    return { overview: overview.join(" "), groups };
  }

  function reportSummarySections(summary, copy) {
    const blocks = summaryBlocks(summary);
    const overview = blocks.overview ? `<p class="summary-overview">${text(blocks.overview)}</p>` : "";
    const groups = blocks.groups.map(group => `<section class="summary-group"><h3 class="summary-topic">${text(group.title)}</h3>${group.items.map(item => `<p class="summary-item">- ${text(item)}</p>`).join("")}</section>`).join("");
    const fallback = overview || groups ? "" : `<p class="summary-overview">${text(copy.none)}</p>`;
    return {
      cover: `<section class="section report-summary"><h3>${text(copy.labels.summary)}</h3>${overview}${fallback}</section>`,
      details: groups ? `<div class="report-summary-details">${groups}</div>` : ""
    };
  }

  function reportDocument(report, copy) {
    const type = copy.reportTypes[report.type] || String(report.type || copy.reportWord);
    const noneValues = new Set(["none", String(copy.none || "").trim().toLowerCase()]);
    const actors = array(report.actors).map(actor => String(actor).trim()).filter(actor => actor && !noneValues.has(actor.toLowerCase()));
    const actorSection = `<section class="section"><h3>${text(copy.labels.actors)}</h3><p class="section-body">${text(actors.length ? actors.join(" · ") : copy.unknownActor)}</p></section>`;
    const sectors = array(report.sectors).map(sector => `<span class="chip">${text(sector)}</span>`).join("") || `<span class="section-body">${text(copy.none)}</span>`;
    const summary = reportSummarySections(report.summary, copy);
    const body = `<section class="report-cover"><section class="metric-grid">${metric(report.total, copy.labels.total, "total")}${metric(report.critical, copy.labels.critical, "critical")}${metric(report.high, copy.labels.high, "high")}${metric(report.medium, copy.labels.medium, "medium")}</section><section class="section"><h3>${text(copy.labels.topThreat)}</h3><p class="section-body">${text(report.top_threat || copy.none)}</p></section>${actorSection}${summary.cover}</section><section class="report-details">${summary.details}<section class="section"><h3>${text(copy.labels.sectors)}</h3><div class="chips">${sectors}</div></section></section>`;
    return documentHTML(`${type} ${copy.reportWord}`, `${report.period_start} – ${report.period_end}`, copy, body);
  }

  function dailyDocument(daily, copy) {
    const articles = array(daily.articles);
    const articleLabel = articles.length === 1 ? copy.labels.article : copy.labels.articleCount;
    const body = `<section class="metric-grid metric-grid-daily">${metric(articles.length, articleLabel)}<div class="metric"><strong>${text(copy.day)}</strong><span>${text(copy.labels.date)}</span></div></section><section class="section daily-summary-section"><h3>${text(copy.dailySummary)}</h3><p class="section-body">${text(daily.summary || copy.noSummary)}</p></section>`;
    return documentHTML(copy.dailySummary, copy.day, copy, body);
  }

  function print(markup) {
    const printWindow = window.open("", "_blank");
    if (!printWindow) return false;
    printWindow.document.open();
    printWindow.document.write(markup);
    printWindow.document.close();
    printWindow.addEventListener("afterprint", () => printWindow.close(), { once: true });
    window.setTimeout(() => {
      printWindow.focus();
      printWindow.print();
    }, 0);
    return true;
  }

  window.cyberDashboardExport = {
    downloadReportPDF(report, copy) {
      return print(reportDocument(report, copy));
    },
    downloadDailyPDF(daily, copy) {
      return print(dailyDocument(daily, copy));
    }
  };
})();
