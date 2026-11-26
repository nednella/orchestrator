# Verify map

Start the real app in a browser with the isolation env from `docs/testing.md`: `ORCHESTRATOR_HTTP=127.0.0.1:PORT` on a bare copy of `bin/orchestrator-dev.app/Contents/MacOS/orchestrator`. The app starts its services before it listens, so a page is not needed first; it prints the token the page needs in its address (`?k=<token>`) on stderr.

- [Shell tabs](shell-tabs.md): the tab row of the shell drawer, open and collapsed, and the same row in the narrow layout.
- [Home composer](home-composer.md): the one composer on a project's Home, its Thread and Note modes, chips, issue picker and send button.
- [Thread chat](thread-chat.md): a thread's chat in the centre, its scroll follow, the "Jump to latest" pill and the kept position.
- [Thread composer](thread-composer.md): a thread's composer: send, queue and Send now, Stop and Esc, drafts from orchestrator, the read-only model and the settled thread.
