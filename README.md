# tempo

A [Temporal](https://temporal.io) TUI that matches your rhythm

<p align="center">
  <img src="./assets/tempo-demo.gif" alt="Tempo Demo" width="800">
</p>

## Features

**Workflow Management**
- Browse workflows across namespaces
- View workflow details, inputs, outputs, and metadata
- Inspect full event history with tree and timeline views
- Cancel, terminate, or signal running workflows
- Compare two workflow executions side-by-side (diff view)
- Advanced search with visibility queries and saved filters

**Namespace Operations**
- List and browse all namespaces
- View namespace configuration and details
- Quick namespace switching

**Task Queues & Schedules**
- Monitor task queue activity
- View and manage schedules

**Connection Profiles**
- Save multiple Temporal server configurations
- TLS/mTLS support with certificate paths
- Quick profile switching with `P` key

**Customization**
- 26 built-in color themes (dark and light variants)
- Themes include: TokyoNight, Catppuccin, Dracula, Nord, Gruvbox, One Dark, Solarized, Rosé Pine, Kanagawa, Everforest, Monokai, GitHub
- Live theme preview while selecting

## Installation

### From Source

```bash
go install github.com/galaxy-io/tempo/cmd/tempo@latest
```

### Brew

Brew installed versions will not recieve auto-updates you must update with `brew update`

```bash
brew install galaxy-io/tap/tempo
```

### Build Locally

```bash
git clone https://github.com/galaxy-io/tempo.git
cd tempo
go build -o tempo ./cmd/tempo
```

## Usage

```bash
tempo --address localhost:7233 // default dev server address loads without flag
```

### Command Line Flags

| Flag | Description |
|------|-------------|
| `--address` | Temporal server address (host:port) |
| `--namespace` | Default namespace |
| `--profile` | Connection profile name (from config) |
| `--tls-cert` | Path to TLS certificate |
| `--tls-key` | Path to TLS private key |
| `--tls-ca` | Path to CA certificate |
| `--tls-server-name` | Server name for TLS verification |
| `--tls-skip-verify` | Skip TLS verification (insecure) |
| `--theme` | Theme name |

### Keybindings

**Navigation**
| Key | Action |
|-----|--------|
| `j` / `k` | Navigate down / up |
| `Enter` | Select / expand |
| `Esc` / `Backspace` | Go back |
| `q` | Quit (from root view) |

**Global**
| Key | Action |
|-----|--------|
| `?` | Show help |
| `T` | Theme selector |
| `P` | Profile selector |
| `:` | Command mode |
| `/` | Filter (in workflow list) |

**Workflow Actions**
| Key | Action |
|-----|--------|
| `c` | Cancel workflow |
| `t` | Terminate workflow |
| `s` | Signal workflow |
| `d` | Compare workflows (diff) |

## Configuration

Configuration is stored in `~/.config/tempo/config.yaml` (or `$XDG_CONFIG_HOME/tempo/config.yaml`).

```yaml
theme: tokyonight-night
active_profile: local

profiles:
  local:
    address: localhost:7233
    namespace: default

  staging:
    address: temporal.staging.example.com:7233
    namespace: staging
    tls:
      cert: /path/to/client.pem
      key: /path/to/client-key.pem
      ca: /path/to/ca.pem
```

## Themes

<p align="center">
  <img src="./assets/tempo-themeselect.gif" alt="Theme Selection" width="800">
</p>

26 themes are available, organized by color scheme family:

**Dark Themes**
- `tokyonight-night`, `tokyonight-storm`, `tokyonight-moon`
- `catppuccin-mocha`, `catppuccin-macchiato`, `catppuccin-frappe`
- `dracula`
- `nord`
- `gruvbox-dark`
- `onedark`
- `solarized-dark`
- `rosepine`, `rosepine-moon`
- `kanagawa`
- `everforest-dark`
- `monokai`
- `github-dark`

**Light Themes**
- `tokyonight-day`
- `catppuccin-latte`
- `dracula-light`
- `gruvbox-light`
- `onelight`
- `solarized-light`
- `rosepine-dawn`
- `everforest-light`
- `github-light`

Press `T` to open the theme selector with live preview.

## Requirements

- Go 1.21+
- A running Temporal server

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [Temporal](https://temporal.io) - The workflow engine this client connects to
- [tview](https://github.com/rivo/tview) - Terminal UI library
- [jig](https://github.com/atterpac/jig) - UI component framework
