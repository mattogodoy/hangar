# Hangar

Persist and restore [AeroSpace](https://github.com/nikitabobko/AeroSpace) window layouts across restarts and monitor changes.

Hangar saves your complete workspace state — which monitor each workspace lives on, which workspace each window belongs to, the tiling tree structure (tiles, accordions, nesting), window order, and window sizes — and restores it on demand.

## Install

```sh
brew install mattogodoy/tap/hangar
```

Or build from source:

```sh
go build -o hangar .
```

## Usage

### Save a layout

```sh
hangar save work
```

Snapshots all workspace-to-monitor assignments, window-to-workspace placements, layout tree structure, and window sizes into a named profile stored at `~/.config/hangar/profiles/`.

### Restore a layout

```sh
hangar restore work
```

Restores everything from the saved profile:

1. Moves workspaces back to their saved monitors (skips monitors that aren't connected)
2. Moves windows to their saved workspaces
3. Reconstructs the layout tree (tiles, accordions, nested sub-containers)
4. Restores window order and sizes

Windows are matched by app bundle ID, falling back to app name and title similarity. Windows that no longer exist are skipped; new windows not in the profile are left alone.

### View the layout tree

```sh
hangar tree
```

Draws the current layout tree for all monitors and workspaces:

```
┏━━━━━━━━━━━━━━┓
┃ DELL U3421WE ┃
┗━━━━━━━━━━━━━━┛
  ├── workspace 1 [h_tiles]
  │├── ▪ Firefox — GitHub (1705x1389)
  │└── ▪ Slack — Spotify - Slack (1705x1389)
  └── workspace 2 [h_tiles]
   ├── ▪ LibreWolf — Gmail (850x1389)
   ├── ┬ [v_accordion]
   │   ├── ▪ Telegram — Telegram (850x690)
   │   └── ▪ Element — Element (850x690)
   └── ◈ ChatGPT — ChatGPT (939x1179)
```

Use `--all` to include empty workspaces.

### List saved profiles

```sh
hangar list         # names only
hangar list -d      # with details (date, window/monitor count)
```

### Delete a profile

```sh
hangar delete work
```

## How it works

Hangar talks to the `aerospace` CLI to query and manipulate the window tree. On save, it captures:

- Monitor list and workspace-to-monitor mapping
- All windows with their app bundle ID, title, workspace, parent container layout, and frame geometry

On restore, it rebuilds the layout by:

1. Assigning workspaces to monitors via `move-workspace-to-monitor`
2. Moving windows to workspaces via `move-node-to-workspace`
3. Flattening each workspace tree and reinserting windows in saved position order
4. Reconstructing sub-containers with `join-with` and setting their layout type
5. Resizing windows to match saved dimensions

## Requirements

- macOS
- [AeroSpace](https://github.com/nikitabobko/AeroSpace) window manager

## License

MIT
