# UI modernization plan

Status: implemented. Refreshed screenshots are in `docs/screenshots/`; run `npm run demo` in `web/` to try it.

## Goals

- Make the dashboard feel like a current product, not a 2020 admin panel.
- Change styling only. No data, backend, routing or behaviour changes.
- Keep every accessibility guarantee: validated palette, status as icon plus label, 4.5:1 text contrast, reduced-motion support, keyboard focus.
- No new runtime dependencies.

## Problems and fixes

| # | Problem | Fix | Where |
|---|---------|-----|-------|
| 1 | 3px coloured top border on every card reads as dated | 1px neutral border, a faint type-coloured gradient wash at the top of the card, and a short rounded accent tab at the top-left. Hover lifts the card and adds a glow in its own hue | `modern.css` (cards), `border-top` removed from 6 card components |
| 2 | Monospace used for brand, server stats, drawer titles | Sans UI font with tabular numbers for numeric text. Monospace stays only for identifiers (pool, dataset, device names, paths, cron) | `--font-sans` / `--font-mono` tokens |
| 3 | Flat, uniform surfaces with no hierarchy | 14px radius, soft layered shadow, more padding, KPI-sized numbers | `modern.css` |
| 4 | Server card is one dense horizontal strip | Responsive grid of stat tiles (host, version, uptime, CPU, temp, network, load) with large values; memory bar below | `modern.css` (server card) |
| 5 | Saturated green bars everywhere | Thinner 6px bars, neutral track, softened fill. Status hues are unchanged, so the validated palette still holds | `modern.css` (bars) |
| 6 | Sidebar wraps "Data protection", loud badges, weak active state | Wider rail, smaller badges, active item gets a tinted pill with an accent marker | `modern.css` (rail), `App.vue` |
| 7 | Top bar scrolls away | Sticky top bar with a blurred translucent background | `modern.css` |
| 8 | Tight 32px table rows | 44px rows, subtle zebra and hover, rounder container | `modern.css` (tables) |
| 9 | Several one-off pill styles | One shared radius, size and weight for pills and badges | `modern.css` |

## Implementation steps

1. Add tokens (`--font-sans`, `--font-mono`, `--radius`, `--shadow-*`) and a new `web/src/modern.css`, imported after `style.css`.
2. Remove `border-top: 3px solid var(--accent)` from the Pool, Dataset, VM, Stack and Container cards. The card accent now comes from `--accent` in `modern.css`.
3. Switch the brand, server stats and drawer titles to the sans font.
4. Restyle the server card, bars, rail, top bar and tables in `modern.css`.
5. Regenerate screenshots with `npm run screenshots`.

## Verification

- `npm run lint`, `npm run test` and `npm run build` pass.
- Dark and light screenshots checked for contrast and clipping.
- Narrow viewport (mobile rail) checked in the demo.

## Out of scope / follow-ups

- Self-hosting Inter (a `@fontsource-variable/inter` dependency). The demo uses the system font stack, which renders as the OS UI font.
- Sparklines in the server tiles (needs history from the backend).
- A command palette and density toggle.

## Rollback

All changes are CSS plus a handful of one-line removals. Reverting the branch restores the old look.
