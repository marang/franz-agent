# Focus Standard (Franz TUI)

## Purpose

This document defines the mandatory focus and keyboard navigation behavior for
all current and future Franz screens and dialogs.

## Scope

This standard applies to every interactive screen/dialog that contains more
than one actionable UI region.

## Core Concepts

- **Focus area**: A named interactive region (for example: `view_tabs`,
  `input`, `actions`, `results_list`).
- **Active focus area**: Exactly one focus area is active at any time.
- Focus areas must be declared explicitly per screen/dialog.

## Global Key Rules

- `tab`: Move to next focus area.
- `shift+tab`: Move to previous focus area.
- `up` / `down`:
  - If current area supports vertical navigation (for example list), move
    inside the area.
  - On list boundaries, move focus to adjacent area (no dead-end).
  - Otherwise, move to previous/next focus area.
- `left` / `right`:
  - Only for horizontal selection inside active area (tabs/actions).
  - Must not implicitly switch focus area.
- `enter`: Run the primary action in the active area.
- `space`: Toggle selection only when the active area is a selectable list.
- `esc`: Close/back behavior is screen-specific, but must never leave focus in
  an inconsistent state.

## Visual Rules

- Every focus area must render a visible label (for example `View:`,
  `Action:`, `Results:`).
- The active area label must be highlighted.
- Inputs must use `>` label and highlight `>` when input has focus.
- Users must always be able to identify the active focus area at a glance.

## Transition Rules

- Focus transitions are cyclic by default.
- No focus traps are allowed.
- No hidden fallback paths: all transitions must be explicit and testable.

## Implementation Contract

- Do not use magic strings for focus area IDs, labels, or transition logic.
- Use constants/enums for area IDs and labels.
- Use shared helper functions for:
  - next area
  - previous area
  - move within area or leave area at boundary
- Keep key handling deterministic and area-driven.

## Definition of Done for New Screens

- Declared focus areas with deterministic order.
- `tab`/`shift+tab` implemented.
- `up`/`down` behavior implemented with boundary exits.
- Visible highlighted labels for all areas.
- Automated tests for focus traversal and boundary behavior.

## Reference: Skills Manager

- Installed tab: `View -> Action -> Skills Selection`.
- Search tab: `View -> Input -> Action -> Skill Search Results`.
