"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const reportThreats = require("../static/report-utils.js");

test("report threats keep only critical structured entries", () => {
  const report = {
    type: "weekly",
    top_threat: "Legacy title",
    top_threats: [
      { title: "Primary incident", severity: "CRITICAL", published_at: "2026-08-15", source_count: 2 },
      { title: "Second incident", severity: "HIGH", published_at: "2026-08-14", source_count: 1 }
    ]
  };

  assert.deepEqual(reportThreats(report), report.top_threats.slice(0, 1));
});

test("report threats enforce weekly and monthly display limits", () => {
  const threats = Array.from({ length: 11 }, (_, index) => ({ title: `Incident ${index + 1}`, severity: "CRITICAL" }));

  assert.deepEqual(reportThreats({ type: "weekly", top_threats: threats }), threats.slice(0, 3));
  assert.deepEqual(reportThreats({ type: "monthly", top_threats: threats }), threats.slice(0, 10));
});

test("report threats omit structured entries below critical", () => {
  assert.deepEqual(reportThreats({ type: "weekly", top_threats: [
    { title: "High incident", severity: "HIGH" },
    { title: "Medium incident", severity: "MEDIUM" }
  ] }), []);
});

test("report threats preserve reports created before structured lists", () => {
  assert.deepEqual(reportThreats({ top_threat: "Legacy incident" }), [
    { title: "Legacy incident", severity: "", published_at: "", source_count: 1 }
  ]);
  assert.deepEqual(reportThreats({ top_threat: "  " }), []);
});

test("index loads report utilities before report renderers", () => {
  const source = fs.readFileSync(path.join(__dirname, "../static/index.html"), "utf8");
  const utilities = source.indexOf('<script defer src="/report-utils.js"></script>');
  const exporter = source.indexOf('<script defer src="/export.js"></script>');
  const app = source.indexOf('<script defer src="/app.js"></script>');

  assert.ok(utilities >= 0 && utilities < exporter && exporter < app);
});
