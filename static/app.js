(() => {
  "use strict";

  const TEXT = {
    ko: {
      dashboard: "대시보드", reports: "보고서", settings: "설정", newReport: "새로 만들기",
      total: "총 기사", critical: "Critical", high: "High", cve: "CVE",
      attackMethods: "위협 유형 분포", articleClassified: "기사 본문 기반 AI 분류", aiGenerated: "AI 자동 생성", threatActors: "위협 행위자 분포",
      windowCumulative: days => `최근 ${days}일 누적 기준`, recentCVEs: "최근 언급된 CVE",
      aggregationRange: "집계 기간", range7: "최근 7일", range30: "최근 30일", range90: "최근 90일", hideNoneActor: "None 제외",
      collectionTrend: "수집 추이", collectionTrendHint: days => `${days}일 단위 · 막대는 심각도, 선은 전체 기사`,
      attributionTrend: "행위자 판별 추이", attributionTrendHint: days => `${days}일 단위 · 막대는 판별 수준, 선은 행위자가 있는 기사`,
      actorNamed: "판별됨", actorQualified: "부분 판별", actorUnknown: "미판별", actorAttributed: "행위자 있음",
      cveHint: "NVD API 보강 · 최초 등장일 최신순", viewAllCVEs: "전체 CVE 보기", allCVEs: "전체 CVE 목록",
      cveExplorerHint: "CVSS + 언급수 × 0.2 기준 내림차순", cveSortLabel: "정렬 기준", cveHintCVSS: "CVSS 점수 높은 순", cveHintMentions: "언급 수 많은 순", cveHintFirstSeen: "최초 등장일 최신순", cveScrollHint: "좌우로 스크롤하여 전체 열을 확인하세요.", rank: "순위", riskScore: "정렬 점수", entries: "개", backToDashboard: "대시보드로 돌아가기", refreshCVEs: "CVE 갱신", refreshingCVEs: "CVE 갱신 중…", cvssPending: "NVD 평가 대기", cveLoadFailed: "CVE 목록을 불러오지 못했습니다.", retry: "다시 시도", noData: "아직 수집된 데이터가 없습니다",
      noDataHint: "왼쪽 캘린더에서 최근 10일 이내 날짜를 선택해 첫\u00a0수집을 시작하세요.",
      noArticles: "이 날짜에 수집된 기사가 없습니다", collectNow: "수집을 시작하시겠습니까?",
      sourcesActive: "개의 활성 소스에서 메타데이터를 가져옵니다.", cancel: "취소", close: "닫기", start: "수집 시작",
      collecting: "수집 중…", collectingHint: "닫아도 백그라운드에서 계속 수집합니다.", collectionCancelled: "수집을 취소했습니다.", collected: "수집을 완료했습니다.", sourceSettings: "수집 소스", sourceOn: "활성", sourceOff: "비활성",
      language: "언어", languageHint: "대시보드와 AI 요약에 사용할 언어를 선택합니다.",
      nvdTitle: "NVD API 키", nvdHint: "수집을 시작하려면 NVD API 키를 먼저 등록해야 합니다.",
      llmTitle: "LLM 설정", llmHint: "OpenAI Chat Completions 호환 엔드포인트를 연결합니다.",
      baseURL: "Base URL", model: "모델 이름", apiKey: "API 키", timeout: "타임아웃(초)",
      keySaved: "키 저장됨", keyStoredPlaceholder: "저장된 키 유지", keyInputHint: "저장된 키는 표시하지 않습니다. 비워 두면 기존 키를 유지합니다.",
      preset: "프리셋", presetAdd: "현재 설정 추가", presetAddHint: "입력한 Base URL과 모델을 프리셋으로 저장", presetRemove: "프리셋 삭제",
      presetAdded: "프리셋을 추가했습니다.", presetRemoved: "프리셋을 삭제했습니다.",
      save: "설정 저장", test: "연결 테스트", requestPreview: "요청 미리보기", schema: "기사 분류 스키마",
      unsavedSettings: "저장하지 않은 변경사항이 있습니다.", revert: "되돌리기",
      sourceFilter: "뉴스 출처", allSources: "모든 출처", recollect: "재수집", recollectConfirm: "이 날짜의 뉴스를 다시 수집하시겠습니까?",
      timezone: "시간대", timezoneHint: "보고서 저장 시간에 적용됩니다.",
      reportTitle: "보고서 생성", reportHint: "기간 내 일간 요약을 바탕으로 주간·월간 보고서를 만듭니다.", weekly: "주간", monthly: "월간",
      pickYear: "연도", pickMonth: "월", pickWeek: "주 선택 (일 – 토)", articleUnit: "건", articlesUnit: "건", scrollHint: "주간 범위를 선택하세요",
      periodStart: "시작일", periodEnd: "종료일", generate: "생성", topThreat: "주요 위협",
      keyActors: "핵심 위협 행위자", summary: "요약", targetSectors: "주요 타겟 섹터", medium: "Medium",
      firstSeen: "최초 등장", mentions: "언급", product: "제품 / 벤더", article: "기사", articleCount: "기사",
      reportWord: "보고서", downloadPDF: "PDF 다운로드", downloadReportPDF: "보고서 PDF 다운로드", downloadDailyPDF: "일일요약 PDF 다운로드",
      pdfStarted: "PDF 저장 창을 열었습니다. 저장하거나 취소하면 창이 닫힙니다.", pdfBlocked: "PDF 저장 창을 열 수 없습니다. 팝업 차단을 확인하세요.", generatedBy: "Cyber Dashboard에서 생성",
      date: "날짜", noSummary: "요약이 없습니다.", none: "없음",
      saved: "설정을 저장했습니다.", connectionOK: "연결되었습니다.", connectionFail: "연결에 실패했습니다.",
      outsideRetention: "피드 보존 기간(10일)을 벗어난 날짜입니다.", futureDate: "미래 날짜는 선택할 수 없습니다.",
      emptyReport: "선택한 기간에 수집된 기사가 없습니다.",
      noReports: "생성된 보고서가 없습니다", feedSubtitle: "위협 인텔리전스 피드", dailySummary: "일간 요약",
      settingsSub: "보안 · 언어 · 소스 · API 설정", testing: "확인 중…", unknownActor: "미확인",
      cveRefreshDone: (updated, removed) => `CVE ${updated}개를 갱신하고 ${removed}개를 제거했습니다.`,
      cveRefreshWarned: "일부 항목을 확인하세요.", deleteReport: "보고서 삭제", deleteReportTitle: "보고서 삭제",
      deleteReportConfirm: "정말로 삭제하시겠습니까?", deleteReportHint: "삭제한 보고서는 복구할 수 없습니다.",
      deleting: "삭제 중…", reportDeleted: "보고서를 삭제했습니다.",
      licenseTitle: "라이선스", licenseHint: "Cyber Dashboard와 함께 배포되는 오픈소스 라이선스를 확인합니다.",
      viewLicenses: "라이선스 확인", licenseDialogTitle: "라이선스 및 오픈소스 고지",
      licenseDialogHint: "본 프로그램과 서드파티 구성요소의 라이선스 원문입니다.",
      programLicense: "본 프로그램", thirdPartyLicenses: "서드파티",
      loadingLicenses: "라이선스를 불러오는 중…", licenseLoadFailed: "라이선스를 불러오지 못했습니다.",
      login: "로그인", logout: "로그아웃", loginTitle: "관리자 로그인", loginHint: "관리 기능을 사용하려면 비밀번호를 입력하세요.",
      password: "비밀번호", signingIn: "로그인 중…", securityTitle: "비밀번호 변경", securityHint: "변경하면 다른 브라우저의 로그인도 만료됩니다.",
      currentPassword: "현재 비밀번호", newPassword: "새 비밀번호", confirmPassword: "새 비밀번호 확인", changePassword: "비밀번호 변경",
      passwordRule: "12바이트 이상 128바이트 이하", passwordMismatch: "새 비밀번호가 일치하지 않습니다.", passwordChanged: "비밀번호를 변경했습니다.",
      changingPassword: "변경 중…", requestFailed: "요청을 처리하지 못했습니다."
    },
    en: {
      dashboard: "Dashboard", reports: "Reports", settings: "Settings", newReport: "New",
      total: "Total", critical: "Critical", high: "High", cve: "CVE",
      attackMethods: "Threat category breakdown", articleClassified: "AI-classified from article content", aiGenerated: "Auto-generated by AI", threatActors: "Threat actor breakdown",
      windowCumulative: days => `Last ${days} days cumulative`, recentCVEs: "Recently mentioned CVEs",
      aggregationRange: "Aggregation period", range7: "Last 7 days", range30: "Last 30 days", range90: "Last 90 days", hideNoneActor: "Exclude None",
      collectionTrend: "Collection trend", collectionTrendHint: days => `${days}-day buckets · bars by severity, line is every article`,
      attributionTrend: "Attribution trend", attributionTrendHint: days => `${days}-day buckets · bars by precision, line is articles with an actor`,
      actorNamed: "Identified", actorQualified: "Partly identified", actorUnknown: "Unidentified", actorAttributed: "Has actor",
      cveHint: "Enriched via NVD API · newest first", viewAllCVEs: "View all CVEs", allCVEs: "All CVEs",
      cveExplorerHint: "Ranked by CVSS + mentions × 0.2", cveSortLabel: "Sort by", cveHintCVSS: "Ranked by CVSS score", cveHintMentions: "Ranked by mention count", cveHintFirstSeen: "Newest first seen first", cveScrollHint: "Scroll horizontally to view every column.", rank: "Rank", riskScore: "Rank score", entries: "entries", backToDashboard: "Back to dashboard", refreshCVEs: "Refresh CVEs", refreshingCVEs: "Refreshing CVEs…", cvssPending: "NVD assessment pending", cveLoadFailed: "The CVE catalogue could not be loaded.", retry: "Retry", noData: "No collected data yet",
      noDataHint: "Pick a date within the last 10 days in the calendar to start your first collection.",
      noArticles: "No articles collected for this date", collectNow: "Start collection for this date?",
      sourcesActive: " active sources will provide metadata.", cancel: "Cancel", close: "Close", start: "Start",
      collecting: "Collecting…", collectingHint: "You can close this dialog while collection continues in the background.", collectionCancelled: "Collection cancelled.", collected: "Collection finished.", sourceSettings: "Collection sources", sourceOn: "Active", sourceOff: "Off",
      language: "Language", languageHint: "Choose the language used by the dashboard and AI summaries.",
      nvdTitle: "NVD API key", nvdHint: "Register an NVD API key before starting collection.",
      llmTitle: "LLM settings", llmHint: "Connect any OpenAI Chat Completions compatible endpoint.",
      baseURL: "Base URL", model: "Model name", apiKey: "API key", timeout: "Timeout (s)",
      keySaved: "Key saved", keyStoredPlaceholder: "Keep saved key", keyInputHint: "Saved keys are never displayed. Leave this blank to keep the existing key.",
      preset: "Presets", presetAdd: "Add current", presetAddHint: "Save the current Base URL and model as a preset", presetRemove: "Remove preset",
      presetAdded: "Preset added.", presetRemoved: "Preset removed.",
      save: "Save settings", test: "Test connection", requestPreview: "Request preview", schema: "Article classification schema",
      unsavedSettings: "You have unsaved changes.", revert: "Reset",
      sourceFilter: "News source", allSources: "All sources", recollect: "Recollect", recollectConfirm: "Recollect news for this date?",
      timezone: "Timezone", timezoneHint: "Applied when reports are saved.",
      reportTitle: "Generate report", reportHint: "Builds weekly and monthly reports from daily summaries in the selected range.", weekly: "Weekly", monthly: "Monthly",
      pickYear: "Year", pickMonth: "Month", pickWeek: "Pick a week (Sun – Sat)", articleUnit: "item", articlesUnit: "items", scrollHint: "Select a weekly range",
      periodStart: "Start", periodEnd: "End", generate: "Generate", topThreat: "Top threats",
      keyActors: "Key threat actors", summary: "Summary", targetSectors: "Target sectors", medium: "Medium",
      firstSeen: "First seen", mentions: "Mentions", product: "Product / Vendor", article: "Article", articleCount: "Articles",
      reportWord: "report", downloadPDF: "Download PDF", downloadReportPDF: "Download report PDF", downloadDailyPDF: "Download daily summary PDF",
      pdfStarted: "The PDF save dialog opened. The window closes after saving or canceling.", pdfBlocked: "The PDF save dialog could not open. Check your popup blocker.", generatedBy: "Generated by Cyber Dashboard",
      date: "Date", noSummary: "No summary available.", none: "None",
      saved: "Settings saved.", connectionOK: "Connected.", connectionFail: "Connection failed.",
      outsideRetention: "This date is outside the 10-day feed retention window.", futureDate: "Future dates cannot be selected.",
      emptyReport: "No articles were collected in this period.",
      noReports: "No reports yet", feedSubtitle: "Threat intelligence feed", dailySummary: "Daily summary",
      settingsSub: "Security · Language · Sources · API settings", testing: "Testing…", unknownActor: "Unknown",
      cveRefreshDone: (updated, removed) => `Updated ${updated} CVEs and removed ${removed}.`,
      cveRefreshWarned: "Review the warnings for some entries.", deleteReport: "Delete report", deleteReportTitle: "Delete report",
      deleteReportConfirm: "Are you sure you want to delete this report?", deleteReportHint: "Deleted reports cannot be recovered.",
      deleting: "Deleting…", reportDeleted: "Report deleted.",
      licenseTitle: "Licenses", licenseHint: "Review the licenses for Cyber Dashboard and its bundled open-source software.",
      viewLicenses: "View licenses", licenseDialogTitle: "Licenses and open-source notices",
      licenseDialogHint: "License terms for this program and its third-party components.",
      programLicense: "This program", thirdPartyLicenses: "Third party",
      loadingLicenses: "Loading license…", licenseLoadFailed: "The license could not be loaded.",
      login: "Log in", logout: "Log out", loginTitle: "Administrator login", loginHint: "Enter the password to use administrative features.",
      password: "Password", signingIn: "Logging in…", securityTitle: "Change password", securityHint: "Changing it expires logins in other browsers.",
      currentPassword: "Current password", newPassword: "New password", confirmPassword: "Confirm new password", changePassword: "Change password",
      passwordRule: "12 to 128 bytes", passwordMismatch: "The new passwords do not match.", passwordChanged: "Password changed.",
      changingPassword: "Changing…", requestFailed: "The request could not be completed."
    }
  };

  // Only seeds the first render; /api/bootstrap overwrites it. Stale values can name a dropped language.
  const cachedLanguage = () => {
    const language = localStorage.getItem("cyber-lang");
    return TEXT[language] ? language : "en";
  };

  // Duplicated by the pre-paint script in index.html — keep both in sync.
  const resolveTheme = () => {
    const stored = localStorage.getItem("cyber-theme");
    if (stored === "dark" || stored === "light") return stored;
    return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
  };

  const cachedHideNoneActor = () => localStorage.getItem("cyber-hide-none-actor") === "1";
  // One list drives the options and both entry points; the server rejects the rest.
  const dashboardRanges = [7, 30, 90];
  // A stale value can name a range the server no longer takes, so fall back to the default.
  const normalizedDashboardDays = value => dashboardRanges.includes(Number(value)) ? Number(value) : 30;
  const cachedDashboardDays = () => normalizedDashboardDays(localStorage.getItem("cyber-dashboard-days"));
  const cveSortKeys = ["score", "cvss", "mentions", "firstSeen"];
  const cvePageSize = 100;
  const normalizedCVESort = value => cveSortKeys.includes(String(value)) ? String(value) : "score";
  const cachedCVESort = () => normalizedCVESort(localStorage.getItem("cyber-dashboard-cve-sort"));

  const state = {
    view: "dashboard",
    lang: cachedLanguage(),
    theme: resolveTheme(),
    dashboardDays: cachedDashboardDays(),
    cveSort: cachedCVESort(),
    hideNoneActor: cachedHideNoneActor(),
    // Placeholders until /api/bootstrap supplies the configured offset; start() re-derives both.
    month: null,
    selectedDay: null,
    bootstrap: { auth: { enabled: false, authenticated: true }, sources: [], reports: [], settings: {}, llm_presets: [], collected_days: [] },
    dashboard: null,
    daily: null,
    currentReport: null,
    reportType: "weekly",
    reportYear: null,
    reportMonth: null,
    reportWeekStart: null,
    dailySource: "all",
    sourceDraft: null,
    dashboardScroll: 0
  };
  let modalLastFocus = null;
  let cvesRequest = 0;
  let viewRequest = 0;
  let dashboardRequest = 0;
  let renderedScrollKey = null;
  let mainResizeObserver = null;
  let pendingLogin = null;
  const licenseDocuments = { program: "/legal/LICENSE.txt", thirdParty: "/legal/THIRD_PARTY_NOTICES.txt" };
  const licenseDocumentCache = {};
  const collectionTask = window.createCollectionTask(serverCollection);
  const cveRefreshTask = window.createCVERefreshTask(serverCVERefresh);

  const t = key => TEXT[state.lang][key] || key;
  const articleCountLabel = count => state.lang === "en" && Number(count) === 1 ? t("article") : t("articleCount");
  // Single writer for state.lang, so the cache can't drift from the server.
  function adoptLanguage(language) {
    state.lang = TEXT[language] ? language : state.lang;
    localStorage.setItem("cyber-lang", state.lang);
  }
  // Escape text and quoted attributes because LLM and user values may contain quotes.
  const esc = value => String(value == null ? "" : value)
    .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  const toneClass = severity => severity === "CRITICAL" ? "badge-danger" : severity === "HIGH" ? "badge-warning" : severity === "MEDIUM" ? "badge-info" : "";
  const chartColor = index => `var(--chart-${(index % 5) + 1})`;
  // Every dashboard fetch shares one path so the cached response matches the visible controls.
  const dashboardPath = () => `/api/dashboard?days=${state.dashboardDays}${state.hideNoneActor ? "&hide_none=1" : ""}`;

  function cvssBadgeHTML(value) {
    const score = Number(value) || 0;
    const tone = score <= 0 ? "" : score >= 9 ? "badge-danger" : score >= 7 ? "badge-warning" : "badge-success";
    const pending = score <= 0 ? ` title="${esc(t("cvssPending"))}" aria-label="CVSS 0.0 · ${esc(t("cvssPending"))}"` : "";
    const label = score <= 0 ? `${score.toFixed(1)} · ${esc(t("cvssPending"))}` : score.toFixed(1);
    return `<span class="badge${tone ? ` ${tone}` : ""}"${pending}>${label}</span>`;
  }

  function cveRiskScore(cve) {
    return Number(cve.cvss) + Number(cve.mentions) * 0.2;
  }

  function cveSortHint() {
    return t({ cvss: "cveHintCVSS", mentions: "cveHintMentions", firstSeen: "cveHintFirstSeen" }[state.cveSort] || "cveExplorerHint");
  }

  function cveExplorerRowsHTML(cves) {
    const rows = cves.map((cve, index) => `<tr data-href="https://nvd.nist.gov/vuln/detail/${encodeURIComponent(cve.id)}" tabindex="0"><td class="mono" data-label="${esc(t("rank"))}">${index + 1}</td><td class="mono cve-link" data-label="CVE ID">${esc(cve.id)}</td><td data-label="CVSS">${cvssBadgeHTML(cve.cvss)}</td><td data-label="${esc(t("mentions"))}">${cve.mentions}</td><td class="mono" data-label="${esc(t("riskScore"))}">${cveRiskScore(cve).toFixed(1)}</td><td data-label="${esc(t("product"))}">${esc(cve.affected_product)}</td><td class="mono" data-label="${esc(t("firstSeen"))}">${esc(cve.first_seen)}</td></tr>`).join("");
    return rows || `<tr><td colspan="7">${esc(t("noData"))}</td></tr>`;
  }

  function normalizedEndpoint(value) {
    return String(value || "").trim().replace(/\/+$/, "");
  }

  function matchesPreset(preset, baseURL, model) {
    return normalizedEndpoint(preset.base_url) === normalizedEndpoint(baseURL) && preset.model === String(model || "").trim();
  }

  function canAddPreset(baseURL, model) {
    try {
      const parsed = new URL(String(baseURL || "").trim());
      const validProtocol = parsed.protocol === "http:" || parsed.protocol === "https:";
      const duplicate = (state.bootstrap.llm_presets || []).some(preset => matchesPreset(preset, baseURL, model));
      return validProtocol && String(model || "").trim().length > 0 && !duplicate;
    } catch (_) {
      return false;
    }
  }

  function llmKeyIsConfigured(settings) {
    if (String(settings.llm_api_key || "").trim()) return true;
    const preset = (state.bootstrap.llm_presets || []).find(item => matchesPreset(item, settings.llm_base_url, settings.llm_model));
    if (preset?.api_key_configured) return true;
    const current = state.bootstrap.settings;
    return Boolean(current.llm_api_key_configured && normalizedEndpoint(current.llm_base_url) === normalizedEndpoint(settings.llm_base_url) && current.llm_model === settings.llm_model);
  }

  function formatDay(date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, "0");
    const day = String(date.getDate()).padStart(2, "0");
    return `${year}-${month}-${day}`;
  }

  function parseDay(day) {
    const [year, month, date] = day.split("-").map(Number);
    return new Date(year, month - 1, date);
  }

  // Derive every UI "today" from the configured UTC offset, not the device clock.
  function configuredToday() {
    const offset = Number(state.bootstrap.settings.timezone_offset_minutes) || 0;
    const shifted = new Date(Date.now() + offset * 60000);
    return new Date(shifted.getUTCFullYear(), shifted.getUTCMonth(), shifted.getUTCDate());
  }

  function applyConfiguredToday() {
    const today = configuredToday();
    if (!state.selectedDay) state.selectedDay = formatDay(today);
    if (!state.month) state.month = new Date(today.getFullYear(), today.getMonth(), 1);
  }

  // The feed keeps 10 days inclusive of today, matching the server's collect validation.
  function retentionWindow() {
    const today = configuredToday();
    const oldest = new Date(today);
    oldest.setDate(oldest.getDate() - 9);
    return { today, oldest };
  }

  function timezoneOptions(selected) {
    const options = [];
    for (let minutes = -12 * 60; minutes <= 14 * 60; minutes += 15) {
      const sign = minutes >= 0 ? "+" : "−";
      const absolute = Math.abs(minutes);
      const label = `UTC${sign}${String(Math.floor(absolute / 60)).padStart(2, "0")}:${String(absolute % 60).padStart(2, "0")}`;
      options.push(`<option value="${minutes}"${minutes === selected ? " selected" : ""}>${label}</option>`);
    }
    return options.join("");
  }

  function formatDisplayDate(day) {
    return new Intl.DateTimeFormat(state.lang === "ko" ? "ko-KR" : "en-US", {
      year: "numeric", month: "long", day: "numeric", weekday: "long"
    }).format(parseDay(day));
  }

  function formatTrendDate(day, compact = false) {
    return new Intl.DateTimeFormat(state.lang === "ko" ? "ko-KR" : "en-US", compact
      ? { month: "numeric", day: "numeric" }
      : { year: "numeric", month: "short", day: "numeric" }).format(parseDay(day));
  }

  function formatArticleTime(value) {
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime()) || !String(value).includes("T")) return value;
    return new Intl.DateTimeFormat(state.lang === "ko" ? "ko-KR" : "en-US", {
      hour: "2-digit", minute: "2-digit", hour12: false, timeZone: "UTC"
    }).format(parsed) + " UTC";
  }

  const { request, api, refreshSession } = window.createAuthTransport({
    $, locks: navigator.locks, isAuthenticated, onSessionExpired: resumeAfterLogin
  });

  // A refresh that fails mid-request means the session expired under the user. The view stays on
  // screen so nothing they were editing is discarded, and the request waits on the login modal
  // rather than failing outright — logging back in completes it.
  function resumeAfterLogin(retry, error) {
    state.bootstrap.auth.authenticated = false;
    renderAuthAction();
    const deferred = $.Deferred();
    // A second request can expire while the first is already waiting; both have to settle.
    const displaced = pendingLogin;
    pendingLogin = {
      expired: true,
      resume: () => { retry().then(deferred.resolve, deferred.reject); if (displaced) displaced.resume(); },
      cancel: () => { deferred.reject(error); if (displaced && displaced.cancel) displaced.cancel(); }
    };
    openLoginModal();
    return deferred.promise();
  }

  function isAuthenticated() {
    const auth = state.bootstrap.auth;
    return !auth || !auth.enabled || Boolean(auth.authenticated);
  }

  function renderAuthAction() {
    const enabled = Boolean(state.bootstrap.auth && state.bootstrap.auth.enabled);
    const authenticated = isAuthenticated();
    const loginSlot = enabled && !authenticated;
    $("#auth-action").prop("hidden", !enabled || !authenticated);
    $("#open-report-modal").prop("hidden", !authenticated);
    // The button's own label names it; only the landmark needs a separate one.
    $("#settings-navigation").attr("aria-label", t(loginSlot ? "login" : "settings"));
    $("#settings-action").prop("hidden", false);
    $("#settings-action .nav-symbol").text(loginSlot ? "L" : "S");
    $("#settings-action .settings-action-label").text(t(loginSlot ? "login" : "settings"));
  }

  function requireAdmin(action) {
    if (isAuthenticated()) return true;
    pendingLogin = action ? { expired: false, resume: action, cancel: null } : null;
    openLoginModal();
    return false;
  }

  // Password managers need a username to attach a saved entry to; the account is fixed, so the
  // field exists for them alone.
  function adminUsernameField() {
    return `<input type="text" name="username" value="admin" autocomplete="username" readonly tabindex="-1" hidden>`;
  }

  function openLoginModal() {
    closeDrawer();
    openModal(`<form class="modal modal-small" id="login-form" role="dialog" aria-modal="true" aria-labelledby="login-title">
      <div class="modal-header"><div><h2 id="login-title">${esc(t("loginTitle"))}</h2><p>${esc(t("loginHint"))}</p></div><button class="icon-button modal-dismiss" type="button" aria-label="${esc(t("close"))}">×</button></div>
      <div class="modal-body">${adminUsernameField()}<div class="field"><label for="login-password">${esc(t("password"))}</label><input id="login-password" type="password" autocomplete="current-password" required><small class="field-message" role="alert" hidden></small></div></div>
      <div class="modal-footer"><button class="secondary-button modal-close" type="button">${esc(t("cancel"))}</button><button class="primary-button" id="submit-login" type="submit">${esc(t("login"))}</button></div>
    </form>`, "#login-password");
  }

  // .field-error is the sheet's validation style; the message lands in the field's own <small>, so
  // a field whose <small> already carries rule text only needs recolouring.
  function showFieldError(selector, message) {
    const $field = $(selector).closest(".field").addClass("field-error");
    if (message) $field.find(".field-message").text(message).prop("hidden", false);
    $(selector).trigger("focus").trigger("select");
  }

  function clearFieldErrors(scope) {
    $(scope).find(".field-error").removeClass("field-error");
    $(scope).find(".field-message").text("").prop("hidden", true);
  }

  function loadBootstrap() {
    return request("GET", "/api/bootstrap").then(data => {
      state.bootstrap = data;
      adoptLanguage(data.settings.language);
      applyConfiguredToday();
      applyChrome();
      return data;
    });
  }

  function beginView(view) {
    state.view = view;
    return ++viewRequest;
  }

  // Bootstrap carries only sidebar entries, so the body is read fresh every time a report opens.
  function openReport(id) {
    const request = beginView("report");
    state.currentReport = null;
    setLoading();
    api("GET", `/api/reports/${encodeURIComponent(id)}`).done(report => {
      if (request === viewRequest) renderReport(report);
    }).fail(error => {
      if (request === viewRequest) showRequestError(error);
    });
  }

  function submitLogin() {
    const password = $("#login-password").val();
    clearFieldErrors("#login-form");
    const $button = $("#submit-login").prop("disabled", true).text(t("signingIn"));
    request("POST", "/api/auth/login", { password }).then(loadBootstrap).done(() => {
      const pending = pendingLogin;
      pendingLogin = null;
      closeModal();
      if (pending) pending.resume();
      else routeCurrentView();
    }).fail(error => {
      $button.prop("disabled", false).text(t("login"));
      // The dialog is aria-modal, so a toast outside it is the one place the message must not go.
      showFieldError("#login-password", errorMessage(error));
    });
  }

  function logout() {
    request("POST", "/api/auth/logout").always(() => {
      loadBootstrap().done(() => {
        if (state.view === "settings") renderDashboard();
        else routeCurrentView();
      }).fail(showRequestError);
    });
  }

  function serverCollection(day, existingJob = null) {
    const deferred = $.Deferred();
    let jobID = null;
    let timer = null;
    let cancelling = false;
    let cancelInFlight = false;
    const startRequest = existingJob ? $.Deferred().resolve(existingJob).promise() : api("POST", "/api/collect", { date: day });

    function scheduleCheck() {
      if (deferred.state() === "pending") timer = window.setTimeout(check, 1000);
    }

    function settle(job) {
      if (job.status === "running") {
        if (cancelling) cancel();
        else scheduleCheck();
      } else if (job.status === "completed") {
        deferred.resolve(job.result);
      } else if (job.status === "cancelled") {
        deferred.reject({}, "abort");
      } else {
        deferred.reject({ responseJSON: { message: job.error || "Collection failed" } }, "error");
      }
    }

    function check() {
      api("GET", `/api/collect/${encodeURIComponent(jobID)}`).done(job => {
        settle(job);
      }).fail(scheduleCheck);
    }

    function cancel() {
      if (!jobID || cancelInFlight || deferred.state() !== "pending") return;
      cancelInFlight = true;
      api("DELETE", `/api/collect/${encodeURIComponent(jobID)}`).done(job => {
        cancelInFlight = false;
        settle(job);
      }).fail(() => {
        cancelInFlight = false;
        scheduleCheck();
      });
    }

    startRequest.done(job => {
      jobID = job.id;
      settle(job);
    }).fail(error => deferred.reject(error, "error"));
    return deferred.promise({
      abort() {
        cancelling = true;
        if (timer !== null) window.clearTimeout(timer);
        cancel();
      }
    });
  }

  function serverCVERefresh(existingJob = null) {
    const deferred = $.Deferred();
    let jobID = null;
    let timer = null;
    const startRequest = existingJob ? $.Deferred().resolve(existingJob).promise() : api("POST", "/api/cves/refresh");

    function scheduleCheck() {
      if (deferred.state() === "pending") timer = window.setTimeout(check, 1000);
    }

    function settle(job) {
      if (job.status === "running") {
        scheduleCheck();
      } else if (job.status === "completed") {
        deferred.resolve(job.result);
      } else {
        deferred.reject({ responseJSON: { message: job.error || "CVE refresh failed" } }, "error");
      }
    }

    function check() {
      api("GET", "/api/cves/refresh/" + encodeURIComponent(jobID)).done(settle).fail(scheduleCheck);
    }

    startRequest.done(job => {
      jobID = job.id;
      settle(job);
    }).fail((error, status) => deferred.reject(error, status));

    return deferred.promise({
      abort() {
        if (timer !== null) window.clearTimeout(timer);
      }
    });
  }

  function safeURL(raw) {
    try {
      const parsed = new URL(raw);
      return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.href : "#";
    } catch (_) {
      return "#";
    }
  }

  // The header carries either subtitle text or a view's own control, never both.
  function setHeader(title, subtitle) {
    $("#page-title").text(title);
    $("#page-subtitle").text(subtitle).prop("hidden", !subtitle);
    $("#dashboard-range").prop("hidden", true);
    document.title = `${title} · Cyber Dashboard`;
  }

  function clearCVEHash() {
    if (window.location.hash !== "#cves") return;
    window.history.replaceState(null, "", window.location.pathname + window.location.search);
  }

  function applyViewScroll(key, offset = 0) {
    if (renderedScrollKey !== key) $("#main-content").scrollTop(offset);
    renderedScrollKey = key;
  }

  function setLoading() {
    $("#main-content").html(`<div class="content stats-grid" aria-label="Loading">
      ${Array.from({ length: 4 }, () => '<div class="skeleton skeleton-card"></div>').join("")}
    </div>`);
  }

  function toast(message, error = false) {
    const $toast = $("<div>", { class: `toast${error ? " is-error" : ""}`, text: message });
    const $region = $("#toast-region").append($toast);
    $region.children().slice(0, -3).remove();
    window.setTimeout(() => $toast.remove(), 4000);
  }

  function applyChrome() {
    document.documentElement.lang = state.lang;
    document.documentElement.dataset.theme = state.theme;
    $("#theme-toggle").attr("aria-pressed", String(state.theme === "light"));
    $("[data-i18n]").each(function () { $(this).text(t($(this).data("i18n"))); });
    renderAuthAction();
    renderCalendar();
    renderReportList();
  }

  function renderCalendar() {
    const year = state.month.getFullYear();
    const month = state.month.getMonth();
    const locale = state.lang === "ko" ? "ko-KR" : "en-US";
    $("#calendar-label").text(new Intl.DateTimeFormat(locale, { year: "numeric", month: "long" }).format(state.month));
    const weekdays = state.lang === "ko" ? ["일", "월", "화", "수", "목", "금", "토"] : ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];
    $("#calendar-weekdays").html(weekdays.map(day => `<span>${day}</span>`).join(""));
    const first = new Date(year, month, 1).getDay();
    const daysInMonth = new Date(year, month + 1, 0).getDate();
    const previousDays = new Date(year, month, 0).getDate();
    const cells = [];
    for (let index = first - 1; index >= 0; index -= 1) cells.push({ value: previousDays - index, outside: true });
    for (let day = 1; day <= daysInMonth; day += 1) cells.push({ value: day, outside: false });
    let next = 1;
    while (cells.length % 7 !== 0) cells.push({ value: next++, outside: true });
    const { today, oldest } = retentionWindow();
    const dataDays = new Set(state.bootstrap.collected_days || []);
    const html = cells.map(cell => {
      if (cell.outside) return `<button class="calendar-day is-outside" type="button" disabled>${cell.value}</button>`;
      const date = new Date(year, month, cell.value);
      const day = formatDay(date);
      const future = date > today;
      const expired = date < oldest;
      const classes = ["calendar-day"];
      if (day === formatDay(today)) classes.push("is-today");
      if (day === state.selectedDay) classes.push("is-selected");
      if (dataDays.has(day)) classes.push("has-data");
      if (expired) classes.push("is-expired");
      const hint = future ? t("futureDate") : expired ? `${formatDisplayDate(day)} — ${t("outsideRetention")}` : formatDisplayDate(day);
      const disabled = future;
      return `<button class="${classes.join(" ")}" type="button"${disabled ? " disabled" : ` data-day="${day}"`} title="${esc(hint)}" aria-label="${esc(hint)}" aria-pressed="${day === state.selectedDay}"${day === formatDay(today) ? ' aria-current="date"' : ""}>${cell.value}</button>`;
    }).join("");
    $("#calendar-grid").html(html);
  }

  function renderReportList() {
    const reports = state.bootstrap.reports || [];
    if (!reports.length) {
      $("#report-list").html(`<p class="card-subtitle">${esc(t("noReports"))}</p>`);
      return;
    }
    $("#report-list").html(reports.map(report => `<button type="button" class="report-link${state.currentReport && state.currentReport.id === report.id ? " is-active" : ""}" data-report-id="${report.id}">
      <span><strong>${esc(report.type === "weekly" ? `${t("weekly")} · ${report.period_start}` : `${t("monthly")} · ${report.period_start.slice(0, 7)}`)}</strong><small>${esc(report.period_start)} – ${esc(report.period_end)}</small></span>
      <span class="badge ${report.type === "weekly" ? "badge-info" : "badge-success"}">${esc(report.type)}</span>
    </button>`).join(""));
  }

  // The id makes each block addressable so a control can repaint it without a full render.
  function statsHTML(values, id) {
    return `<div class="stats-grid" id="${id}">${values.map(item => `<article class="stat-card"><span>${esc(item.label)}</span><strong class="${item.tone || ""}">${item.value}</strong></article>`).join("")}</div>`;
  }

  function barsHTML(rows, percent, id) {
    if (!rows || !rows.length) return `<div class="empty-state" id="${id}"><p>${esc(t("noData"))}</p></div>`;
    const max = Math.max(...rows.map(row => row.value), 1);
    const total = rows.reduce((sum, row) => sum + row.value, 0);
    return `<div class="bar-list" id="${id}">${rows.map((row, index) => {
      const width = Math.max(3, Math.round(row.value / max * 100));
      const value = percent ? `${Math.round(row.value / Math.max(total, 1) * 100)}%` : row.value;
      // Keep the full label accessible; the selectable visual reveal is aria-hidden to avoid
      // duplicate announcements.
      return `<div class="bar-row"><span class="bar-label">${esc(row.label)}</span><span class="bar-track"><span class="bar-fill" style="width:${width}%;background:${chartColor(index)}"></span></span><span class="bar-value">${value}</span><span class="bar-reveal" aria-hidden="true">${esc(row.label)}</span></div>`;
    }).join("")}</div>`;
  }

  // Measure each label because CSS cannot report clipping; Range rects preserve fractional
  // widths that scrollWidth and clientWidth round away.
  function markClippedBarLabels() {
    const range = document.createRange();
    $(".bar-row").each(function () {
      const label = this.querySelector(".bar-label");
      if (!label) return;
      range.selectNodeContents(label);
      const text = range.getBoundingClientRect().width;
      this.classList.toggle("is-clipped", text > label.getBoundingClientRect().width + 0.05);
    });
  }

  // The 30- and 90-day ranges bucket to these same ten slots, so they share one geometry.
  const trendSlots = 10;
  const trendLabelUnits = 13, trendLabelMaxPx = 12;

  // Axis text scales with the card like the rest of the drawing, up to a ceiling; past that a
  // wide card renders it larger than the card's own heading.
  function fitTrendLabels() {
    $(".chart svg").each(function () {
      const rendered = this.getBoundingClientRect().width;
      if (!rendered) return;
      const scale = rendered / this.viewBox.baseVal.width;
      this.style.setProperty("--trend-label", `${Math.min(trendLabelUnits, trendLabelMaxPx / scale).toFixed(2)}px`);
    });
  }

  function measureCharts() {
    markClippedBarLabels();
    fitTrendLabels();
  }

  function niceCeiling(value) {
    if (value <= 4) return Math.max(value, 1);
    const step = Math.pow(10, Math.floor(Math.log10(value))) / 2;
    return Math.ceil(value / step) * step;
  }

  function bucketDays(points) {
    if (!points || !points.length) return 1;
    return Math.round((parseDay(points[0].end) - parseDay(points[0].start)) / 86400000) + 1;
  }

  const trendSpan = point => point.start === point.end
    ? formatTrendDate(point.start)
    : `${formatTrendDate(point.start)} – ${formatTrendDate(point.end)}`;
  // Line first, then the stacks bottom-up, so a panel reads like the bar it describes.
  const trendSeries = config => [config.line].concat(config.stacks);

  function trendBucketLabel(point, config) {
    const parts = trendSeries(config).map(series => `${series.label} ${point[series.key] || 0}`);
    return `${trendSpan(point)} · ${parts.join(" · ")}`;
  }

  // One combo chart: stacked bars for the parts, a line for the whole they belong to.
  function trendChartHTML(id, points, config) {
    if (!points || !points.length) return `<div class="empty-state" id="${id}"><p>${esc(t("noData"))}</p></div>`;
    const width = 360, height = 172, left = 24, right = 4, top = 8, bottom = 20;
    const plotWidth = width - left - right, plotHeight = height - top - bottom;
    // Too few buckets for the shared grid, so a daily range spreads across the full width.
    const daily = bucketDays(points) === 1;
    const slot = plotWidth / (daily ? points.length : trendSlots);
    const barWidth = Math.min(slot * 0.54, daily ? 30 : 22);
    const stackTotal = point => config.stacks.reduce((sum, series) => sum + (point[series.key] || 0), 0);
    const ceiling = niceCeiling(Math.max(1, ...points.map(point => Math.max(stackTotal(point), point[config.line.key] || 0))));
    const scale = value => top + plotHeight - (value / ceiling) * plotHeight;

    const grid = [0, ceiling / 2, ceiling].map(value => `<line x1="${left}" y1="${scale(value)}" x2="${width - right}" y2="${scale(value)}"></line>
      <text class="trend-axis" x="${left - 4}" y="${scale(value) + 3}" text-anchor="end">${value}</text>`).join("");
    const columns = points.map((point, index) => {
      const x = left + index * slot + (slot - barWidth) / 2;
      let stacked = 0;
      const bars = config.stacks.map(series => {
        const value = point[series.key] || 0;
        if (!value) return "";
        stacked += value;
        return `<rect x="${x.toFixed(1)}" y="${scale(stacked).toFixed(1)}" width="${barWidth.toFixed(1)}" height="${((value / ceiling) * plotHeight).toFixed(1)}" fill="${series.color}"></rect>`;
      }).join("");
      const label = `<text class="trend-axis" x="${(left + index * slot + slot / 2).toFixed(1)}" y="${height - 6}" text-anchor="middle">${esc(formatTrendDate(point.start, true))}</text>`;
      const labelStep = points.length > 7 ? 3 : 2;
      const labelClass = index % labelStep && index !== points.length - 1 ? " trend-label-skip" : "";
      return `<g class="trend-column${labelClass}"><rect class="trend-slot" x="${(left + index * slot).toFixed(1)}" y="${top}" width="${slot.toFixed(1)}" height="${plotHeight}"></rect>${bars}${label}</g>`;
    }).join("");
    const targets = points.map((point, index) => {
      const x = left + index * slot;
      const position = `inset-inline-start:${(x / width * 100).toFixed(3)}%;inset-block-start:${(top / height * 100).toFixed(3)}%;width:${(slot / width * 100).toFixed(3)}%;height:${(plotHeight / height * 100).toFixed(3)}%`;
      return `<span class="trend-bucket" role="img" tabindex="0" style="${position}" aria-label="${esc(trendBucketLabel(point, config))}" aria-describedby="${id}-tip" data-bucket="${index}"></span>`;
    }).join("");
    const vertices = points.map((point, index) => `${(left + index * slot + slot / 2).toFixed(1)},${scale(point[config.line.key] || 0).toFixed(1)}`).join(" ");
    const dots = points.map((point, index) => `<circle cx="${(left + index * slot + slot / 2).toFixed(1)}" cy="${scale(point[config.line.key] || 0).toFixed(1)}" r="2.1" fill="${config.line.color}" stroke="var(--card)" stroke-width="0.9"></circle>`).join("");
    // The attribution line rides its own stack top, so a hairline casing keeps it off the bars.
    const line = `<polyline points="${vertices}" fill="none" stroke="var(--card)" stroke-width="3" stroke-linejoin="round" stroke-linecap="round"></polyline>
      <polyline points="${vertices}" fill="none" stroke="${config.line.color}" stroke-width="1.8" stroke-linejoin="round" stroke-linecap="round"></polyline>`;

    const legend = config.stacks.concat([config.line]).map(series =>
      `<span><i class="${series === config.line ? "legend-line" : "legend-swatch"}" style="background:${series.color}" aria-hidden="true"></i>${esc(series.label)}</span>`).join("");
    return `<div class="chart" id="${id}" data-chart="${config.name}">
      <svg viewBox="0 0 ${width} ${height}" role="group" aria-label="${esc(config.summary)}">
        <g class="trend-grid">${grid}</g>${columns}${line}${dots}
      </svg>
      <div class="trend-targets">${targets}</div>
      <div class="chart-tip" id="${id}-tip" role="status" hidden></div>
      <div class="chart-legend">${legend}</div>
    </div>`;
  }

  const trendConfigs = { collection: collectionTrendConfig, attribution: attributionTrendConfig };

  // Read back from the response, so the panel shows the numbers the bars were drawn from.
  function showTrendTooltip(slot) {
    const chart = slot.closest(".chart");
    const point = (state.dashboard?.trend || [])[Number(slot.dataset.bucket)];
    const config = trendConfigs[chart.dataset.chart]?.();
    if (!point || !config) return;
    const rows = trendSeries(config).map(series =>
      `<span><i class="${series === config.line ? "legend-line" : "legend-swatch"}" style="background:${series.color}" aria-hidden="true"></i>${esc(series.label)}<b>${point[series.key] || 0}</b></span>`).join("");
    const tip = chart.querySelector(".chart-tip");
    tip.innerHTML = `<strong>${esc(trendSpan(point))}</strong>${rows}`;
    tip.hidden = false;
    // No room above a bar, so park it on the half the pointer is not in.
    const chartBox = chart.getBoundingClientRect();
    const slotBox = slot.getBoundingClientRect();
    const center = slotBox.left - chartBox.left + slotBox.width / 2;
    const start = center < chartBox.width / 2 ? center + slotBox.width : center - slotBox.width - tip.offsetWidth;
    tip.style.insetInlineStart = `${Math.min(Math.max(start, 0), Math.max(chartBox.width - tip.offsetWidth, 0))}px`;
    tip.style.insetBlockStart = "0px";
  }

  function bindTrendTargets() {
    document.querySelectorAll(".trend-bucket").forEach(slot => {
      slot.onfocus = () => showTrendTooltip(slot);
      slot.onblur = () => $(slot).closest(".chart").find(".chart-tip").prop("hidden", true);
      slot.onkeydown = event => {
        if (event.key === "Escape") $(slot).closest(".chart").find(".chart-tip").prop("hidden", true);
      };
    });
  }

  function collectionTrendConfig() {
    return {
      name: "collection", summary: t("collectionTrend"),
      stacks: [
        { key: "critical", label: t("critical"), color: "var(--fill-bad)" },
        { key: "high", label: t("high"), color: "var(--fill-warn)" },
        { key: "medium", label: t("medium"), color: "var(--fill-info)" }
      ],
      line: { key: "total", label: t("total"), color: "var(--acc-text)" }
    };
  }

  function attributionTrendConfig() {
    return {
      name: "attribution", summary: t("attributionTrend"),
      stacks: [
        { key: "named_actor", label: t("actorNamed"), color: "var(--fill-ok)" },
        { key: "qualified_unknown", label: t("actorQualified"), color: "var(--fill-warn)" },
        { key: "unknown_actor", label: t("actorUnknown"), color: "var(--fill-muted)" }
      ],
      line: { key: "attributed", label: t("actorAttributed"), color: "var(--acc-text)" }
    };
  }

  function trendCardsHTML(data) {
    const days = bucketDays(data.trend);
    return `<div class="dashboard-grid">
      <section class="card"><div class="card-header"><div><h2>${esc(t("collectionTrend"))}</h2><p class="card-subtitle">${esc(t("collectionTrendHint")(days))}</p></div></div>${trendChartHTML("collection-trend", data.trend, collectionTrendConfig())}</section>
      <section class="card"><div class="card-header"><div><h2>${esc(t("attributionTrend"))}</h2><p class="card-subtitle">${esc(t("attributionTrendHint")(days))}</p></div></div>${trendChartHTML("attribution-trend", data.trend, attributionTrendConfig())}</section>
    </div>`;
  }

  // Lives in the header instead of the content so the range survives the loading skeleton.
  function renderDashboardRange() {
    $("#dashboard-range")
      .attr("aria-label", t("aggregationRange"))
      .html(dashboardRanges.map(days => `<option value="${days}"${days === state.dashboardDays ? " selected" : ""}>${esc(t(`range${days}`))}</option>`).join(""))
      .prop("hidden", false);
  }

  function dashboardStatsHTML(data) {
    return statsHTML([
      { label: t("total"), value: data.total, tone: "tone-info" }, { label: t("critical"), value: data.critical, tone: "tone-danger" },
      { label: t("high"), value: data.high, tone: "tone-warning" }, { label: t("cve"), value: data.cve_count }
    ], "dashboard-stats");
  }

  function beginDashboardRequest(busySelector) {
    const request = ++dashboardRequest;
    $("#main-content [aria-busy='true']").attr("aria-busy", null);
    if (busySelector) $(busySelector).attr("aria-busy", "true");
    return request;
  }

  // Repaint only what the control changed; scope "actors" is the None filter's one breakdown.
  function refreshDashboard(scope) {
    if (state.view !== "dashboard" || !$("#threat-actor-bars").length) {
      renderDashboard();
      return;
    }
    const busySelector = scope === "actors" ? "#threat-actors-card" : "#main-content .content";
    const request = beginDashboardRequest(busySelector);
    api("GET", dashboardPath()).done(data => {
      // A response for a superseded control press must not repaint over the current one.
      if (request !== dashboardRequest || state.view !== "dashboard") return;
      // The empty state replaces the whole layout, so let the full render own that switch.
      if (data.empty !== Boolean(state.dashboard?.empty)) {
        renderDashboard();
        return;
      }
      state.dashboard = data;
      $("#threat-actor-bars").replaceWith(barsHTML(data.threat_actors, false, "threat-actor-bars"));
      if (scope !== "actors") {
        $("#dashboard-stats").replaceWith(dashboardStatsHTML(data));
        $("#attack-method-bars").replaceWith(barsHTML(data.attack_methods, true, "attack-method-bars"));
        $("#threat-actors-card .card-subtitle").text(t("windowCumulative")(state.dashboardDays));
        // The trends re-bucket with the range; the None filter never reaches them.
        $("#collection-trend").replaceWith(trendChartHTML("collection-trend", data.trend, collectionTrendConfig()));
        $("#attribution-trend").replaceWith(trendChartHTML("attribution-trend", data.trend, attributionTrendConfig()));
        bindTrendTargets();
        $("#collection-trend").closest(".card").find(".card-subtitle").text(t("collectionTrendHint")(bucketDays(data.trend)));
        $("#attribution-trend").closest(".card").find(".card-subtitle").text(t("attributionTrendHint")(bucketDays(data.trend)));
      }
      measureCharts();
    }).fail(error => {
      if (request === dashboardRequest && state.view === "dashboard") showRequestError(error);
    }).always(() => {
      if (request === dashboardRequest) $(busySelector).attr("aria-busy", null);
    });
  }

  function renderDashboard() {
    const restoreScroll = state.view === "cves" ? state.dashboardScroll : 0;
    beginView("dashboard");
    state.currentReport = null;
    clearCVEHash();
    updateNavigation();
    closeDrawer();
    setHeader(t("dashboard"), "");
    renderDashboardRange();
    const request = beginDashboardRequest();
    setLoading();
    api("GET", dashboardPath()).done(data => {
      if (request !== dashboardRequest || state.view !== "dashboard") return;
      state.dashboard = data;
      const empty = data.empty ? `<section class="empty-state"><span class="empty-mark">01</span><h2>${esc(t("noData"))}</h2><p>${esc(t("noDataHint"))}</p></section>` : "";
      const cveRows = (data.cves || [])
        .map(cve => `<tr><td class="mono cve-link" data-label="CVE ID">${esc(cve.id)}</td><td data-label="CVSS">${cvssBadgeHTML(cve.cvss)}</td><td data-label="${esc(t("product"))}">${esc(cve.affected_product)}</td><td class="mono" data-label="${esc(t("firstSeen"))}">${esc(cve.first_seen)}</td><td data-label="${esc(t("mentions"))}">${cve.mentions}</td></tr>`).join("");
      $("#main-content").html(`<div class="content stack">
        ${dashboardStatsHTML(data)}
        ${empty}
        <div class="dashboard-grid">
          <section class="card"><div class="card-header"><div><h2>${esc(t("attackMethods"))}</h2><p class="card-subtitle">${esc(t("articleClassified"))}</p></div></div>${barsHTML(data.attack_methods, true, "attack-method-bars")}</section>
          <section class="card" id="threat-actors-card"><div class="card-header"><div><h2>${esc(t("threatActors"))}</h2><p class="card-subtitle">${esc(t("windowCumulative")(state.dashboardDays))}</p></div>
            <div class="card-switch"><span id="hide-none-actor-label">${esc(t("hideNoneActor"))}</span><button type="button" class="toggle" role="switch" aria-checked="${state.hideNoneActor}" aria-labelledby="hide-none-actor-label" id="hide-none-actor"></button></div></div>${barsHTML(data.threat_actors, false, "threat-actor-bars")}</section>
        </div>
        ${trendCardsHTML(data)}
        <section class="card cve-overview-card"><a class="cve-card-link" id="open-cve-explorer" href="#cves" aria-label="${esc(t("viewAllCVEs"))}" title="${esc(t("viewAllCVEs"))}"></a><div class="card-header"><div><h2>${esc(t("recentCVEs"))}</h2><p class="card-subtitle">${esc(t("cveHint"))}</p></div><span class="icon-button cve-forward" aria-hidden="true"><span>›</span></span></div>
          <div class="table-region" role="region" aria-label="${esc(t("recentCVEs"))}" tabindex="0"><table class="data-table"><thead><tr><th>CVE ID</th><th>CVSS</th><th>${esc(t("product"))}</th><th>${esc(t("firstSeen"))}</th><th>${esc(t("mentions"))}</th></tr></thead><tbody>${cveRows || `<tr><td colspan="5">${esc(t("noData"))}</td></tr>`}</tbody></table></div>
        </section>
      </div>`);
      bindTrendTargets();
      measureCharts();
      // Re-measure after fonts load so fallback metrics do not determine clipping.
      if (document.fonts) document.fonts.ready.then(measureCharts);
      applyViewScroll("dashboard", restoreScroll);
    }).fail(error => {
      if (request === dashboardRequest && state.view === "dashboard") showRequestError(error);
    });
  }

  function renderCVEExplorer() {
    beginView("cves");
    state.currentReport = null;
    updateNavigation();
    closeDrawer();
    setHeader(t("allCVEs"), cveSortHint());

    const sortOptions = [["score", t("riskScore")], ["cvss", "CVSS"], ["mentions", t("mentions")], ["firstSeen", t("firstSeen")]]
      .map(([value, label]) => `<option value="${value}"${value === state.cveSort ? " selected" : ""}>${esc(label)}</option>`).join("");

    const renderRows = cves => {
      $("#main-content").html(`<div class="content stack">
        <section class="card cve-page-summary"><div><span class="badge badge-info">${cves.length} ${esc(t("entries"))}</span><p class="card-subtitle" id="cve-sort-hint">${esc(cveSortHint())}</p></div><div class="cluster"><div class="field cve-sort"><label for="cve-sort">${esc(t("cveSortLabel"))}</label><select id="cve-sort">${sortOptions}</select></div>${isAuthenticated() ? `<button class="secondary-button" id="refresh-cves" type="button">${esc(t("refreshCVEs"))}</button>` : ""}<a class="secondary-button cve-back-link" href="#">${esc(t("backToDashboard"))}</a></div></section>
        <section class="card"><div class="table-region cve-page-table" role="region" aria-label="${esc(t("allCVEs"))}" tabindex="0"><p class="cve-scroll-hint">${esc(t("cveScrollHint"))}</p><table class="data-table"><thead><tr><th>${esc(t("rank"))}</th><th>CVE ID</th><th>CVSS</th><th>${esc(t("mentions"))}</th><th>${esc(t("riskScore"))}</th><th>${esc(t("product"))}</th><th>${esc(t("firstSeen"))}</th></tr></thead><tbody>${cveExplorerRowsHTML(cves)}</tbody></table></div></section>
      </div>`);
      applyViewScroll("cves");
      $("#main-content").trigger("focus");
      refreshCVEControls();
    };

    setLoading();
    loadCVEInsights(renderRows, error => {
      $("#main-content").html(`<div class="content stack"><section class="empty-state">
        <span class="empty-mark">!</span><h2>${esc(t("cveLoadFailed"))}</h2>
        <button class="secondary-button" id="retry-cves" type="button">${esc(t("retry"))}</button>
      </section></div>`);
      showRequestError(error);
    });
  }

  function loadCVEInsights(onComplete, onFailure = showRequestError) {
    const requestID = ++cvesRequest;
    const sort = state.cveSort;
    let restarts = 0;
    const loadRanking = () => {
      const values = [];
      const loadPage = (offset, revision = "") => {
        const continuation = revision ? `&revision=${encodeURIComponent(revision)}` : "";
        request("GET", `/api/cves?sort=${encodeURIComponent(sort)}&offset=${offset}${continuation}`).done((page, _status, response) => {
          if (requestID !== cvesRequest || state.view !== "cves") return;
          const currentRevision = revision || response.getResponseHeader("X-CVE-Revision") || "";
          values.push(...page);
          if (page.length === cvePageSize) loadPage(offset + page.length, currentRevision);
          else onComplete(values);
        }).fail(error => {
          if (requestID !== cvesRequest || state.view !== "cves") return;
          if (error.status === 409 && error.responseJSON?.code === "cve_page_stale" && restarts < 3) {
            restarts++;
            loadRanking();
            return;
          }
          onFailure(error);
        });
      };
      loadPage(0);
    };
    loadRanking();
  }

  function refreshCVEs() {
    if (!requireAdmin(refreshCVEs)) return;
    runCVERefresh();
  }

  function runCVERefresh(existingJob = null) {
    const request = existingJob
      ? cveRefreshTask.resume(serverCVERefresh(existingJob))
      : cveRefreshTask.start();
    if (!request) return;
    request
      .then(result => {
        const dashboardRequestID = beginDashboardRequest();
        return api("GET", dashboardPath()).then(
          data => ({ data, result, dashboardRequestID }),
          error => ({ error, result, dashboardRequestID })
        );
      })
      .done(({ data, error, result, dashboardRequestID }) => {
        if (error) {
          if (dashboardRequestID === dashboardRequest) showRequestError(error);
          return;
        }
        if (dashboardRequestID === dashboardRequest) {
          state.dashboard = data;
          if (state.view === "cves") renderCVEExplorer();
          else if (state.view === "dashboard") renderDashboard();
        }
        const message = t("cveRefreshDone")(result.updated, result.removed);
        toast(result.warnings?.length ? `${message} ${t("cveRefreshWarned")}` : message, Boolean(result.warnings?.length));
      })
      .fail(showRequestError);
  }

  function refreshCVEControls() {
    const $button = $("#refresh-cves");
    if (!$button.length) return;
    const active = cveRefreshTask.isActive();
    $button.prop("disabled", active)
      .attr("aria-busy", active ? "true" : null)
      .text(t(active ? "refreshingCVEs" : "refreshCVEs"));
  }

  function selectDay(day) {
    const date = parseDay(day);
    const { today, oldest } = retentionWindow();
    if (date > today) return toast(t("futureDate"), true);
    const request = beginView("daily");
    state.selectedDay = day;
    state.dailySource = "all";
    renderCalendar();
    setLoading();
    api("GET", `/api/daily/${encodeURIComponent(day)}`).done(data => {
      if (request !== viewRequest) return;
      state.daily = data;
      renderDaily();
      if (isAuthenticated() && date >= oldest && !data.articles.length && !(state.bootstrap.collected_days || []).includes(day)) openCollectModal(day);
    }).fail(error => {
      if (request === viewRequest) showRequestError(error);
    });
  }

  function exportCopy(day = state.selectedDay) {
    return {
      language: state.lang, reportWord: t("reportWord"), dailySummary: t("dailySummary"), generatedBy: t("generatedBy"),
      reportTypes: { weekly: t("weekly"), monthly: t("monthly") }, unknownActor: t("unknownActor"), noSummary: t("noSummary"), none: t("none"),
      day: formatDisplayDate(day),
      labels: {
        total: t("total"), critical: t("critical"), high: t("high"), medium: t("medium"), topThreat: t("topThreat"),
        actors: t("keyActors"), summary: t("summary"), sectors: t("targetSectors"), article: t("article"), articleCount: t("articleCount"),
        date: t("date")
      }
    };
  }

  const DOWNLOAD_ICON = `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v11m0 0 4-4m-4 4-4-4M5 19h14"></path></svg>`;

  function pdfDownloadHTML(view) {
    const label = view === "report" ? t("downloadReportPDF") : t("downloadDailyPDF");
    const emphasisClass = view === "report" ? " report-pdf-button" : "";
    return `<button class="icon-button export-pdf-button${emphasisClass}" id="download-${view}-pdf" type="button" title="${esc(label)}" aria-label="${esc(label)}">${DOWNLOAD_ICON}</button>`;
  }

  function downloadReportPDF() {
    if (!state.currentReport) return;
    if (window.cyberDashboardExport.downloadReportPDF(state.currentReport, exportCopy())) toast(t("pdfStarted"));
    else toast(t("pdfBlocked"), true);
  }

  function downloadDailyPDF() {
    if (!state.daily) return;
    if (window.cyberDashboardExport.downloadDailyPDF(state.daily, exportCopy(state.daily.day || state.selectedDay))) toast(t("pdfStarted"));
    else toast(t("pdfBlocked"), true);
  }

  function dailyArticlesHTML(daily) {
    const visible = state.dailySource === "all" ? daily.articles : daily.articles.filter(article => article.source === state.dailySource);
    const articles = visible.map(article => `<a class="article-row" href="${esc(safeURL(article.url))}" target="_blank" rel="noopener noreferrer">
      <div class="cluster"><span class="badge badge-info">${esc(article.source)}</span><span class="badge">${esc(article.attack_method)}</span><span class="badge ${toneClass(article.severity)}">${esc(article.severity)}</span></div>
      <h3>${esc(article.title)}</h3><p>${esc(article.summary || t("noArticles"))}</p>
      <div class="article-meta"><span>${esc(article.threat_actor)}${article.actor_country ? ` (${esc(article.actor_country)})` : ""}</span><span class="mono">${esc(formatArticleTime(article.published_at))}</span></div>
    </a>`).join("");
    return `<section class="article-list">${articles || `<div class="empty-state"><span class="empty-mark">00</span><h2>${esc(t("noArticles"))}</h2></div>`}</section>`;
  }

  function renderDaily() {
    beginView("daily");
    state.currentReport = null;
    clearCVEHash();
    updateNavigation();
    renderReportList();
    const daily = state.daily || { articles: [], summary: "" };
    setHeader(formatDisplayDate(state.selectedDay), t("feedSubtitle"));
    const sources = [...new Set(daily.articles.map(article => article.source))].sort((left, right) => left.localeCompare(right));
    if (state.dailySource !== "all" && !sources.includes(state.dailySource)) state.dailySource = "all";
    const dailySummary = daily.summary ? `<section class="card brief-card"><div class="card-header"><div><h2>${esc(t("dailySummary"))}</h2><p class="card-subtitle">${esc(t("aiGenerated"))}</p></div><div class="daily-summary-actions"><span class="badge badge-info">${daily.articles.length} ${esc(articleCountLabel(daily.articles.length))}</span>${pdfDownloadHTML("daily")}</div></div><div class="brief-body">${esc(daily.summary)}</div></section>` : "";
    const dailyPDFAction = daily.summary ? "" : pdfDownloadHTML("daily");
    const collectionActions = isAuthenticated() ? `<div class="cluster collection-actions"><button class="secondary-button" id="recollect-day" type="button">${esc(t("recollect"))}</button><button class="secondary-button cancel-collection" id="cancel-active-collection" type="button" hidden>${esc(t("cancel"))}</button></div>` : "";
    $("#main-content").html(`<div class="content stack">
      ${dailySummary}
      <div class="daily-toolbar"><div class="field daily-filter"><label for="source-filter">${esc(t("sourceFilter"))}</label><select id="source-filter"><option value="all">${esc(t("allSources"))}</option>${sources.map(source => `<option value="${esc(source)}"${source === state.dailySource ? " selected" : ""}>${esc(source)}</option>`).join("")}</select></div><div class="cluster daily-actions">${dailyPDFAction}${collectionActions}</div></div>
      ${dailyArticlesHTML(daily)}
    </div>`);
    applyViewScroll(`daily:${state.selectedDay}`);
    refreshCollectionControls();
    closeDrawer();
  }

  function openCollectModal(day, recollect = false) {
    if (!requireAdmin(() => openCollectModal(day, recollect))) return;
    const active = state.bootstrap.sources.filter(source => source.enabled).length;
    openModal(`<div class="modal modal-small" role="dialog" aria-modal="true" aria-labelledby="collect-title" data-collect-day="${day}">
      <div class="modal-header"><div><h2 id="collect-title">${esc(t(recollect ? "recollectConfirm" : "collectNow"))}</h2><p>${esc(formatDisplayDate(day))}</p></div><button class="icon-button modal-dismiss" type="button" aria-label="${esc(t("close"))}">×</button></div>
      <div class="modal-body"><p class="prose">${active}${esc(t("sourcesActive"))}</p><p class="collection-background-hint" hidden>${esc(t("collectingHint"))}</p></div>
      <div class="modal-footer"><button class="secondary-button modal-close" data-collection-secondary type="button">${esc(t("cancel"))}</button><button class="primary-button" id="confirm-collect" data-day="${day}" data-idle-label="${esc(t(recollect ? "recollect" : "start"))}" type="button">${esc(t(recollect ? "recollect" : "start"))}</button></div>
    </div>`);
    refreshCollectionControls();
  }

  function runCollection(day, existingJob = null) {
    const request = existingJob ? collectionTask.resume(day, serverCollection(day, existingJob)) : collectionTask.start(day);
    if (!request) return;
    request.done(result => {
      if ($(`[data-collect-day="${day}"]`).length) closeModal();
      if (!state.bootstrap.collected_days.includes(day)) state.bootstrap.collected_days.push(day);
      toast(`${t("collected")} ${result.collected} ${articleCountLabel(result.collected)}`);
      (result.warnings || []).forEach(warning => toast(warning, true));
      renderCalendar();
      if (state.view === "daily" && state.selectedDay === day) selectDay(day);
    }).fail((error, status) => {
      if (status === "abort") {
        if ($(`[data-collect-day="${day}"]`).length) closeModal();
        toast(t("collectionCancelled"));
      } else showRequestError(error);
    });
  }

  function refreshCollectionControls() {
    const activeDay = collectionTask.activeDay();
    const active = activeDay !== null;
    const { today, oldest } = retentionWindow();
    const selectedDate = state.selectedDay ? parseDay(state.selectedDay) : today;
    const collectionAvailable = selectedDate >= oldest && selectedDate <= today;
    $("#recollect-day").prop("disabled", active || !collectionAvailable).attr("aria-busy", String(active)).attr("title", collectionAvailable ? null : t("outsideRetention")).text(active ? t("collecting") : t("recollect"));
    $("#cancel-active-collection").prop("hidden", !active);
    const modalDay = $("[data-collect-day]").data("collect-day");
    const modalActive = active && modalDay === activeDay;
    const $confirm = $("#confirm-collect");
    $confirm.prop("disabled", active).attr("aria-busy", String(modalActive)).text(modalActive ? t("collecting") : $confirm.data("idle-label"));
    $(".collection-background-hint").prop("hidden", !modalActive);
    $("[data-collection-secondary]").toggleClass("modal-close", !modalActive).toggleClass("cancel-collection", modalActive);
  }

  function renderSettings() {
    if (!requireAdmin(renderSettings)) return;
    // Arriving from another view discards a source draft the user never saved.
    if (state.view !== "settings") state.sourceDraft = null;
    const passwords = state.view === "settings" ? passwordFormValue() : null;
    beginView("settings");
    state.currentReport = null;
    clearCVEHash();
    updateNavigation();
    setHeader(t("settings"), t("settingsSub"));
    const settings = state.bootstrap.settings;
    const presets = state.bootstrap.llm_presets || [];
    const languageOptions = [["ko", "한국어"], ["en", "English"]].map(([code, label]) => `<option value="${code}"${code === settings.language ? " selected" : ""}>${label}</option>`).join("");
    const draftSources = sourceDraft();
    const sources = state.bootstrap.sources.map(source => {
      const enabled = draftSources[source.id];
      return `<div class="source-row" data-source-row="${source.id}"><span class="status-dot${enabled ? " is-on" : ""}" aria-hidden="true"></span><span><strong>${esc(source.name)}</strong><small>${esc(source.host)} · RSS feed</small></span><button type="button" class="toggle" role="switch" aria-label="${esc(source.name)}" aria-checked="${enabled}" data-source-id="${source.id}"></button><span class="badge ${enabled ? "badge-success" : ""}">${esc(t(enabled ? "sourceOn" : "sourceOff"))}</span></div>`;
    }).join("");
    const preview = `POST ${(settings.llm_base_url || "<base-url>").replace(/\/$/, "")}/chat/completions\nAuthorization: Bearer ${llmKeyIsConfigured(settings) ? "••••••••" : "<api-key>"}\n\n{ "model": "${settings.llm_model || "<model>"}",\n  "temperature": 0.2,\n  "messages": [ … ] }`;
    const schema = state.lang === "ko"
      ? `{ "attack_method": "APT | 랜섬웨어 | 공급망 | …",\n  "threat_actor": "Lazarus Group | SideCopy | TeamPCP | 미확인",\n  "actor_country": "DPRK | Pakistan | null",\n  "target_sector": "금융 | 정부 | 통신 | …",\n  "severity": "Critical | High | Medium",\n  "cve": ["CVE-YYYY-NNNN", …]   // 정규식 추출, LLM 응답 아님\n}`
      : `{ "attack_method": "APT | Ransomware | Supply chain | …",\n  "threat_actor": "Lazarus Group | SideCopy | TeamPCP | Unknown",\n  "actor_country": "DPRK | Pakistan | null",\n  "target_sector": "Finance | Government | Telecommunications | …",\n  "severity": "Critical | High | Medium",\n  "cve": ["CVE-YYYY-NNNN", …]   // Extracted with regex, not returned by the LLM\n}`;
    const presetItems = presets.map(preset => {
      const active = matchesPreset(preset, settings.llm_base_url, settings.llm_model);
      return `<span class="preset-item"><button class="preset-chip${active ? " is-active" : ""}" type="button" data-preset-id="${preset.id}" aria-pressed="${active}" title="${esc(preset.base_url)} · ${esc(preset.model)}"><span class="preset-dot" aria-hidden="true"></span><span><strong>${esc(preset.label)}</strong><small>${esc(preset.model)}${preset.api_key_configured ? ` · ${esc(t("keySaved"))}` : ""}</small></span></button>${preset.builtin ? "" : `<button class="preset-remove" type="button" data-remove-preset-id="${preset.id}" aria-label="${esc(t("presetRemove"))}: ${esc(preset.label)}" title="${esc(t("presetRemove"))}">×</button>`}</span>`;
    }).join("");
    const addDisabled = !canAddPreset(settings.llm_base_url, settings.llm_model);
    const securityCard = state.bootstrap.auth && state.bootstrap.auth.enabled ? `<section class="card"><div class="card-header"><div><h2>${esc(t("securityTitle"))}</h2><p class="card-subtitle">${esc(t("securityHint"))}</p></div></div><form class="field-grid password-change-grid" id="password-form">${adminUsernameField()}<div class="field"><label for="current-password">${esc(t("currentPassword"))}</label><input id="current-password" type="password" autocomplete="current-password"><small class="field-message" role="alert" hidden></small></div><div class="field"><label for="new-password">${esc(t("newPassword"))}</label><input id="new-password" type="password" autocomplete="new-password"><small>${esc(t("passwordRule"))}</small></div><div class="field"><label for="confirm-password">${esc(t("confirmPassword"))}</label><input id="confirm-password" type="password" autocomplete="new-password"><small class="field-message" role="alert" hidden></small></div><div class="full-span cluster"><button class="secondary-button" id="change-password" type="submit">${esc(t("changePassword"))}</button></div></form></section>` : "";
    $("#main-content").html(`<div class="content settings-stack">
      ${securityCard}
      <section class="card"><div class="card-header"><div><h2>${esc(t("language"))}</h2><p class="card-subtitle">${esc(t("languageHint"))}</p></div></div><div class="field"><label for="setting-language">${esc(t("language"))}</label><select id="setting-language">${languageOptions}</select></div></section>
      <section class="card"><div class="card-header"><h2>${esc(t("sourceSettings"))}</h2></div><div class="source-list">${sources}</div></section>
      <section class="card"><div class="card-header"><div><h2>${esc(t("nvdTitle"))}</h2><p class="card-subtitle">${esc(t("nvdHint"))}</p></div><a href="https://nvd.nist.gov/developers/request-an-api-key" target="_blank" rel="noopener noreferrer">NVD ↗</a></div><div class="field"><label for="nvd-api-key">${esc(t("apiKey"))}</label><input id="nvd-api-key" type="password" autocomplete="off" value="" placeholder="${settings.nvd_api_key_configured ? esc(t("keyStoredPlaceholder")) : ""}"><small>${esc(t("keyInputHint"))} · <span class="rate-limit">50 requests / 30s</span></small></div></section>
      <section class="card"><div class="card-header"><div><h2>${esc(t("timezone"))}</h2><p class="card-subtitle">${esc(t("timezoneHint"))}</p></div></div><div class="field"><label for="timezone-offset">UTC offset</label><select id="timezone-offset">${timezoneOptions(Number(settings.timezone_offset_minutes) || 0)}</select></div></section>
      <section class="card"><div class="card-header"><div><h2>${esc(t("llmTitle"))}</h2><p class="card-subtitle">${esc(t("llmHint"))}</p></div><span class="badge badge-info">OpenAI compatible</span></div>
        <div class="field-grid"><div class="preset-field full-span"><div class="preset-head"><span>${esc(t("preset"))}</span><button class="preset-add" id="add-llm-preset" type="button" title="${esc(t("presetAddHint"))}"${addDisabled ? " disabled" : ""}><span aria-hidden="true">＋</span>${esc(t("presetAdd"))}</button></div><div class="preset-list" id="llm-preset-list">${presetItems}</div></div>
          <div class="field"><label for="llm-base-url">${esc(t("baseURL"))}</label><input id="llm-base-url" type="url" value="${esc(settings.llm_base_url || "")}"></div><div class="field"><label for="llm-model">${esc(t("model"))}</label><input id="llm-model" value="${esc(settings.llm_model || "")}"></div><div class="field"><label for="llm-api-key">${esc(t("apiKey"))}</label><input id="llm-api-key" type="password" autocomplete="off" value="" placeholder="${llmKeyIsConfigured(settings) ? esc(t("keyStoredPlaceholder")) : ""}"><small>${esc(t("keyInputHint"))}</small></div><div class="field"><label for="llm-timeout">${esc(t("timeout"))}</label><input id="llm-timeout" type="number" min="1" max="600" value="${Number(settings.llm_timeout) || 120}"></div>
          <div class="full-span cluster"><button class="secondary-button" id="test-llm" type="button">${esc(t("test"))}</button></div>
          <div class="field full-span"><label>${esc(t("requestPreview"))}</label><pre class="request-preview" id="request-preview">${esc(preview)}</pre></div></div>
      </section>
      <section class="card"><div class="card-header"><h2>${esc(t("schema"))}</h2></div><pre class="schema-preview">${esc(schema)}</pre></section>
      <section class="card settings-legal-card"><div><h2>${esc(t("licenseTitle"))}</h2><p class="card-subtitle">${esc(t("licenseHint"))}</p></div><button class="secondary-button" id="open-license-modal" type="button">${esc(t("viewLicenses"))}</button></section>
    </div><div class="settings-save-bar" id="settings-save-bar" role="status" aria-live="polite" hidden><strong>${esc(t("unsavedSettings"))}</strong><div class="cluster"><button class="secondary-button" id="revert-settings" type="button">${esc(t("revert"))}</button><button class="primary-button" id="save-settings" type="button">${esc(t("save"))}</button></div></div>`);
    if (passwords) restorePasswordForm(passwords);
    applyViewScroll("settings");
    refreshSettingsFormState();
    closeDrawer();
  }

  // Saving or reverting settings re-renders the page, but the password form is a separate submit
  // path, so a half-typed change has to survive the round trip.
  const passwordFieldIDs = ["#current-password", "#new-password", "#confirm-password"];

  function passwordFormValue() {
    return passwordFieldIDs.map(selector => $(selector).val() || "");
  }

  function restorePasswordForm(values) {
    passwordFieldIDs.forEach((selector, index) => { if (values[index]) $(selector).val(values[index]); });
  }

  function sourceDraft() {
    if (!state.sourceDraft) state.sourceDraft = Object.fromEntries(state.bootstrap.sources.map(source => [source.id, source.enabled]));
    return state.sourceDraft;
  }

  function sameSources() {
    const draft = sourceDraft();
    return state.bootstrap.sources.every(source => draft[source.id] === source.enabled);
  }

  function paintSourceRow(id, enabled) {
    const $row = $(`[data-source-row="${id}"]`);
    $row.find(".status-dot").toggleClass("is-on", enabled);
    $row.find(".toggle").attr("aria-checked", String(enabled));
    $row.find(".badge").toggleClass("badge-success", enabled).text(t(enabled ? "sourceOn" : "sourceOff"));
  }

  function settingsFormValue() {
    return {
      language: $("#setting-language").val() || state.lang, accent: state.bootstrap.settings.accent || "#4f6ef7",
      llm_base_url: $("#llm-base-url").val().trim(), llm_model: $("#llm-model").val().trim(),
      llm_api_key: $("#llm-api-key").val(), llm_timeout: Number($("#llm-timeout").val()), nvd_api_key: $("#nvd-api-key").val(),
      timezone_offset_minutes: Number($("#timezone-offset").val())
    };
  }

  function applyNonSecretSettingsDraft(settings) {
    $("#setting-language").val(settings.language);
    $("#llm-base-url").val(settings.llm_base_url);
    $("#llm-model").val(settings.llm_model);
    $("#llm-api-key").val("");
    $("#llm-timeout").val(settings.llm_timeout);
    $("#nvd-api-key").val("");
    $("#timezone-offset").val(settings.timezone_offset_minutes);
    refreshSettingsFormState();
  }

  function sameSettings(left, right) {
    return left.language === right.language && left.accent === right.accent &&
      left.llm_base_url === right.llm_base_url && left.llm_model === right.llm_model && left.llm_api_key === (right.llm_api_key || "") &&
      left.llm_timeout === Number(right.llm_timeout) && left.nvd_api_key === (right.nvd_api_key || "") &&
      left.timezone_offset_minutes === Number(right.timezone_offset_minutes);
  }

  function refreshSettingsFormState() {
    if (!$("#llm-base-url").length) return;
    refreshPresetControls();
    const settings = settingsFormValue();
    const configured = llmKeyIsConfigured(settings);
    const preview = `POST ${(settings.llm_base_url || "<base-url>").replace(/\/$/, "")}/chat/completions\nAuthorization: Bearer ${configured ? "••••••••" : "<api-key>"}\n\n{ "model": "${settings.llm_model || "<model>"}",\n  "temperature": 0.2,\n  "messages": [ … ] }`;
    $("#request-preview").text(preview);
    $("#llm-api-key").attr("placeholder", configured ? t("keyStoredPlaceholder") : "");
    $("#settings-save-bar").prop("hidden", sameSettings(settings, state.bootstrap.settings) && sameSources());
  }

  function revertSettings() {
    const settings = state.bootstrap.settings;
    state.sourceDraft = null;
    adoptLanguage(settings.language);
    applyChrome();
    renderSettings();
  }

  function refreshPresetControls() {
    const baseURL = $("#llm-base-url").val();
    const model = $("#llm-model").val();
    $("#add-llm-preset").prop("disabled", !canAddPreset(baseURL, model));
    $(".preset-chip").each(function () {
      const preset = (state.bootstrap.llm_presets || []).find(item => item.id === Number($(this).data("preset-id")));
      const active = Boolean(preset && matchesPreset(preset, baseURL, model));
      $(this).toggleClass("is-active", active).attr("aria-pressed", String(active));
    });
  }

  function selectPreset(id) {
    const preset = (state.bootstrap.llm_presets || []).find(item => item.id === id);
    if (!preset) return;
    $("#llm-base-url").val(preset.base_url);
    $("#llm-model").val(preset.model);
    $("#llm-api-key").val("");
    refreshSettingsFormState();
  }

  function addCurrentPreset() {
    const draft = settingsFormValue();
    const request = { base_url: draft.llm_base_url, model: draft.llm_model, api_key: draft.llm_api_key };
    if (!canAddPreset(request.base_url, request.model)) return;
    $("#add-llm-preset").prop("disabled", true);
    api("POST", "/api/llm/presets", request).done(preset => {
      state.bootstrap.llm_presets.push(preset);
      toast(t("presetAdded"));
      renderSettings();
      applyNonSecretSettingsDraft(draft);
    }).fail(error => { refreshPresetControls(); showRequestError(error); });
  }

  function removePreset(id) {
    const draft = settingsFormValue();
    api("DELETE", `/api/llm/presets/${id}`).done(() => {
      state.bootstrap.llm_presets = state.bootstrap.llm_presets.filter(preset => preset.id !== id);
      toast(t("presetRemoved"));
      renderSettings();
      applyNonSecretSettingsDraft(draft);
    }).fail(showRequestError);
  }

  function changedSourceStates() {
    const draft = sourceDraft();
    return state.bootstrap.sources.filter(source => draft[source.id] !== source.enabled)
      .map(source => ({ id: source.id, enabled: draft[source.id] }));
  }

  function saveSettings() {
    const settings = settingsFormValue();
    const sources = changedSourceStates();
    const preset = (state.bootstrap.llm_presets || []).find(item => matchesPreset(item, settings.llm_base_url, settings.llm_model));
    $("#save-settings,#revert-settings").prop("disabled", true);
    api("PUT", "/api/settings", { ...settings, sources }).then(saved => {
      state.bootstrap.settings = saved;
      sources.forEach(changed => {
        const source = state.bootstrap.sources.find(item => item.id === changed.id);
        if (source) source.enabled = changed.enabled;
      });
      if (!preset || !String(settings.llm_api_key || "").trim()) return saved;
      return api("PUT", `/api/llm/presets/${preset.id}`, { api_key: settings.llm_api_key }).then(() => {
        preset.api_key_configured = true;
        return saved;
      });
    }).done(() => {
      state.sourceDraft = null;
      adoptLanguage(state.bootstrap.settings.language);
      applyChrome();
      toast(t("saved"));
      renderSettings();
    }).fail(error => {
      $("#save-settings,#revert-settings").prop("disabled", false);
      showRequestError(error);
    });
  }

  function testConnection() {
    const $button = $("#test-llm").prop("disabled", true).text(t("testing"));
    api("POST", "/api/llm/test", settingsFormValue()).done(() => toast(t("connectionOK"))).fail(() => toast(t("connectionFail"), true)).always(() => $button.prop("disabled", false).text(t("test")));
  }

  function changePassword() {
    clearFieldErrors("#password-form");
    const currentPassword = $("#current-password").val();
    const newPassword = $("#new-password").val();
    // The rule is checked before the match so a password that is too short says so on the first
    // attempt, instead of after the user has fixed a mismatch it was never going to accept.
    const byteLength = new TextEncoder().encode(newPassword).length;
    if (byteLength < 12 || byteLength > 128) return showFieldError("#new-password", null);
    if (newPassword !== $("#confirm-password").val()) return showFieldError("#confirm-password", t("passwordMismatch"));
    const $button = $("#change-password").prop("disabled", true).text(t("changingPassword"));
    api("PUT", "/api/auth/password", { current_password: currentPassword, new_password: newPassword }).done(() => {
      $(passwordFieldIDs.join(",")).val("");
      toast(t("passwordChanged"));
    }).fail(error => {
      const code = error.responseJSON && error.responseJSON.code;
      if (code === "invalid_credentials") showFieldError("#current-password", errorMessage(error));
      else if (code === "weak_password") showFieldError("#new-password", null);
      else showRequestError(error);
    }).always(() => $button.prop("disabled", false).text(t("changePassword")));
  }

  function openReportModal() {
    if (!requireAdmin(openReportModal)) return;
    state.reportType = "weekly";
    closeDrawer();
    const today = configuredToday();
    state.reportYear = today.getFullYear();
    state.reportMonth = today.getMonth();
    state.reportWeekStart = null;
    openModal(`<div class="modal report-composer" role="dialog" aria-modal="true" aria-labelledby="report-modal-title">
      <div class="modal-header"><div><h2 id="report-modal-title">${esc(t("reportTitle"))}</h2><p>${esc(t("reportHint"))}</p></div></div>
      <div class="modal-body"><div class="segmented report-type-tabs" role="group" aria-label="${esc(t("reportTitle"))}"><button type="button" data-report-type="weekly" class="is-selected" aria-pressed="true">${esc(t("weekly"))}</button><button type="button" data-report-type="monthly" aria-pressed="false">${esc(t("monthly"))}</button></div><div id="report-period-picker"></div></div>
      <div class="modal-footer"><button class="secondary-button modal-close" type="button">${esc(t("cancel"))}</button><button class="primary-button" id="generate-report" type="button" disabled>${esc(t("generate"))}</button></div>
    </div>`);
    renderReportPeriodPicker();
  }

  function reportWeeks(year, month) {
    const first = new Date(year, month, 1);
    const last = new Date(year, month + 1, 0);
    const cursor = new Date(first);
    cursor.setDate(cursor.getDate() - cursor.getDay());
    const weeks = [];
    while (cursor <= last) {
      const start = new Date(cursor);
      const end = new Date(start); end.setDate(start.getDate() + 6);
      weeks.push({ start, end });
      cursor.setDate(cursor.getDate() + 7);
    }
    return weeks;
  }

  function renderReportPeriodPicker() {
    const today = configuredToday();
    const years = [today.getFullYear() - 1, today.getFullYear()];
    const yearOptions = years.map(year => `<option value="${year}"${year === state.reportYear ? " selected" : ""}>${year}${state.lang === "ko" ? "년" : ""}</option>`).join("");
    const monthOptions = Array.from({ length: 12 }, (_, month) => {
      const future = state.reportYear === today.getFullYear() && month > today.getMonth();
      const label = state.lang === "ko" ? `${month + 1}월` : new Intl.DateTimeFormat("en-US", { month: "long" }).format(new Date(2026, month, 1));
      return `<option value="${month}"${month === state.reportMonth ? " selected" : ""}${future ? " disabled" : ""}>${label}</option>`;
    }).join("");
    const rows = reportWeeks(state.reportYear, state.reportMonth).map((week, index) => {
      const disabled = week.start > today;
      const key = formatDay(week.start);
      const endKey = formatDay(week.end);
      const count = (state.bootstrap.collected_days || []).filter(day => day >= key && day <= endKey).length;
      const selected = state.reportWeekStart === key;
      const unit = count === 1 ? t("articleUnit") : t("articlesUnit");
      return `<button class="report-week${selected ? " is-selected" : ""}" type="button" data-report-week="${key}"${disabled ? " disabled" : ""}><strong>W${index + 1}</strong><span>${esc(key)} – ${esc(endKey)}</span><small>${count} ${esc(unit)}</small></button>`;
    }).join("");
    $("#report-period-picker").html(`<div class="report-selects"><div class="field"><label for="report-year">${esc(t("pickYear"))}</label><select id="report-year">${yearOptions}</select></div><div class="field"><label for="report-month">${esc(t("pickMonth"))}</label><select id="report-month">${monthOptions}</select></div></div>${state.reportType === "weekly" ? `<div class="report-week-head"><span>${esc(t("pickWeek"))}</span><small>${esc(t("scrollHint"))}</small></div><div class="report-week-list">${rows}</div>` : `<div class="report-month-summary"><strong>${state.reportYear}.${String(state.reportMonth + 1).padStart(2, "0")}</strong><span>${esc(t("reportHint"))}</span></div>`}`);
    $("#generate-report").prop("disabled", state.reportType === "weekly" && !state.reportWeekStart);
  }

  function generateReport() {
    const $button = $("#generate-report").prop("disabled", true);
    let start;
    let end;
    if (state.reportType === "weekly") {
      start = parseDay(state.reportWeekStart);
      end = new Date(start); end.setDate(start.getDate() + 6);
    } else {
      start = new Date(state.reportYear, state.reportMonth, 1);
      end = new Date(state.reportYear, state.reportMonth + 1, 0);
    }
    const today = configuredToday();
    if (end > today) end = today;
    api("POST", "/api/reports", { type: state.reportType, period_start: formatDay(start), period_end: formatDay(end) })
      .done(report => {
        state.bootstrap.reports.unshift({ id: report.id, type: report.type, period_start: report.period_start, period_end: report.period_end });
        closeModal();
        renderReport(report);
        renderReportList();
      })
      .fail(error => { $button.prop("disabled", false); if (error.status === 404) toast(t("emptyReport"), true); else showRequestError(error); });
  }

  function renderReport(report) {
    beginView("report");
    state.currentReport = report;
    clearCVEHash();
    updateNavigation();
    renderReportList();
    setHeader(report.type === "weekly" ? t("weekly") : t("monthly"), `${report.period_start} – ${report.period_end}`);
    const actors = report.actors.map(actor => esc(actor)).join(" · ") || esc(t("unknownActor"));
    const threats = reportThreats(report);
    const threatList = threats.length ? `<ol class="report-threat-list">${threats.map(threat => `<li>${esc(threat.title)}</li>`).join("")}</ol>` : `<p>${esc(t("none"))}</p>`;
    const deleteAction = isAuthenticated() ? `<button class="danger-button" id="delete-report" type="button">${esc(t("deleteReport"))}</button>` : "";
    $("#main-content").html(`<div class="content"><article class="report-sheet">
      <header class="report-sheet-header"><div><h2>${esc(report.type === "weekly" ? t("weekly") : t("monthly"))}</h2><p>${esc(report.period_start)} – ${esc(report.period_end)}</p></div><div class="cluster report-sheet-actions">${pdfDownloadHTML("report")}${deleteAction}</div></header>
      <div class="report-metrics"><div><strong class="tone-info">${report.total}</strong><span>${esc(t("total"))}</span></div><div><strong class="tone-danger">${report.critical}</strong><span>${esc(t("critical"))}</span></div><div><strong class="tone-warning">${report.high}</strong><span>${esc(t("high"))}</span></div><div><strong>${report.medium}</strong><span>${esc(t("medium"))}</span></div></div>
      <section class="report-section"><h3>${esc(t("topThreat"))}</h3>${threatList}</section>
      <section class="report-section"><h3>${esc(t("keyActors"))}</h3><p>${actors}</p></section>
      <section class="report-section"><h3>${esc(t("summary"))}</h3><p class="prose">${esc(report.summary)}</p></section>
      <section class="report-section"><h3>${esc(t("targetSectors"))}</h3><div class="report-sectors">${report.sectors.map(sector => `<span>${esc(sector)}</span>`).join("")}</div></section>
    </article></div>`);
    applyViewScroll(`report:${report.id}`);
    closeDrawer();
  }

  function openDeleteReportModal(report) {
    if (!requireAdmin(() => openDeleteReportModal(report))) return;
    openModal(`<div class="modal modal-small" role="dialog" aria-modal="true" aria-labelledby="delete-report-title" aria-describedby="delete-report-description">
      <div class="modal-header"><div><h2 id="delete-report-title">${esc(t("deleteReportTitle"))}</h2><p>${esc(report.period_start)} – ${esc(report.period_end)}</p></div></div>
      <div class="modal-body"><p class="prose" id="delete-report-description">${esc(t("deleteReportConfirm"))}</p><p class="card-subtitle">${esc(t("deleteReportHint"))}</p></div>
      <div class="modal-footer"><button class="secondary-button modal-close" type="button">${esc(t("cancel"))}</button><button class="danger-button" id="confirm-delete-report" data-delete-report-id="${report.id}" type="button">${esc(t("deleteReport"))}</button></div>
    </div>`);
  }

  function deleteReport(id) {
    const $button = $("#confirm-delete-report").prop("disabled", true).attr("aria-busy", "true").text(t("deleting"));
    api("DELETE", `/api/reports/${encodeURIComponent(id)}`).done(() => {
      state.bootstrap.reports = state.bootstrap.reports.filter(report => report.id !== id);
      state.currentReport = null;
      renderReportList();
      // Do not restore focus to the report control that this deletion detaches.
      modalLastFocus = null;
      closeModal();
      toast(t("reportDeleted"));
      renderDashboard();
      $("#main-content").trigger("focus");
    }).fail(error => {
      $button.prop("disabled", false).removeAttr("aria-busy").text(t("deleteReport"));
      showRequestError(error);
    });
  }

  function openLicenseModal() {
    openModal(`<div class="modal license-modal" role="dialog" aria-modal="true" aria-labelledby="license-dialog-title" aria-describedby="license-dialog-description">
      <div class="modal-header"><div><h2 id="license-dialog-title">${esc(t("licenseDialogTitle"))}</h2><p id="license-dialog-description">${esc(t("licenseDialogHint"))}</p></div><button class="icon-button modal-dismiss" type="button" aria-label="${esc(t("close"))}">×</button></div>
      <div class="modal-body"><div class="segmented license-tabs" role="tablist" aria-label="${esc(t("licenseTitle"))}"><button id="license-tab-program" class="is-selected" type="button" role="tab" aria-selected="true" aria-controls="license-document" data-license-tab="program">${esc(t("programLicense"))}</button><button id="license-tab-third-party" type="button" role="tab" aria-selected="false" aria-controls="license-document" tabindex="-1" data-license-tab="thirdParty">${esc(t("thirdPartyLicenses"))}</button></div><pre class="license-document" id="license-document" role="tabpanel" aria-labelledby="license-tab-program" aria-live="polite" tabindex="0"></pre></div>
      <div class="modal-footer"><button class="secondary-button modal-close" type="button">${esc(t("close"))}</button></div>
    </div>`);
    selectLicenseTab("program");
  }

  function selectLicenseTab(requestedTab) {
    const tab = Object.prototype.hasOwnProperty.call(licenseDocuments, requestedTab) ? requestedTab : "program";
    const $tabs = $("[data-license-tab]");
    $tabs.each(function () {
      const selected = $(this).data("license-tab") === tab;
      $(this).toggleClass("is-selected", selected).attr("aria-selected", String(selected)).attr("tabindex", selected ? "0" : "-1");
    });
    const $panel = $("#license-document").attr("aria-labelledby", `license-tab-${tab === "thirdParty" ? "third-party" : "program"}`);
    if (Object.prototype.hasOwnProperty.call(licenseDocumentCache, tab)) {
      $panel.removeAttr("aria-busy").text(licenseDocumentCache[tab]);
      return;
    }
    $panel.attr("aria-busy", "true").text(t("loadingLicenses"));
    $.ajax({ url: licenseDocuments[tab], dataType: "text", cache: true }).done(content => {
      licenseDocumentCache[tab] = content;
      if ($("[data-license-tab].is-selected").data("license-tab") === tab) $panel.removeAttr("aria-busy").text(content);
    }).fail(() => {
      if ($("[data-license-tab].is-selected").data("license-tab") === tab) $panel.removeAttr("aria-busy").text(t("licenseLoadFailed"));
    });
  }

  // Focus defaults to the first control, which is the header dismiss button; forms that open for
  // the sake of one input name it instead.
  function openModal(content, focus) {
    modalLastFocus = document.activeElement;
    $("#modal-root").html(`<div class="modal-backdrop">${content}</div>`);
    const $focus = focus ? $("#modal-root").find(focus) : $();
    ($focus.length ? $focus : $("#modal-root").find("button, input, select")).first().trigger("focus");
  }

  function closeModal() {
    if (!$("#modal-root").children().length) return;
    // Dismissing the login modal abandons whatever was waiting on it.
    const abandoned = $("#login-form").length ? pendingLogin : null;
    if (abandoned) pendingLogin = null;
    $("#modal-root").empty();
    if (modalLastFocus && document.contains(modalLastFocus)) modalLastFocus.focus();
    modalLastFocus = null;
    if (!abandoned) return;
    if (abandoned.cancel) abandoned.cancel();
    // Only an expired session leaves admin chrome on screen that no longer matches the session.
    if (abandoned.expired) {
      if (state.view === "settings") renderDashboard();
      else routeCurrentView();
    }
  }

  function trapModalFocus(event) {
    if (event.key !== "Tab") return;
    const focusable = $("#modal-root").find("button:not(:disabled), input:not(:disabled):not([hidden]), select:not(:disabled), [href], [tabindex]:not([tabindex='-1'])").get();
    if (!focusable.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  function trapDrawerFocus(event) {
    if (event.key !== "Tab" || !$(".app-shell").hasClass("is-drawer-open")) return;
    const focusable = $("#sidebar").find("button:not(:disabled), [href], [tabindex]:not([tabindex='-1'])").get();
    if (!focusable.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  function setDrawer(open) {
    const compact = window.matchMedia("(max-width: 900px)").matches;
    const expanded = compact && open;
    const wasExpanded = $(".app-shell").hasClass("is-drawer-open");
    $(".app-shell").toggleClass("is-drawer-open", expanded);
    $("#menu-button").attr("aria-expanded", String(expanded));
    $("#sidebar").prop("inert", compact && !expanded);
    $(".main-column").prop("inert", expanded);
    if (expanded) $("#drawer-close").trigger("focus");
    else if (wasExpanded) $("#menu-button").trigger("focus");
  }

  function closeDrawer() { setDrawer(false); }

  function updateNavigation() {
    $("[data-view]").removeClass("is-active");
    $("[data-view]").removeAttr("aria-current");
    const view = state.view === "cves" ? "dashboard" : state.view === "daily" || state.view === "report" ? "" : state.view;
    $(`[data-view="${view}"]`).addClass("is-active").attr("aria-current", "page");
  }

  // Errors carry both languages plus a joined fallback; the dashboard shows only the one in use.
  function errorMessage(error) {
    const body = error.responseJSON;
    if (!body) return t("requestFailed");
    return (state.lang === "ko" ? body.message_ko : body.message_en) || body.message || t("requestFailed");
  }

  function showRequestError(error) {
    toast(errorMessage(error), true);
  }

  function bindEvents() {
    $(document).on("click", "[data-view='dashboard']", () => renderDashboard());
    $(document).on("click", "#settings-action", () => {
      if (isAuthenticated()) renderSettings();
      else {
        pendingLogin = null;
        openLoginModal();
      }
    });
    $(document).on("submit", "#login-form", event => { event.preventDefault(); submitLogin(); });
    $(document).on("submit", "#password-form", event => { event.preventDefault(); changePassword(); });
    $(document).on("input", "#login-password", () => clearFieldErrors("#login-form"));
    $(document).on("input", "#password-form input", () => clearFieldErrors("#password-form"));
    $(document).on("click", "#open-cve-explorer", () => { state.dashboardScroll = $("#main-content").scrollTop(); });
    $(document).on("click", "#retry-cves", renderCVEExplorer);
    $(document).on("click", "#refresh-cves", refreshCVEs);
    $(document).on("click", ".calendar-day[data-day]", function () { if (!this.disabled) selectDay($(this).data("day")); });
    $(document).on("click", "[data-report-id]", function () { openReport(Number($(this).data("report-id"))); });
    $(document).on("click", "#download-report-pdf", downloadReportPDF);
    $(document).on("click", "#download-daily-pdf", downloadDailyPDF);
    $(document).on("click", "#delete-report", () => { if (state.currentReport) openDeleteReportModal(state.currentReport); });
    $(document).on("click", "#confirm-delete-report", function () { deleteReport(Number($(this).data("delete-report-id"))); });
    $(document).on("click", "#open-license-modal", openLicenseModal);
    $(document).on("click", "[data-license-tab]", function () { selectLicenseTab($(this).data("license-tab")); });
    $(document).on("keydown", "[data-license-tab]", function (event) {
      if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
      event.preventDefault();
      const tabs = $("[data-license-tab]").get();
      const current = tabs.indexOf(this);
      const next = event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : (current + (event.key === "ArrowRight" ? 1 : -1) + tabs.length) % tabs.length;
      selectLicenseTab($(tabs[next]).data("license-tab"));
      tabs[next].focus();
    });
    $(document).on("click", "tr[data-href]", function () { window.open($(this).data("href"), "_blank", "noopener"); });
    $(document).on("keydown", "tr[data-href]", function (event) { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); $(this).trigger("click"); } });
    $(document).on("click", ".modal-close", closeModal);
    $(document).on("click", ".modal-dismiss", closeModal);
    $(document).on("click", ".modal-backdrop", function (event) { if (event.target === this) closeModal(); });
    $(document).on("click", "#confirm-collect", function () { runCollection($(this).data("day")); });
    $(document).on("click", ".cancel-collection", () => {
      if (requireAdmin(() => collectionTask.cancel())) collectionTask.cancel();
    });
    $(document).on("click", "#recollect-day", () => openCollectModal(state.selectedDay, true));
    // Only the list is filtered, so rebuilding the toolbar would just steal focus from the select.
    $(document).on("change", "#source-filter", function () {
      state.dailySource = $(this).val();
      $("#main-content .article-list").replaceWith(dailyArticlesHTML(state.daily || { articles: [] }));
    });
    $(document).on("change", "#cve-sort", function () {
      state.cveSort = normalizedCVESort($(this).val());
      localStorage.setItem("cyber-dashboard-cve-sort", state.cveSort);
      $("#cve-sort-hint").text(cveSortHint());
      $("#page-subtitle").text(cveSortHint());
      $("#main-content .cve-page-table").attr("aria-busy", "true");
      loadCVEInsights(cves => {
        $("#main-content .cve-page-table tbody").html(cveExplorerRowsHTML(cves));
        $("#main-content .cve-page-table").attr("aria-busy", null);
      }, error => {
        $("#main-content .cve-page-table").attr("aria-busy", null);
        showRequestError(error);
      });
    });
    $(document).on("change", "#dashboard-range", function () {
      state.dashboardDays = normalizedDashboardDays($(this).val());
      localStorage.setItem("cyber-dashboard-days", String(state.dashboardDays));
      refreshDashboard("range");
    });
    $(document).on("mouseover", ".chart [data-bucket]", function () { showTrendTooltip(this); });
    $(document).on("mouseleave", ".chart", function () {
      const active = document.activeElement;
      if (!active?.matches("[data-bucket]") || !this.contains(active)) $(this).find(".chart-tip").prop("hidden", true);
    });
    // The switch answers immediately and keeps focus; only its bars wait for the response.
    $(document).on("click", "#hide-none-actor", function () {
      state.hideNoneActor = !state.hideNoneActor;
      localStorage.setItem("cyber-hide-none-actor", state.hideNoneActor ? "1" : "0");
      $(this).attr("aria-checked", String(state.hideNoneActor));
      refreshDashboard("actors");
    });
    $(document).on("click", ".toggle[data-source-id]", function () {
      const id = Number($(this).data("source-id"));
      const draft = sourceDraft();
      if (!(id in draft)) return;
      draft[id] = !draft[id];
      paintSourceRow(id, draft[id]);
      refreshSettingsFormState();
    });
    $(document).on("click", "#save-settings", saveSettings);
    $(document).on("click", "#revert-settings", revertSettings);
    $(document).on("click", "#test-llm", testConnection);
    $(document).on("input change", "#setting-language,#llm-base-url,#llm-model,#llm-api-key,#llm-timeout,#nvd-api-key,#timezone-offset", refreshSettingsFormState);
    $(document).on("click", "[data-preset-id]", function () { selectPreset(Number($(this).data("preset-id"))); });
    $(document).on("click", "#add-llm-preset", addCurrentPreset);
    $(document).on("click", "[data-remove-preset-id]", function () { removePreset(Number($(this).data("remove-preset-id"))); });
    $(document).on("click", "[data-report-type]", function () { state.reportType = $(this).data("report-type"); state.reportWeekStart = null; $("[data-report-type]").removeClass("is-selected").attr("aria-pressed", "false"); $(this).addClass("is-selected").attr("aria-pressed", "true"); renderReportPeriodPicker(); });
    $(document).on("change", "#report-year,#report-month", function () { state.reportYear = Number($("#report-year").val()); state.reportMonth = Number($("#report-month").val()); state.reportWeekStart = null; renderReportPeriodPicker(); });
    $(document).on("click", "[data-report-week]", function () { state.reportWeekStart = $(this).data("report-week"); renderReportPeriodPicker(); });
    $(document).on("click", "#generate-report", generateReport);
    $("#auth-action").on("click", logout);
    $("#open-report-modal").on("click", openReportModal);
    $("#calendar-prev").on("click", () => { state.month = new Date(state.month.getFullYear(), state.month.getMonth() - 1, 1); renderCalendar(); });
    $("#calendar-next").on("click", () => { state.month = new Date(state.month.getFullYear(), state.month.getMonth() + 1, 1); renderCalendar(); });
    // Pins an explicit choice; the system preference stops applying from here on.
    $("#theme-toggle").on("click", () => { state.theme = state.theme === "dark" ? "light" : "dark"; localStorage.setItem("cyber-theme", state.theme); document.documentElement.dataset.theme = state.theme; $("#theme-toggle").attr("aria-pressed", String(state.theme === "light")); });
    $("#menu-button").attr({ "aria-controls": "sidebar", "aria-expanded": "false" }).on("click", () => setDrawer(true));
    $("#drawer-close,#drawer-scrim").on("click", closeDrawer);
    $(window).on("resize", () => {
      closeDrawer();
      measureCharts();
    });
    // Observe non-window width changes and retain the observer for its target's lifetime.
    if (typeof ResizeObserver === "function") {
      mainResizeObserver = new ResizeObserver(measureCharts);
      mainResizeObserver.observe(document.getElementById("main-content"));
    }
    $(window).on("hashchange", () => {
      if (window.location.hash === "#cves") renderCVEExplorer();
      else if (state.view === "cves") renderDashboard();
    });
    $(document).on("keydown", event => {
      trapModalFocus(event);
      if (!$("#modal-root").children().length) trapDrawerFocus(event);
      if (event.key === "Escape") { closeModal(); closeDrawer(); }
    });
  }

  function routeCurrentView() {
    if (state.view === "settings") renderSettings();
    else if (state.view === "daily") renderDaily();
    else if (state.view === "report" && state.currentReport) renderReport(state.currentReport);
    else if (state.view === "cves") renderCVEExplorer();
    else renderDashboard();
  }

  function start() {
    bindEvents();
    collectionTask.subscribe(refreshCollectionControls);
    cveRefreshTask.subscribe(refreshCVEControls);
    closeDrawer();
    setLoading();
    refreshSession().always(() => loadBootstrap().done(data => {
      if (window.location.hash === "#cves") renderCVEExplorer();
      else renderDashboard();
      if (data.collection && data.collection.status === "running") {
        runCollection(data.collection.day, data.collection);
      }
      if (data.cve_refresh && data.cve_refresh.status === "running") {
        runCVERefresh(data.cve_refresh);
      }
    }).fail(showRequestError));
  }

  $(start);
})();
