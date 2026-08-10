# Cyber Dashboard

[English](README.md) · [한국어](README_KR.md)

Cyber Dashboard turns daily cybersecurity news into a local threat-intelligence workspace. It collects enabled news feeds, reads article content, enriches mentioned CVEs with NVD data, classifies the stories through your own OpenAI-compatible LLM endpoint, and produces daily, weekly, and monthly summaries.

It runs locally, stores its data in SQLite, and is available as a single executable with the frontend embedded.

[Download the latest release](https://github.com/found-cake/cyber-dashboard/releases/latest) · [Report an issue](https://github.com/found-cake/cyber-dashboard/issues) · [MIT License](LICENSE)

<div align="center">
  <img src="assets/dashboard_white.webp" alt="Cyber Dashboard in the light theme" width="49%">
  <img src="assets/dashboard.webp" alt="Cyber Dashboard in the dark theme" width="49%">
</div>

## What you can do

- Collect recent articles from six curated cybersecurity sources.
- Read AI-generated article summaries and a combined daily briefing.
- Filter a collected day by news source or recollect it when needed.
- Track recently mentioned CVEs, CVSS scores, affected products, first-seen dates, and mention counts.
- Explore every collected CVE ranked by `CVSS + mentions × 0.2`.
- Review threat-category and threat-actor distributions across the dashboard period.
- Generate and keep weekly or monthly reports in English or Korean.
- Connect cloud-hosted or local models through an OpenAI Chat Completions-compatible API.
- Save multiple LLM presets when different servers use different endpoints or API keys.
- Select collection sources, report timezone, language, and light or dark appearance.

The default sources are:

| Source | Default |
| --- | --- |
| The Hacker News | Enabled |
| Cybersecurity News | Enabled |
| StepSecurity Blog | Enabled |
| Dark Reading TI | Enabled |
| BleepingComputer | Enabled |
| 보안뉴스 / BoanNews | Disabled |

## Requirements

For full functionality, you need:

- An [NVD API key](https://nvd.nist.gov/developers/request-an-api-key). Collection does not start until one is registered.
- An OpenAI-compatible LLM endpoint for article analysis, daily summaries, and reports. Collection can still retain feed data without it, but AI features are skipped and the dashboard shows a warning.
- Google Chrome or Chromium when Dark Reading or BleepingComputer is enabled, because their article pages require browser-based loading.

Release binaries are provided for 64-bit Linux, macOS, and Windows (`amd64` and `arm64`). Users of another platform or architecture can build Cyber Dashboard from source when Go and its dependencies support that target.

The LLM may be remote or run on the same machine, but its server must provide an OpenAI Chat Completions-compatible API. An API key is optional for a compatible local endpoint that does not require authentication.

If API costs are a concern or running a local LLM is impractical, an unofficial local proxy such as [chatgpt-oauth-go](https://github.com/found-cake/chatgpt-oauth-go) provides another way to use a ChatGPT subscription.

## Quick start

### 1. Download and run

Download the `cyber-dashboard-full` file for your operating system and architecture from [GitHub Releases](https://github.com/found-cake/cyber-dashboard/releases/latest). This is the recommended build because it includes the frontend.

On Linux or macOS, rename the downloaded file to `cyber-dashboard-full`, then run:

```sh
chmod +x cyber-dashboard-full
./cyber-dashboard-full
```

On Windows, run the downloaded `.exe` file.

Open <http://127.0.0.1:8080> in your browser. The server listens only on the local loopback address by default.

### 2. Configure the dashboard

Open **Settings**, then:

1. Choose English or Korean.
2. Select the collection sources you want to use.
3. Enter your NVD API key.
4. Select the UTC offset used for collection dates and saved reports.
5. Enter an OpenAI-compatible base URL, model name, and API key when required.
6. Use **Test connection** to verify the LLM endpoint.
7. Select **Save settings**. Unsaved source and settings changes can also be reverted.

![Language, news sources, and NVD API key settings](assets/setting.webp)

The UTC offset defaults to the timezone of the machine that first ran the server. It is stored as a fixed offset rather than a named zone and does not follow daylight saving time, so collection dates and daily summaries keep the same day boundary all year.

Use the API base URL, not the full completion URL. For example:

```text
https://api.openai.com/v1
http://127.0.0.1:8888/v1
```

Cyber Dashboard appends `/chat/completions` when it sends a request.

![OpenAI-compatible LLM and timezone settings](assets/setting_2.webp)

### 3. Daily collection

Select a date in the calendar and start collection. The selectable feed window covers the most recent 10 days according to the configured timezone.

Collection may take several minutes because the application can load full article pages, run per-article AI analysis, query NVD, and create the daily summary. Closing the collection dialog leaves the server-side job running; use the explicit cancel action if you want to stop it. Duplicate collection and CVE-refresh requests are blocked while a job is active.

![Daily threat-intelligence feed with an AI-generated summary](assets/daily.webp)

### 4. Generate reports

Select **New** beside Reports, choose a weekly or monthly period, and generate the report. Reports use the language and timezone saved in Settings at generation time. Deleting a report requires confirmation.

<details>
<summary>Weekly report example</summary>

![Weekly cybersecurity report](assets/weekly.webp)

</details>

<details>
<summary>Monthly report example</summary>

![Monthly cybersecurity report](assets/monthly.webp)

</details>

## How analysis works

Initial article listings and RSS metadata come from [cyber-news-feed](https://github.com/found-cake/cyber-news-feed). It normalizes only the content each source publishes through RSS or Atom into per-source static JSON; it does not crawl article pages. Cyber Dashboard separately loads an article page when it needs the full text.

1. Fetch enabled RSS metadata and load available article pages.
2. Extract full article text and publication metadata.
3. Classify and summarize with the LLM, then enrich CVEs with NVD/CNA data.
4. Store the results in the local SQLite database.
5. Present the dashboard, daily briefings, and weekly/monthly reports.

Article analysis uses the article body when it is available, not only the RSS title or description. The configured LLM classifies the attack method, threat actor, actor country, target sector, victim count, financial damage, patch availability, and zero-day signal. Severity combines relevant CVSS data with contextual signals such as zero-day status, victim impact, financial damage, and patch availability.

NIST-provided CVSS data is preferred when present. CNA assessment data is retained as a fallback, and NVD records marked as rejected are removed from the active CVE view.

Long daily and report inputs are summarized in batches of at most five fact groups and then combined. Two batch summaries are joined directly; three or more are merged through an additional model request. This reduces oversized requests while preserving the collected facts.

### Prompt development

The prompts used to create LLM summaries and weekly/monthly reports were explored, compared, and iteratively refined with **GPT-5.4-mini**, **GPT-5.6-luna**, and **Gemma 4**. These models were used as prompt-development references; they are not runtime requirements. You may register another sufficiently capable model as long as its server provides an OpenAI Chat Completions-compatible API and reliably returns the requested JSON format.

AI output can still contain mistakes. Treat summaries, classifications, and severity as analysis aids, and verify important findings against the linked article and authoritative vulnerability sources before making security decisions.

## Data and privacy

Cyber Dashboard is local-first:

- Articles, CVEs, reports, presets, and settings are stored in a local SQLite database.
- NVD and LLM API keys are encrypted at rest with AES-256-GCM using a locally generated key file.
- Saved API keys are never returned to the settings page. Leave a key field blank to keep the existing value.
- The theme preference stays only in browser `localStorage` and defaults to the system theme.
- The server binds to `127.0.0.1` unless you explicitly change the address.

The default data directory is the operating system's user configuration directory:

| Operating system | Typical location |
| --- | --- |
| Linux | `~/.config/cyber-dashboard/` |
| macOS | `~/Library/Application Support/cyber-dashboard/` |
| Windows | `%AppData%\cyber-dashboard\` |

Stop the application before copying its data, then back up both `dashboard.db` and `dashboard.db.key`. The key file is required to decrypt the API keys stored in the database.

## Executable variants

Each release contains:

- **Embedded executable:** `cyber-dashboard-full-<os>-<arch>` is the recommended standalone application.
- **Server-only executable:** `cyber-dashboard-server-only-<os>-<arch>` serves frontend files from an external `static` directory.
- **Frontend archive:** `frontend.zip` contains the files used by the server-only executable.
- **Checksums:** `SHA256SUMS` contains SHA-256 checksums for the release artifacts.

To use the server-only build on Linux or macOS, extract `frontend.zip` and rename the downloaded executable to `cyber-dashboard-server-only`. Then point it at the extracted `static` directory:

```sh
CYBER_DASHBOARD_STATIC_DIR=/path/to/static ./cyber-dashboard-server-only
```

## Environment variables

- **`CYBER_DASHBOARD_ADDR`** — HTTP listen address. Default: `127.0.0.1:8080`.
- **`CYBER_DASHBOARD_TRUSTED_HOST`** — one additional hostname or IP address accepted by the Host and Origin guard for exceptional non-loopback access.
- **`CYBER_DASHBOARD_DATA_DIR`** — directory containing the database and encryption key. Default: the operating system's user configuration directory.
- **`CYBER_DASHBOARD_STATIC_DIR`** — frontend directory used only by the server-only executable. Default: `static`.

If you only need another port, keep the loopback address and change the port, for example `127.0.0.1:8081`. Setting `CYBER_DASHBOARD_ADDR=0.0.0.0:<port>` listens on every network interface and can make the application reachable from other devices. For exceptional access through a hostname, set that single hostname in `CYBER_DASHBOARD_TRUSTED_HOST` before startup. `localhost` and loopback IP addresses are always accepted. Cyber Dashboard does not provide user authentication, so avoid `0.0.0.0` unless it is necessary and never expose the application directly to the public internet.

## Build from source

Go 1.26 is required. Clone [this repository](https://github.com/found-cake/cyber-dashboard), enter its directory, then run:

```sh
go generate ./generator/license
go run ./cmd/cyber-dashboard-full
```

To run with editable frontend files, use this from the repository root:

```sh
CYBER_DASHBOARD_STATIC_DIR=static go run ./cmd/cyber-dashboard-server-only
```

## Troubleshooting

### `address already in use`

Another process is using port 8080. Stop that process, or start on another loopback port:

```sh
CYBER_DASHBOARD_ADDR=127.0.0.1:8081 ./cyber-dashboard-full
```

### `Check the AI API`

Confirm the base URL and model name, add the server-specific API key when required, increase the timeout for a slow local model, and run **Test connection** again. The base URL should normally end at `/v1`, not `/chat/completions`.

### Article loading is slow or occasionally fails

Dark Reading and BleepingComputer can require browser verification. Keep Chrome or Chromium installed, allow extra time for collection, and retry the affected day if a site temporarily rejects automation. Other successfully collected articles remain available.

### A CVE shows `NVD assessment pending`

Verify the NVD API key and use **Refresh CVEs** later. A score of `0.0` is shown as pending rather than treated as a low-risk score.

## License

Cyber Dashboard is released under the [MIT License](LICENSE). The application and its generated `frontend.zip` include third-party license notices, which are also available from **Settings → Licenses**.
