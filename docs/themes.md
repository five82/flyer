# Theme Guidelines

Flyer supports multiple color themes defined in `internal/ui/theme.go`. When modifying themes:

1. **Use official palettes only** - do not invent colors
2. **Follow established UI hierarchies** - reference canonical implementations
3. **Maintain proper contrast** - backgrounds should be dark enough for text readability

## Nightfox

**Spec**: https://github.com/EdenEast/nightfox.nvim

| Role | Hex | Usage |
|------|-----|-------|
| bg0 | `#131a24` | Outermost background |
| bg1 | `#192330` | Main content panels |
| bg2 | `#212e3f` | Secondary surfaces |
| bg3 | `#29394f` | Focus/active states |
| fg1 | `#cdcecf` | Primary text (cool gray) |
| comment | `#738091` | Muted text (3.3:1 contrast) |
| fg3 | `#71839b` | Dimmest text (3.1:1 contrast) |
| blue | `#719cd6` | Focus borders, accent |
| cyan | `#63cdcf` | Info |
| green | `#81b29a` | Success |
| yellow | `#dbc074` | Warning |
| red | `#c94f6d` | Error/danger |

## Kanagawa

**Spec**: https://github.com/rebelot/kanagawa.nvim

| Role | Hex | Usage |
|------|-----|-------|
| sumiInk0 | `#16161D` | Outermost background |
| sumiInk3 | `#1F1F28` | Main content panels |
| sumiInk4 | `#2A2A37` | Focus/active states, secondary surfaces |
| fujiWhite | `#DCD7BA` | Primary text (warm parchment) |
| oldWhite | `#C8C093` | Muted text (7.6:1 contrast) |
| fujiGray | `#727169` | Dimmest text (2.8:1 contrast) |
| crystalBlue | `#7E9CD8` | Focus borders, accent |
| springBlue | `#7FB4CA` | Info |
| springGreen | `#98BB6C` | Success |
| carpYellow | `#E6C384` | Warning |
| waveRed | `#E46876` | Error/danger |

## Slate

**Palette**: https://tailwindcss.com/docs/colors
**UI hierarchy**: https://ui.shadcn.com/docs/theming

| Role | Tailwind | Hex | Usage |
|------|----------|-----|-------|
| Background | slate-950 | `#020617` | Outermost background |
| Surface | slate-900 | `#0f172a` | Main content panels |
| SurfaceAlt | slate-800 | `#1e293b` | Secondary surfaces, borders |
| FocusBg | ~slate-750 | `#283548` | Focus/active states |
| Foreground | slate-100 | `#f1f5f9` | Primary text |
| Muted | slate-400 | `#94a3b8` | Muted text |
| Faint | slate-500 | `#64748b` | Dimmest text |
| Accent | sky-400 | `#38bdf8` | Focus borders, links |

Use semantic color roles (Success, Warning, Danger, Info) mapped to appropriate palette colors for each theme.
