# Home composer

## What exists

One composer at the bottom of a project's Home, after T3 Code's: a rounded box with the text, and a footer with the mode switch **Thread · Note** at the left and a round send button at the right. In Thread mode the footer also has the access chip (Read starts a chat, Write starts a task), the model and effort chip, and `#`, which opens the issue picker in a drawer attached above the box. A picked issue shows as a chip above the text; an issue that already has a thread shows a line above the box, and the send button says "Open thread". In Note mode the send button says "Add note", the text is filed as a note, and pasted or dropped images go with it. The draft stays when the user leaves Home and comes back.

## How a user reaches it

⌘N opens Home in Thread mode with Read, ⌘M with Write; the palette's "New note" opens it in Note mode. Esc on Home puts the focus in the composer. Typing `#` starts the issue picker; Up and Down move in it, Enter or Tab picks, Esc closes it.

## How to drive it

`npx vite` in `desktop/frontend` serves the front end on `mock.ts`, which has issues, notes and sessions. The text is `textarea[aria-label="New thread"]` (or `"New note"`), the mode switch `[aria-label="Composer mode"]`, the chips `[aria-label^="Access"]` and `[aria-label^="Model"]`, the picker `[aria-label="Pick an issue"]` and its list `[role="listbox"][aria-label="Issues"]`. A paste of an image can be sent as a `ClipboardEvent` with a `DataTransfer` that holds a `File`.

## What goes wrong

- Enter with nothing to send must do nothing; with an issue picked and no text it starts the issue.
- A chip menu must close on Esc without the composer losing the focus.
- Wait out the start-up splash (about 6 s) before the first key, or the key is lost.
- Check all three themes: the box fill, outline and segment colours are tokens of their own.
