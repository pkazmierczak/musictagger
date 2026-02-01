# Pattern Configuration Guide

Musictagger now supports configurable renaming patterns using a template syntax.

## Quick Start

Create a `config.json` file:

```json
{
  "replacements": {
    " ": "_",
    "&": "_"
  },
  "pattern": {
    "dir_pattern": "{{artist}}-{{album}}",
    "file_pattern": "{{disc_prefix}}{{track}}-{{title}}"
  }
}
```

Run with the config file:

```bash
musictagger -library /path/to/library -source /path/to/source -config config.json
```

## Available Template Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `{{artist}}` | Artist name (or album artist if available) | `The Beatles` |
| `{{album}}` | Album name | `Abbey Road` |
| `{{title}}` | Track title | `Come Together` |
| `{{track}}` | Track number (zero-padded) | `01` |
| `{{track_raw}}` | Track number (no padding) | `1` |
| `{{total_tracks}}` | Total tracks on disc | `12` |
| `{{disc}}` | Disc number | `1` |
| `{{discs}}` | Total number of discs | `2` |
| `{{disc_prefix}}` | Disc number with dash (only for multi-disc albums) | `2-` or empty |
| `{{year}}` | Release year | `2023` |
| `{{genre}}` | Genre | `Rock` |

## Pattern Examples

### Default Pattern (Current Behavior)
```json
{
  "pattern": {
    "dir_pattern": "{{artist}}-{{album}}",
    "file_pattern": "{{disc_prefix}}{{track}}-{{title}}"
  }
}
```

Result: `the_beatles-abbey_road/01-come_together.flac`

Multi-disc: `pink_floyd-the_wall/2-01-hey_you.flac`

### Year-Based Organization
```json
{
  "pattern": {
    "dir_pattern": "{{year}}/{{artist}}-{{album}}",
    "file_pattern": "{{track}}-{{title}}"
  }
}
```

Result: `1969/the_beatles-abbey_road/01-come_together.flac`

### Genre-Based Organization
```json
{
  "pattern": {
    "dir_pattern": "{{genre}}/{{artist}}/{{album}}",
    "file_pattern": "{{track}}-{{title}}"
  }
}
```

Result: `rock/the_beatles/abbey_road/01-come_together.flac`

### Flat Artist Structure
```json
{
  "pattern": {
    "dir_pattern": "{{artist}}",
    "file_pattern": "{{album}}-{{track}}-{{title}}"
  }
}
```

Result: `the_beatles/abbey_road-01-come_together.flac`

### Album Name in File
```json
{
  "pattern": {
    "dir_pattern": "{{artist}}",
    "file_pattern": "{{year}}-{{album}}-{{track}}-{{title}}"
  }
}
```

Result: `the_beatles/1969-abbey_road-01-come_together.flac`

## Configuration Methods

### 1. Config File (Recommended)

Create a `config.json` file with both replacements and patterns:

```bash
musictagger -library /music -source /downloads -config config.json
```

### 2. CLI Flags (Override)

Override patterns from command line:

```bash
musictagger -library /music \
  -dir-pattern "{{year}}/{{artist}}" \
  -file-pattern "{{album}}-{{track}}-{{title}}"
```

### 3. Legacy Mode

Still works with the old replacements.json:

```bash
musictagger -library /music -replacements replacements.json
```

## Pattern Processing

All patterns are processed with the following steps:

1. Template variables are replaced with metadata values
2. Character replacements are applied (from config)
3. Directory separators (`/`) are replaced with underscores
4. Everything is converted to lowercase
5. Directory names are truncated to 40 characters if needed

## Notes

- All output is automatically converted to lowercase
- Spaces and special characters are replaced according to your replacements table
- Multi-disc albums automatically get the disc prefix when using `{{disc_prefix}}`
- If a config file is not found, defaults are used without error
- CLI flags always override config file settings
