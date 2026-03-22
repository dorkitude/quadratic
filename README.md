# quadratic

`quadratic` is a local-first Go CLI for backing up and browsing your own Foursquare and Swarm check-in history.

The app:

- authenticates with your own Foursquare account
- stores raw check-ins locally as JSON
- builds a local SQLite archive for fast browsing
- serves a local browser UI for exploring the archive

## Repository

- GitHub: `https://github.com/dorkitude/quadratic`
- Railway static site: link the Railway service directly to this repo and set the root directory to `site/`

## macOS quick start

### 1. Clone the repo

```bash
git clone https://github.com/dorkitude/quadratic.git
cd quadratic
```

### 2. Build the binary

```bash
go build -o quadratic .
```

### 3. Create your config interactively

```bash
./quadratic init
```

You will be prompted for:

- Foursquare `Client ID`
- Foursquare `Client Secret`
- `Redirect URL`
- local data directory

Recommended redirect URL:

```text
http://127.0.0.1:8765/callback
```

### 4. Log in with Foursquare

```bash
./quadratic login
```

This opens the browser, completes OAuth locally, and saves your access token in your user config.

### 5. Sync your check-ins

```bash
./quadratic sync
```

This writes:

- raw JSON backups under `~/.quadratic/data/checkins/`
- a SQLite archive at `~/.quadratic/data/archive.sqlite`

### 6. Browse the archive locally

```bash
./quadratic browse
```

This starts a local site at:

```text
http://127.0.0.1:8787
```

## Commands

- `./quadratic init`
- `./quadratic login`
- `./quadratic sync`
- `./quadratic browse`
- `./quadratic tui`

## Configuration

Config is stored in the user config directory. On macOS that resolves to:

```text
~/Library/Application Support/quadratic/config.yaml
```

Example:

```yaml
client_id: "your-foursquare-client-id"
client_secret: "your-foursquare-client-secret"
redirect_url: "http://127.0.0.1:8765/callback"
data_dir: "/Users/you/.quadratic/data"
```

You can also override values with `QUADRATIC_*` environment variables.

## Data layout

By default the archive lives outside the repo under:

```text
~/.quadratic/data/
```

Files:

- `checkins/*.json`: raw local backup files
- `archive.sqlite`: local queryable archive
- `state.json`: last sync metadata

## Notes

- The app is local-first. Your check-in archive stays on your machine.
- The website in `site/` exists to provide a project URL and privacy policy for OAuth app setup.
- The intended deploy path for `site/` is Railway's native GitHub integration, watching this repo on `main`.
- The repo intentionally ignores the compiled binary and any local SQLite files.
