# TZZ — Time Zone Zen

An interactive terminal tool for comparing times across zones. Slide through the hours like a slide rule and instantly see what time it is everywhere that matters.

![Go](https://img.shields.io/badge/Go-1.24-blue) ![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **Slide-rule interface** — horizontal bar graph of hours per timezone, navigate with arrow keys
- **Visual indicators** — work hours (green), nighttime (dim blue), current time (teal)
- **Primary zone** — pin your home zone to the top; work hours anchor to it as one solid block across all rows
- **Optional config file** — customize zones, work/night hours, and display format
- **Zero dependencies** — single static binary, nothing to install

## Install

### Download a release

Grab the binary for your platform from the [latest release](https://github.com/wkerekes-godaddy/tzz/releases/latest).

```bash
chmod +x tzz-*
./tzz-linux-amd64   # or whichever platform
```

### Build from source

```bash
go install github.com/wkerekes-godaddy/tzz@latest
```

Or clone and build:

```bash
git clone https://github.com/wkerekes-godaddy/tzz.git
cd tzz
go build -o tzz .
./tzz
```

## Usage

```
←/→  or  h/l    Slide through hours
n                Snap to current time
q / Esc          Quit
```

## Configuration

TZZ works out of the box with no config. To customize, create a YAML file at:

- `~/.config/tzz/config.yaml` (preferred)
- `~/.tzz.yaml` (fallback)

```yaml
# Pin your zone to the top — work hours anchor to it
primary: Arizona

# Work hours highlighted green (24h format)
work_hours: [9, 17]

# Night hours dimmed (wraps past midnight)
night_hours: [22, 6]

# Use 24-hour time in the selected-time readout
use_24h: false

# Your timezone list (IANA identifiers)
zones:
  - name: UTC
    tz: UTC
  - name: Eastern
    tz: America/New_York
  - name: Arizona
    tz: America/Phoenix
  - name: Pacific
    tz: America/Los_Angeles
  - name: London
    tz: Europe/London
  - name: Tokyo
    tz: Asia/Tokyo
```

All settings are optional — omitted values use defaults. See [`config.example.yaml`](config.example.yaml) for the full reference.

Find IANA timezone identifiers at the [tz database list](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).
