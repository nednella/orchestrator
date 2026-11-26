<!-- placeholders: none -->

Drive your thread's own browser window, which has one tab. `action` is one of:

- `open`: load `url` in your window, wait for it to load and return its title. The first `open` makes the window, behind the owner's windows. With `front` true the window comes to the front. Use that each time the owner should look at the page, such as a design you made or a page to log in to, and never for your own checks.
- `screenshot`: file a picture of the tab as evidence for this thread, and return its path. `caption` says what it shows. With `full` true it takes the whole page.
- `close`: close your window. Call it when you are done with the browser.

Everything else on the page goes through `agent-browser` in Bash:

1. Call `open` first. It makes your window and points agent-browser at its tab. Until then every agent-browser command fails.
2. agent-browser is set up for your tab. Never pass `--session`, `--cdp` or `--pin-tab`, and never run `agent-browser connect`, `tab`, `window`, `inspect` or `close`. They take another thread's tab or bring the browser in front of the owner's windows.
3. A link that opens a new tab opens outside your reach. Open its address with `agent-browser open` instead.
4. Run `agent-browser screenshot` with no path and Read the file it prints. A relative path is saved in another folder.
5. agent-browser accepts alert and beforeunload dialogs itself. Answer a confirm or a prompt with `agent-browser dialog accept` or `dialog dismiss`. orchestrator dismisses a dialog left open for 5 seconds.
6. A wait times out after 5 seconds.

Use `show` for other evidence.
