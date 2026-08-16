(function (root, factory) {
  "use strict";

  const reportThreats = factory();
  if (typeof module === "object" && module.exports) module.exports = reportThreats;
  if (root) root.reportThreats = reportThreats;
})(typeof window === "undefined" ? globalThis : window, function () {
  "use strict";

  return function reportThreats(report) {
    const limit = report && report.type === "monthly" ? 10 : report && report.type === "weekly" ? 3 : 1;
    if (Array.isArray(report && report.top_threats) && report.top_threats.length) {
      return report.top_threats.filter(threat => String(threat && threat.severity || "").toUpperCase() === "CRITICAL").slice(0, limit);
    }
    const title = String(report && report.top_threat || "").trim();
    return title ? [{ title, severity: "", published_at: "", source_count: 1 }] : [];
  };
});
