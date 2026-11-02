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

### 5. One-Shot Sync (Backup + Build + Index)

Ideal for cron jobs or automated daily tasks:
```bash
./linkding-ssg sync
```
