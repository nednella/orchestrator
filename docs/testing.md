# Testing a change

## Isolation first

Twice a test that used the default tmux socket killed the owner's live sessions. Never run anything that starts the app, a session or the CLI against the default socket or the real state folders. Every run outside `go test` MUST set all of these:

```sh
S=$(mktemp -d "${TMPDIR:-/tmp}/aos.XXXX")
export AGENTOS_TMUX_SOCKET=aos-test-$$      # never "agentos", the owner's socket
export AGENTOS_STATE_DIR=$S                 # never ~/.local/state/agentos
export AGENTOS_DEV_DATA_DIR=$S/data             # never ~/.local/share/agentos (also moves notes and the event log)
export AGENTOS_CONFIG=$S/config.yaml        # never ~/.config/agentos/config.yaml
unset AGENTOS_SOCKET AGENTOS_SESSION AGENTOS_ISSUE  # an agentos session has them; AGENTOS_SOCKET sends every command to the live app
```

A throwaway config, with `{"model": "haiku"}` in the demo project's `.claude/settings.local.json` so its threads run `claude` on the cheapest model:

```yaml
projects:
  - {name: demo, directory: <$S>/demo}
```

Clean up afterwards: `tmux -L $AGENTOS_TMUX_SOCKET kill-server`, kill the app and any browser started on `$S/data/<key>/browser`, `rm -rf $S`. Check with `pgrep -fl "$S"`.

An agentos session runs with `TMPDIR` set to its own temp folder, which it may use without asking and which goes when the thread is deleted or cleaned up. Put screenshots and scratch files there, never in `/tmp`.

## Unit and integration tests

```sh
go vet ./...
go test -race ./...
```

Each `desktop/<pkg>` service has its tests beside it. They build the whole app through `desktop/internal/apptest`, which starts real tmux servers on private sockets (`aostest-<pid>`), real git repos in temp folders, and real headless Chrome for Testing on temp profiles. GitHub and `claude` are always replaced by fake runners (`apptest.FakeGH`, `FakeClaude`); commands run through `desktop/internal/run`. Every session runs a stand-in `claude`, a shell script the harness puts first on PATH, under the built command's host, as a real Claude session does: it answers each message with a reply and each control request with a success, and `Harness.Stdin` and `Harness.Args` give the lines it read and the arguments it was started with. A test checks what reached Claude there, or in the chat log for a draft, not in the pane. The browser tests skip when Chrome for Testing is not installed; CI points `AGENTOS_BROWSER` at its Google Chrome. The agent-browser test runs the real `agent-browser` only when `AGENTOS_AGENT_BROWSER` names it: run `make agent-browser` and set it to `bin/agent-browser`. Run it before a bump of `AGENT_BROWSER_VERSION`, since agentos writes a file of agent-browser's own. `desktop` embeds `frontend/dist`; build it first, or run with `-tags stub`, which embeds a small page instead. The hooks in the tests run the `agentos` command built from `./desktop` with that tag: the app's binary is the command. The `cli` tests answer from a fake app on a temp control socket. The release check never reaches GitHub: the harness answers curl with `Options.Releases` and refuses every other download.

## Recorded agents

`internal/agent/testdata/<provider>/` holds what the real agents sent: for each scenario in `scenario_test.go` (a prompt, a tool call, a question, a permission prompt, an exit, a resume) the hook payloads and the transcript lines with usage, and in `version` the agent's version. `TestScenarios` replays them through the provider as the app reads them and checks the state, reply and usage each row must end with.

After an update of `claude`, record again with the isolation env above set:

```sh
go test ./internal/agent -run TestRecord -record claude -v
git diff internal/agent/testdata
go test ./internal/agent -run TestScenarios
```

The diff shows what the agent changed, the test what that breaks. Recording runs each agent for real, on a cheap model, in a throwaway folder on a tmux server of its own with no agentos session around it, and spends a few requests. The agents use the owner's login; the recording keeps no line of the transcripts besides the model calls, and puts `$WORK`, `$HOME` and `$RECORDING` in place of the machine's paths.

## The real front end, in a browser

`AGENTOS_HTTP` serves the embedded front end over HTTP with a shim for `window.go` and `window.runtime`: `POST /__call/<pkg>.Service/<Method>` with a JSON array of arguments and the header `X-Agentos: 1`, and server-sent events at `/__events`. The server answers only requests addressed to its own listen address (`127.0.0.1:PORT`, not `localhost`) and refuses a foreign `Origin` with 403.

```sh
make -o desktop-frontend desktop-app          # or: make desktop-app (rebuilds the front end)
(cd / && AGENTOS_HTTP=127.0.0.1:18772 bin/agentos-dev.app/Contents/MacOS/agentos) &
curl -s -XPOST -H 'X-Agentos: 1' 127.0.0.1:18772/__call/projects.Service/Snapshot -d '[]'
```

A thread's tools reach the same app as `orchestrate` requests on its control socket, each with the thread's token (see [The agent tools](#the-agent-tools)). A shell session (`ShellNew`) runs `$SHELL -l`; set `SHELL=/bin/sh` and a throwaway `HOME` in tests so no profile of the owner runs. Stop it with SIGTERM so it stops its browsers.

## The agent tools

A thread calls the agentos tools (`note`, `issue`, `new_thread`, `show`, `track`, `refresh`, `browser` and the orchestration tools) through the `agentos mcp` server its `<id>.mcp.json` starts, which sends each call as `orchestrate` with the thread's token. The stand-in `claude` calls no tool, so the tests in `desktop/orchestration` make the calls themselves: `useTool` sends `orchestrate` with the token read from the thread's MCP config, and `TestMCPServerServesTheAgentToolsAsTheThread` runs the built `agentos mcp` from that config and speaks JSON-RPC to it. A test of a tool checks the effect as the caller (the note in the caller's project, the branch on the caller's row) and that a call whose request or input names another thread still acts as the caller. `internal/host` tests which tool asks the owner: a Read thread asks for `show` and `track`.

Claude signs in through its own config folder, so a check with a real Claude writes its transcript into `~/.claude`; a throwaway `CLAUDE_CONFIG_DIR` is not signed in. To check with one, run the app in a browser as above, with the isolation env, and a demo project whose `.claude/settings.local.json` is `{"model": "haiku"}`; start a Read thread and send "note that the build is slow". The chat shows an `mcp__agentos__note` call with no approval card, and the note shows in the Notes view. Ask the same thread to show some text as evidence: the chat shows an approval card for `mcp__agentos__show`.

## Driving the page

`scripts/cdp.mjs` opens a page in its own headless Chrome for Testing (its own temp profile, never the owner's) and runs steps: wait, click at a point, type, key, eval JavaScript, screenshot.

```sh
node scripts/cdp.mjs http://127.0.0.1:18772/ 1400 900 \
  '[{"wait":1500},{"eval":"document.title"},{"click":[700,400]},{"wait":300},{"shot":"aos-shot.png"}]'
```

A shot path that is not absolute lands in `$TMPDIR`. Read the screenshot to check what the page shows. Resize with the width and height arguments to check the layouts.

## Releases

`make release VERSION=v1.2.3` builds `bin/agentos-darwin-arm64.zip` and its `.sha256` the way the release workflow does, with the version stamped in the bundle's Info.plist. To try the installer or `agentos update` without a real release, serve `bin/` over HTTP and point them at it with a throwaway `HOME`; never run either against the owner's `~/Applications` or `~/.local/bin`.

## What needs a human

The real Wails window: the title bar, drag and resize, native menus and shortcuts, the folder picker, the dock icon, clipboard through the system, and a launch from Finder (its PATH and locale). The browser mode covers everything else.
