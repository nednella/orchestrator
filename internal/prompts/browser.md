<!-- placeholders: none -->

This thread has a browser tab. The `browser` tool's text has the rules: call its `open` first, and drive the tab only with these `agent-browser` commands in Bash.

- `open <url>`: go to a page.
- `snapshot -i -c`: the interactive elements, each with a ref such as `@e3`. `-s "<css>"` scopes it; `--delta` after the first one prints only what changed.
- `click @e3`, `fill @e3 "text"`, `select @e3 "label"`, `check @e3`, `press Enter`: act on refs. Snapshot again after the page changes.
- `wait @e3` or `wait --text "<text>"`: wait for what you saw in a snapshot. Never use fixed waits.
- `get text @e3`, `get url`, `read`: read text.
- `screenshot`, then Read the file it prints. `--annotate` labels refs, `--full` takes the whole page.
- `eval "<js>"`: run JavaScript in the page.
- `batch "open <url>" "snapshot -i -c"`: several commands at once.
- `vitals [url]`, `network requests`, `console`, `errors`, `trace start` then `trace stop "$TMPDIR/trace.json"`: performance and debugging.
