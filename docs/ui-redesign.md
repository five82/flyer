# UI Redesign Plan

Status: implemented (July 2026). This document is the design record.

Flyer's UI is rebuilt around two layers instead of a split pane: a full-width
**dashboard** (glance: is everything OK, what is running, what needs me) and a
full-screen **inspector** (drill: one item's pipeline, media, output, errors,
episodes, logs). This follows the k9s model: list screen, Enter to inspect,
Esc back.

## Why

Findings from the pre-redesign review of `internal/ui`:

- The detail pane grew by accretion: four state branches
  (pending/active/completed/failed) assembling from ~10 section types with
  abstract names (Focus, Scope, Audit, Recovery, Summary, Results). Sections
  overlap (completed items render both Results and Audit with repeated
  fields), errors render in three places, and the layout changes shape as an
  item moves through states, so positions are never learnable.
- Text wrapping in the detail pane was an unimplemented TODO; long error
  messages clip.
- The `P`/Paths binding toggled state nothing rendered (dead).
- Tab conflated pane focus with view switching; Problems was a per-item view
  (scoped to an invisible queue selection) presented as a global one.
- The 40/60 split cramped both halves: no room for queue columns, no room to
  wrap or lay out episode tables.
- Pane background painting (`renderBox`, `BgStyle`, `FillLine`,
  `WithBackground`) was roughly a quarter of the UI code and taxed every
  change.
- API data left unsurfaced: encoding substage, encoder name, ContentID detail.

Keep (the good parts): the scheduler task board, the episode asset grid
(`R? E? S? F?` columns), the stage catalog with graceful unknown-name
handling, resource ordering derived from the daemon's pipeline template, and
the per-task queue-row glyph strip.

## Target design

### Dashboard (default screen)

```
flyer * ON   Queue 7   Failed 1  Review 2                        14:32
NOW  Drive FREE - insert next disc | GPU #12 analyzing 40% | ENC #11 encoding 62% 41fps ETA 18m
f:All  t:Episodes  j/k:Navigate  Enter:Inspect  l:Daemon  p:Problems  ?:More
------------------------------------------------------------------------
  ^^o....  #11  The Thin Man (1934)          encoding    62%   2m ago
  ^^^o...  #12  Columbo S03 Disc 2           analyzing   40%  10s ago
? ^x.....  #09  Unknown Disc                 REVIEW            1h ago
x ^^x....  #08  Heat (1995)                  FAILED            3h ago
  .......  #13  M*A*S*H S05 Disc 1           waiting           5m ago
```

- Header line 1: identity/health only (logo, ON/OFF, queue count,
  failed/review counts, clock, HEALTH/WORKFLOW/ERROR indicators). Parts are
  built in priority order and dropped from the end until the line fits; no
  silent cropping.
- NOW band: full-width line, one segment per scheduler resource with the
  holder's item, task, percent, fps/ETA; the drive segment always renders
  (FREE / PAUSED / holder) because "insert the next disc" is the single most
  useful signal. Falls back to a bare active count for old daemons.
- Queue table: full width, columns = task strip, id, title, stage label
  (role-colored), running percent, updated-ago. Title takes the slack.
  Selection is ID-sticky. Enter opens the inspector.

### Inspector (full screen, one item)

Persistent one-line item header (title, chips, id), then sub-tabs:

- `1` Overview - fixed section skeleton (below)
- `2` Episodes - episode list with asset grid, `t` collapse toggle
- `3` Problems - structured problems + warn/error logs for this item
- `4` Logs - all item logs (follow, search)

Esc returns to the dashboard. Tab/Shift-Tab also cycle tabs.

### Overview skeleton

Same sections, same order, for every item state; rows appear or disappear by
data presence, never by state branching:

1. Header - title, chips (stage, MOVIE/TV, REVIEW, ERROR, CACHE), timestamps
2. Pipeline - task board, one row per scheduler task, inline progress,
   substage on the running encode row, per-task context lines (active
   episode/track, subs progress, files) under the running row
3. Attention - only when review/error/failed validation: review reasons,
   error message, Reel cause/context/suggestion, failing validation steps
4. Media - source title, video specs, audio, crop, encoder + preset/quality/
   tune, commentary count, ContentID method/matched
5. Output - estimated size while encoding; size result, encode stats,
   validation summary, subtitle summary, file states when done
6. Episodes - summary line (full list on the Episodes tab)

All value text word-wraps with continuation lines indented under the value
column.

### Problems view (global)

Triage list: every failed/review item (strip glyph, id, title, lead reason
from failed task / review reasons / error message). j/k + Enter opens the
inspector on that item's Problems tab.

### Chrome

Color-on-default-background everywhere (lazygit/btop style). Background fill
survives only on the queue selection bar and status chips. Panes are titled
with a rule line, not boxes. `renderBox`, `BgStyle`, `FillLine`, and
`Styles.WithBackground` are deleted.

### Navigation

- `q` queue/dashboard, `l` daemon logs, `p` problems, `?` help, `T` theme
- `Enter` inspect selected item, `Esc` back
- `1-4` / Tab / Shift-Tab switch inspector tabs
- j/k always move the obvious thing; no pane-focus cycle

## Phases

Each phase compiles, passes `./check-ci.sh`, and is usable on its own.

- **Phase 0 - dead code.** Remove the `P`/Paths binding and `pathExpanded`
  state.
- **Phase 1 - chrome simplification.** Delete the background-painting layer
  (`BgStyle`, `FillLine`, `WithBackground`, `renderBox`, manual line fills,
  `SurfaceAlt`/`FocusBg` scheme); mechanical rewrite of call sites to plain
  foreground styles. Look changes, information does not.
- **Phase 2 - dashboard.** Header priority-dropping, NOW band, full-width
  queue table; Enter shows detail full-screen (interim); queue-internal focus
  toggle removed.
- **Phase 3 - inspector shell.** Inspector state + tabs; per-item problems
  and item logs move into it; global Problems becomes the triage list;
  `toggleFocus`/`focusedPane` deleted; keymap/help updated.
- **Phase 4 - overview skeleton.** Rewrite `renderDetailContent` as the fixed
  six-section layout; delete the state branches and legacy sections;
  implement word wrap; surface substage/encoder; snapshot tests per item
  state.
- **Phase 5 - polish.** Help modal and command bar around the final key set;
  compact-width behavior for the NOW band and queue columns.

Net LOC ends below the pre-redesign ~6k.
