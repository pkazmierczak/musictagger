# librato

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

### One-Shot Mode (CLI)

Process a directory of music files once and exit:

```bash
librato -library /path/to/music/library -source /path/to/new/files
```

**Dry run** (preview changes without moving files):

```bash
librato -library /path/to/music/library -source /path/to/new/files -dry
```

### Daemon Mode (Background Service)

Run continuously and watch a directory for new files:

```bash
librato -daemon \
  -library /mnt/music \
  -watch-dir /mnt/incoming/music \
  -quarantine-dir /mnt/incoming/needs-tagging \
  -config config.daemon.json
```

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

Create a `config.json` file to customize behavior:

```json
{
  "replacements": {
    "ą": "a",
    " ": "_",
    "&": "_"
  },
  "pattern": {
    "dir_pattern": "{{artist}}-{{album}}",
    "file_pattern": "{{disc_prefix}}{{track}}-{{title}}"
  },
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

See [config.daemon.json](config.daemon.json) for a complete example.

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

### Common Flags

- `-library` - Path to music library (required)
- `-config` - Path to configuration file (default: `config.json`)
- `-dry` - Dry run mode (no files moved)
- `-log-level` - Logging level: debug, info, warn, error (default: `info`)

### One-Shot Mode Flags

- `-source` - Source directory to process (default: current directory)
- `-dir-pattern` - Override directory pattern from config
- `-file-pattern` - Override file pattern from config

### Daemon Mode Flags

- `-daemon` - Enable daemon mode
- `-watch-dir` - Directory to watch for new files (required in daemon mode)
- `-quarantine-dir` - Directory for files without tags (required in daemon mode)
- `-pid-file` - PID file path (default: `/var/run/librato.pid`)
- `-state-file` - State file path (default: `/var/lib/librato/state.json`)

## Examples

### Organize downloaded music

```bash
librato -library ~/Music -source ~/Downloads/NewAlbums
```

### Watch a download folder continuously

```bash
librato -daemon \
  -library ~/Music \
  -watch-dir ~/Downloads/Music \
  -quarantine-dir ~/Downloads/Untagged \
  -log-level debug
```

### Custom patterns

```bash
librato \
  -library ~/Music \
  -source ~/Downloads \
  -dir-pattern "{{year}}/{{artist}}/{{album}}" \
  -file-pattern "{{track}}_{{title}}"
```

## License

MIT