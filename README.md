# linkding-ssg

A fast, self-contained Go CLI to periodically back up bookmarks from a [linkding](https://linkding.link/) instance to a local JSON snapshot and generate a static HTML archive with stable date URLs, tag folders, sharded individual permalinks, and [Pagefind](https://pagefind.app/) full-text client-side search.

---

## Features

- **Linkding API Client**: Fetches all bookmarks using limit/offset pagination with automatic retry and exponential backoff.
- **Atomic Backup Snapshot**: Stores your entire collection into `data/bookmarks.json` atomically (via temporary file + rename), eliminating corruption risks.
- **Stable URL Architecture**:
  - `/`: Recent bookmarks (latest 50 links).
  - `/archive/`: Yearly summary grid linking to months.
  - `/archive/YYYY/MM/`: Monthly archive pages (immutable once past).
  - `/tags/`: Tag cloud / index with counts.
  - `/tags/{tag}/`: All bookmarks matching a tag.
  - `/b/{shard}/{id}/`: Sharded canonical permalinks (`/b/012/12345/`) ensuring clean filesystem organization even with 30,000+ links.
- **Pagefind Client-side Search**:
  - Automatically indexes bookmark cards and tag filters.
  - Built-in search UI at `/search/`.
  - Runs local `pagefind` binary or falls back to `npx -y pagefind`.
- **Local Dev Server**: Built-in HTTP server (`linkding-ssg serve`) with cache-busting headers for instant preview.
- **Embedded Templates & Styles**: Ships as a single binary with default responsive CSS and dark mode support; customize anytime with `--templates-dir`.

---

## Installation & Build

```bash
go build -o linkding-ssg .
```

---

## CLI Usage

### 1. Backup Bookmarks to JSON

```bash
./linkding-ssg backup \
  --url https://your-linkding-instance.com \
  --token YOUR_API_TOKEN \
  --output data/bookmarks.json
```

Or use a `.env` file (see `.env.example`):
```bash
cp .env.example .env
# Edit LINKDING_URL and LINKDING_TOKEN in .env

./linkding-ssg backup
```

By default, `backup` runs **incrementally** if an existing snapshot exists at `--output`, stopping as soon as it reaches bookmarks already saved locally, then merges the new and existing bookmarks.

To force a full sync of all bookmarks:
```bash
./linkding-ssg backup --full
```
### 2. Build the Static Site

Generate HTML into `dist/` directly from the local JSON file (no network calls):

```bash
./linkding-ssg build --input data/bookmarks.json --output-dir dist
```

To configure RSS feed item counts (default 15) or site title:
```bash
./linkding-ssg build --rss-limit 25 --site-title "My Bookmarks"
```

To automatically run Pagefind search indexing after generating HTML:
```bash
./linkding-ssg build --input data/bookmarks.json --output-dir dist --index
```
### 3. Search Indexing Only

If you regenerated or edited files and only need to re-index:
```bash
./linkding-ssg index --site-dir dist
```

### 4. Local Preview Server

Start a local HTTP server at `http://localhost:8080`:
```bash
./linkding-ssg serve --site-dir dist --addr :8080
```
### 5. Customizing Templates & Static Assets (`eject`)

By default, the binary is 100% self-contained and uses embedded templates and CSS. To customize them:

```bash
./linkding-ssg eject
```

This writes the default templates to `data/templates/` and static files to `data/static/`. Once these folders exist in `data/`, `linkding-ssg build` will automatically use them to override the embedded versions.

You can also pass custom paths via CLI flags or environment variables:
```bash
./linkding-ssg build --templates-dir my-templates/ --static-dir my-static/
```

### 6. One-Shot Sync (Backup + Build + Index)

Ideal for cron jobs or automated daily tasks:
```bash
./linkding-ssg sync
```

---

## Configuration Reference

Every CLI option can be configured via flags, environment variables, or a `.env` file (CLI flags take precedence over `.env`):

| CLI Flag | Environment Variable | Default | Description |
|---|---|---|---|
| `--url` | `LINKDING_URL` | *None (required)* | Base URL of Linkding instance |
| `--token` | `LINKDING_TOKEN` | *None (required)* | REST API token |
| `--page-size` | `LINKDING_PAGE_SIZE` | `100` | API items fetched per request |
| `--full` | `LINKDING_FULL_BACKUP` | `false` | Force full backup instead of incremental |
| `--output` / `--input` / `--snapshot` | `LINKDING_SNAPSHOT_PATH` | `data/bookmarks.json` | Path to JSON snapshot |
| `--output-dir` / `--site-dir` | `LINKDING_OUTPUT_DIR` | `dist` | Generated HTML output directory |
| `--base-url` | `LINKDING_BASE_URL` | `""` | Site URL prefix (e.g. `/` or `/bookmarks/`) |
| `--recent-limit` | `LINKDING_RECENT_LIMIT` | `50` | Number of recent items on home page |
| `--rss-limit` | `LINKDING_RSS_LIMIT` | `15` | Number of items in RSS feeds |
| `--site-title` | `LINKDING_SITE_TITLE` | `Bookmarks` | Feed and site header title |
| `--templates-dir` | `LINKDING_TEMPLATES_DIR` | `data/templates` (or embedded) | Path to custom HTML templates |
| `--static-dir` | `LINKDING_STATIC_DIR` | `data/static` (or embedded) | Path to custom static assets |
| `--concurrency` | `LINKDING_CONCURRENCY` | System CPU count | Number of parallel worker goroutines |
| `--index` | `LINKDING_AUTO_INDEX` | `false` | Run Pagefind indexer after build |
