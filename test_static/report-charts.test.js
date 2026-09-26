"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { comparisonRows, render, styles } = require("../static/report-charts.js");

test("distribution compares full period proportions and retains disappeared categories", () => {
  const current = [{ label: "Phishing", value: 3 }, { label: "Ransomware", value: 1 }];
  const previous = [{ label: "Phishing", value: 1 }, { label: "Malware", value: 3 }];
  const rows = comparisonRows(current, previous, true);
  assert.deepEqual(rows.map(row => [row.label, row.value, row.previous, row.delta]), [
    ["Phishing", 75, 25, 50], ["Ransomware", 25, 0, 25], ["Malware", 0, 75, -75]
  ]);
});

test("actor differences are counts even when period totals differ", () => {
  const rows = comparisonRows([{ label: "APT28", value: 10 }], [{ label: "APT28", value: 2 }], false);
  assert.equal(rows[0].delta, 8);
  assert.equal(rows[0].previous, 2);
});

test("comparison explicitly identifies increase, decrease, and unchanged without hatching", () => {
  const report = { type: "weekly", charts: {
    attack_methods: [], threat_actors: [{ label: "Up", value: 60 }, { label: "Down", value: 50 }, { label: "Same", value: 40 }],
    previous: { period_start: "2026-07-01", period_end: "2026-07-07", attack_methods: [],
      threat_actors: [{ label: "Up", value: 20 }, { label: "Down", value: 70 }, { label: "Same", value: 40 }] }
  } };
  const html = render(report, "ko");
  assert.ok(html.includes("▲ 증가 +40 건"));
  assert.ok(html.includes("▼ 감소 −20 건"));
  assert.ok(html.includes("변화 없음 0 건"));
  assert.ok(html.includes('report-chart-change is-increase'));
  assert.ok(html.includes('report-chart-change is-decrease'));
  assert.ok(!styles.includes("repeating-linear-gradient"));
  const english = render(report, "en");
  assert.ok(english.includes("▲ Increase +40 articles"));
  assert.ok(english.includes("▼ Decrease −20 articles"));
  assert.ok(english.includes("No change 0 articles"));
});

test("subprecision share changes retain direction instead of claiming no change", () => {
  const html = render({ type: "weekly", charts: {
    attack_methods: [{ label: "A", value: 1000 }, { label: "B", value: 999000 }], threat_actors: [],
    previous: { period_start: "2026-07-01", period_end: "2026-07-07", threat_actors: [],
      attack_methods: [{ label: "A", value: 999 }, { label: "B", value: 999001 }] }
  } }, "ko");
  assert.ok(html.includes("▲ 증가 &lt;0.1 %p"));
  assert.ok(html.includes("▼ 감소 &lt;0.1 %p"));
  assert.ok(!html.includes("변화 없음"));
});

test("missing baseline differs from an available empty baseline", () => {
  const current = [{ label: "Unknown", value: 1 }];
  assert.equal(comparisonRows(current, null, false)[0].delta, null);
  assert.equal(comparisonRows(current, [], false)[0].delta, 1);
  assert.deepEqual(comparisonRows([], [], true), []);
});

test("distribution handles different denominators and rounds percentage points once", () => {
  const rows = comparisonRows([{ label: "A", value: 1 }, { label: "B", value: 2 }], [{ label: "A", value: 1 }], true);
  assert.equal(rows.find(row => row.label === "A").delta, -66.7);
});

test("charts appear only in weekly and monthly reports and escape stored labels", () => {
  const report = { charts: { attack_methods: [{ label: '<img src=x onerror="alert(1)">', value: 1 }], threat_actors: [] } };
  assert.equal(render({ ...report, type: "daily" }, "ko"), "");
  for (const type of ["weekly", "monthly"]) {
    const html = render({ ...report, type }, "ko");
    assert.ok(html.includes("&lt;img"));
    assert.ok(!html.includes("<img"));
    assert.ok(!html.includes("report-chart-delta"));
    assert.ok(!html.includes('class="report-chart-reference"'));
    assert.ok(!html.includes('class="report-chart-change '));
  }
});

test("unavailable period data is explained without fabricated bars", () => {
  const html = render({ type: "weekly" }, "en");
  assert.ok(html.includes("report-charts"));
  assert.ok(!html.includes("report-chart-bars"));
});

test("selects each period's top seven categories and retains full counts and denominators", () => {
  const current = [
    { label: "F", value: 5 }, { label: "A", value: 30 }, { label: "G", value: 4 },
    { label: "E", value: 6 }, { label: "B", value: 20 }, { label: "D", value: 10 }, { label: "C", value: 15 }, { label: "K", value: 1 }
  ];
  const previous = [
    { label: "G", value: 1 }, { label: "F", value: 40 }, { label: "H", value: 20 },
    { label: "A", value: 15 }, { label: "I", value: 10 }, { label: "J", value: 9 }, { label: "B", value: 5 }, { label: "L", value: 1 }
  ];
  const rows = comparisonRows(current, previous, true);
  assert.deepEqual(rows.map(row => row.label), ["A", "B", "C", "D", "E", "F", "G", "H", "I", "J"]);
  assert.equal(rows.find(row => row.label === "B").previous, 5 / 101 * 100);
  assert.equal(rows.find(row => row.label === "F").count, 5);
  assert.equal(rows.find(row => row.label === "A").value, 30 / 91 * 100);
  assert.equal(rows.find(row => row.label === "H").delta, -19.8);
});

test("uses top seven categories and top five actors with deterministic tie ordering", () => {
  const current = "GFEDCBA".split("").map(label => ({ label, value: 1 }));
  const previous = "NMKJLIH".split("").map(label => ({ label, value: 1 }));
  assert.deepEqual(comparisonRows(current, current, false).map(row => row.label), ["A", "B", "C", "D", "E"]);
  assert.equal(comparisonRows(current, previous, false).length, 10);
  assert.deepEqual(comparisonRows(current, null, false).map(row => row.label), ["A", "B", "C", "D", "E"]);
  assert.deepEqual(comparisonRows([], previous, false).map(row => row.label), ["H", "I", "J", "K", "L"]);
  assert.equal(comparisonRows(current, current, true).length, 7);
  assert.equal(comparisonRows(current, previous, true).length, 14);
});
