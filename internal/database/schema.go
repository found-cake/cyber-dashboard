package database

const schema = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS sources (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  host TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  enabled INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS articles (
  id INTEGER PRIMARY KEY,
  source_id INTEGER NOT NULL REFERENCES sources(id),
  feed_uid TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  published_at TEXT,
  published_time TEXT NOT NULL DEFAULT '',
  collected_at TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  attack_method TEXT NOT NULL DEFAULT '미분류',
  threat_actor TEXT NOT NULL DEFAULT '미확인',
  actor_country TEXT NOT NULL DEFAULT '',
  sector TEXT NOT NULL DEFAULT '일반',
	  victim_count INTEGER NOT NULL DEFAULT 0,
	  zero_day INTEGER NOT NULL DEFAULT 0,
  severity TEXT NOT NULL DEFAULT 'UNKNOWN'
);
CREATE TABLE IF NOT EXISTS daily_summaries (
  day TEXT PRIMARY KEY,
  summary TEXT NOT NULL,
  generated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS cves (
  cve_id TEXT PRIMARY KEY,
  first_seen TEXT NOT NULL,
  cvss_score REAL NOT NULL DEFAULT 0,
  cvss_source TEXT NOT NULL DEFAULT '',
  cvss_version TEXT NOT NULL DEFAULT '',
  affected_product TEXT NOT NULL DEFAULT 'NVD enrichment pending'
);
CREATE TABLE IF NOT EXISTS article_cves (
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  cve_id TEXT NOT NULL REFERENCES cves(cve_id) ON DELETE CASCADE,
  PRIMARY KEY (article_id, cve_id)
);
CREATE TABLE IF NOT EXISTS reports (
  id INTEGER PRIMARY KEY,
  type TEXT NOT NULL,
  period_start TEXT NOT NULL,
  period_end TEXT NOT NULL,
  total INTEGER NOT NULL,
  critical INTEGER NOT NULL,
  high INTEGER NOT NULL,
  medium INTEGER NOT NULL,
  top_threat TEXT NOT NULL,
  actors TEXT NOT NULL,
  sectors TEXT NOT NULL,
  summary TEXT NOT NULL,
  generated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  lang TEXT NOT NULL DEFAULT 'ko',
  theme TEXT NOT NULL DEFAULT 'dark',
  accent TEXT NOT NULL DEFAULT '#4f6ef7',
  llm_base_url TEXT NOT NULL DEFAULT 'https://api.openai.com/v1',
  llm_model TEXT NOT NULL DEFAULT 'gpt-4o-mini',
  llm_api_key TEXT NOT NULL DEFAULT '',
  llm_timeout INTEGER NOT NULL DEFAULT 60,
  nvd_api_key TEXT NOT NULL DEFAULT '',
  timezone_offset_minutes INTEGER
);
CREATE TABLE IF NOT EXISTS llm_presets (
  id INTEGER PRIMARY KEY,
  label TEXT NOT NULL,
  base_url TEXT NOT NULL,
  model TEXT NOT NULL,
  api_key TEXT NOT NULL DEFAULT '',
  builtin INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS llm_presets_endpoint_model_idx
  ON llm_presets (base_url, model);
INSERT OR IGNORE INTO settings (id) VALUES (1);
INSERT OR IGNORE INTO llm_presets (label, base_url, model, builtin)
  VALUES ('OpenAI', 'https://api.openai.com/v1', 'gpt-4o-mini', 1);
INSERT OR IGNORE INTO sources (name, host, slug, enabled) VALUES
  ('보안뉴스', 'boannews.com', 'boannews', 1),
  ('The Hacker News', 'thehackernews.com', 'thehackernews', 1),
  ('Cybersecurity News', 'cybersecuritynews.com', 'cybersecuritynews', 1),
  ('StepSecurity Blog', 'stepsecurity.io/blog', 'stepsecurity', 1),
  ('Dark Reading TI', 'darkreading.com/threat-intelligence', 'darkreading', 1),
  ('BleepingComputer', 'bleepingcomputer.com/news/security', 'bleepingcomputer', 0);
`
