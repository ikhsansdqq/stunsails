# StunSails UI rules

StunSails reuses freelance-dashboard's tokens and components (`src/style.css`, `src/components/ui/*`). On top of those:

- **Never put a table inside a card.** `DataTable`, `SimpleTable` and bordered lists (`ActivityList`) carry their own border. Head them with `Block` (`components/ui/block.tsx`): a plain title, a muted line and actions, on the page itself. `Section` (a card) is for stats, charts and detail grids only.
- **Icons come from `@/components/ui/lucide`**, never `lucide-react`. They are the animated set (`@animateicons/react`), loaded after start-up, and play while their row, button, tab or menu item is hovered or focused.
- **Row menus** use `RowMenu`: every item has an icon; view actions first, lifecycle actions next, destructive ones last in red.
- **Every action toasts its outcome** (sonner). Anything dangerous or crucial asks first in a dialog (`useRun({ confirm })` in `components/guard.tsx`), and changes ask for a fresh authenticator code when the last one is over five minutes old.
- **Authenticator codes** use `OtpInput`: six boxes that check themselves on the sixth digit, no submit button.
- **Inputs** carry a placeholder and an accessible name.
- Keep freelance's compact sizes: PageHeader, StatRow, 40 px table rows, 56 px sparks.
- **Sign-in** (`pages/Login.tsx`, `login.css`) is a console prompt on StunSails' own tokens, so it follows light and dark: one centred input, Enter to continue, and a tmux-style status line along the foot as the only ornament. Mono text there and in OTP digits is IBM Plex Mono (`font-mono`).
- **Activity** pages on the server (`/api/activity/page`); a row opens with a click, Enter on its chevron, or Space, and loads its addresses from `/api/activity/{id}`.
