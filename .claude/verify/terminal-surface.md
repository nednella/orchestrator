# Terminal surface

## What exists

The right panel has a Terminal surface. Each group of terminals is one tab of the panel, after the tabs of the views, with the icon of a terminal, the name of its active terminal (the process in front, else "Terminal 2") and the last part of the folder it started in, dimmer. The tabs are the project's: they stay when the thread changes, and each project shows its own. A group holds one to four terminals side by side or stacked. A lone terminal has a floating toolbar at its top right: split side by side, split stacked, new, close. From two terminals the toolbar becomes the header of a 144px list at the right, with one row for each terminal. Closing one terminal asks first. With no terminal the surface shows "No terminals for this project yet." and a New terminal button.

## How a user reaches it

The Terminal row of the panel's list (T), the Terminal button of the sidebar's bottom bar, ⌘J (open or focus; it closes the panel when the Terminal surface is the one shown), ⌘T (new tab), ⌘D (split side by side). While a terminal has the focus, ⌘N makes a new tab, ⌘⇧D splits stacked and ⌘W closes the terminal; these three take the place of the app's own keys for new read-only thread, toggle right panel and detach. A double Esc leaves the terminal for the chat.

## How to drive it

The real app in a browser (see `docs/testing.md`) runs real shells in tmux on the test socket. Tabs are `[role="tab"]` in `[aria-label="Open views"]`; the buttons have the aria-labels of their tooltips. `npx vite` serves the front end on `mock.ts`, whose shells are fake.

## What goes wrong

- Under `vite dev`, React StrictMode mounts each terminal twice, so the focus after a click does not land in the shell. Check the focus in the real app, which is a production build.
- A shell started from a thread starts in that thread's worktree; one started from Home, in the project folder. Its tab shows that folder, not the one it has cd'ed to.
- Closing the last terminal returns the panel to its list.
