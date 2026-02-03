# librato

[![Build](https://github.com/pkazmierczak/librato/actions/workflows/go.yml/badge.svg)](https://github.com/pkazmierczak/librato/actions/workflows/go.yml)

A music file organizer that uses ID3 tags to automatically organize your music library. Can run as a one-shot CLI tool or as a background daemon service.

## Features

- **Pattern-based organization**: Use customizable templates to organize files by artist, album, track number, etc.
- **One-shot mode**: Process an entire directory at once (traditional CLI behavior)
- **Daemon mode**: Run as a background service that watches a directory for new files
- **Quarantine for untagged files**: Files without metadata are moved to a separate directory
- **State persistence**: Daemon mode tracks processed files to avoid duplicates
- **Systemd integration**: Easy deployment as a Linux service

## Installation

### From Source

```bash
git clone https://github.com/pkazmierczak/librato
cd librato
make build
```

## Usage

Most options are configured via a config file (default: `/etc/librato/config.json`). See the [Configuration](#configuration) section below.

### One-Shot Mode (CLI)

Process a directory of music files once and exit:

```bash
librato -config config.json -source /path/to/new/files
```

**Dry run** (preview changes without moving files):

```bash
librato -config config.json -source /path/to/new/files -dry
```

### Daemon Mode (Background Service)

Run continuously and watch a directory for new files:

```bash
librato -daemon -config config.daemon.json
```

All daemon settings (watch directory, quarantine directory, etc.) are configured in the config file.

**How daemon mode works:**
1. On startup, scans the watch directory for existing files
2. Processes any files with valid ID3 tags and organizes them into the library
3. Moves files without tags to the quarantine directory with a timestamp
4. Continuously watches for new files dropped into the watch directory
5. Automatically processes new files after a 2-second debounce period
6. Removes empty directories after moving files out

### Systemd Service Installation

For production deployment as a systemd service:

```bash
# Build the binary
make build

# Install as systemd service (requires root)
sudo ./scripts/install-daemon.sh

# Edit configuration
sudo nano /etc/librato/config.json

# Enable and start service
sudo systemctl enable librato
sudo systemctl start librato

# Check status
sudo systemctl status librato

# View logs
sudo journalctl -u librato -f
```

## Configuration

Create a `config.json` file to customize behavior. The default location is `/etc/librato/config.json`.

### One-Shot Mode Config

```json
{
  "library": "/path/to/music/library",
  "log_level": "info",
  "replacements": {
    "ą": "a",
    " ": "_",
    "&": "_"
  },
  "pattern": {
    "dir_pattern": "{{artist}}-{{album}}",
    "file_pattern": "{{disc_prefix}}{{track}}-{{title}}"
  }
}
```

### Daemon Mode Config

For daemon mode, add a `daemon` section:

```json
{
  "library": "/mnt/music",
  "log_level": "info",
  "replacements": { ... },
  "pattern": { ... },
  "daemon": {
    "watch_dir": "/mnt/incoming/music",
    "quarantine_dir": "/mnt/incoming/needs-tagging",
    "debounce_time": "3s",
    "state_file": "/var/lib/librato/state.json",
    "pid_file": "/var/run/librato.pid",
    "scan_on_startup": true,
    "cleanup_empty_dirs": true
  }
}
```

See [config.example.json](config.example.json) for a one-shot example and [config.daemon.json](config.daemon.json) for a daemon example.

### Pattern Variables

Available template variables:

- `{{artist}}` - Artist name (or album artist if available)
- `{{album}}` - Album name
- `{{title}}` - Track title
- `{{track}}` - Track number (zero-padded to 2 digits)
- `{{track_raw}}` - Track number (no padding)
- `{{total_tracks}}` - Total number of tracks
- `{{disc}}` - Disc number
- `{{discs}}` - Total number of discs
- `{{disc_prefix}}` - Disc prefix for multi-disc albums (e.g., "2-")
- `{{year}}` - Release year
- `{{genre}}` - Genre

See [PATTERNS.md](PATTERNS.md) for more details.

## CLI Flags

- `-config` - Path to configuration file (default: `/etc/librato/config.json`)
- `-daemon` - Enable daemon mode
- `-source` - Source directory to process in one-shot mode (default: current directory)
- `-dry` - Dry run mode (no files moved)

## Examples

### Organize downloaded music

```bash
librato -config config.json -source ~/Downloads/NewAlbums
```

### Watch a download folder continuously

```bash
librato -daemon -config config.daemon.json
```

## License

MIT