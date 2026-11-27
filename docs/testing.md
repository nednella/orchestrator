# Testing a change

## Isolation first

Twice a test that used the default tmux socket killed the owner's live threads. Never run anything that starts the app, a thread or a host against the default socket or the real state folders. Every run outside `go test` MUST set all of these:

```sh
S=$(mktemp -d "${TMPDIR:-/tmp}/orc.XXXX")
export ORCHESTRATOR_TMUX_SOCKET=orc-test-$$      # never "orchestrator", the owner's socket
export ORCHESTRATOR_STATE_DIR=$S                 # never ~/.local/state/orchestrator
export ORCHESTRATOR_DEV_DATA_DIR=$S/data             # never ~/.local/share/orchestrator (also moves notes and the event log)
export ORCHESTRATOR_CONFIG=$S/config.yaml        # never ~/.config/orchestrator/config.yaml
unset ORCHESTRATOR_THREAD ORCHESTRATOR_CONTROL ORCHESTRATOR_PROJECT ORCHESTRATOR_MCP_TOKEN  # a thread has the first two and its tools' server the third, and a shell has ORCHESTRATOR_PROJECT: the live app sets them, so a check run from one of its threads must unset them
```

A throwaway config, with `{"model": "haiku"}` in the demo project's `.claude/settings.local.json` so its threads run `claude` on the cheapest model:

```yaml
projects:
  - {name: demo, directory: <$S>/demo}
```

Clean up afterwards: `tmux -L $ORCHESTRATOR_TMUX_SOCKET kill-server`, kill the app and any browser started on `$S/data/<key>/browser`, `rm -rf $S`. Check with `pgrep -fl "$S"`.

An orchestrator thread runs with `TMPDIR` set to its own temp folder, which it may use without asking and which goes when the thread is deleted or cleaned up. Put screenshots and scratch files there, never in `/tmp`.

## Unit and integration tests

```sh
go vet -tags stub ./...
go test -race -tags stub ./...
```

CI runs the same tests in numbered shards, one package at a time. `scripts/go-test-shard.sh` holds what each shard runs, by measured time: its last shard runs every package the others do not name, so a new package runs with no change there, and a package that takes longer than a shard is split by its tests. A change of the shard count changes the workflow's matrix too; the script refuses a count other than its own.

Each `desktop/<pkg>` service has its tests beside it. They build the whole app through `desktop/internal/apptest`, which starts real tmux servers on private sockets (`orctest-<pid>-<n>`, one per harness), real git repos in temp folders, and real headless Chrome for Testing on temp profiles. GitHub and `claude` are always replaced by fake runners (`apptest.FakeGH`, `FakeClaude`); commands run through `desktop/internal/run`. Every thread runs a stand-in `claude`, a shell script the harness puts first on PATH, under the built command's host, as the real Claude does: it answers each message with a reply and each control request with a success, and `Harness.Stdin` and `Harness.Args` give the lines it read and the arguments it was started with. A harness gives the app its own environment, `Harness.Env` (`Options.Env` adds to it, `Options.PathWithout` takes folders off its PATH), and never changes the process's, so tests that use a harness may call `t.Parallel()`; a test that calls `t.Setenv` or changes a package-level variable stays serial. `Options` also shortens the waits a test would sit out: `PollEvery`, `StopSettle`, `RetryFirst` and `ConfigTick`. A test takes one of a few slots for its harnesses, files under the temp folder that a lock marks, until it ends, so the test processes of one `go test ./...` never run more tests with apps at once than the machine can serve; `Eventually` gives the app 30 seconds, since a test that waits longer than it needs to is the failure. A test checks what reached Claude there, or in the chat log for a draft, not in the pane. The browser tests skip when Chrome for Testing is not installed; CI points `ORCHESTRATOR_BROWSER` at its Google Chrome. The agent-browser test runs the real `agent-browser` only when `ORCHESTRATOR_AGENT_BROWSER` names it: run `make agent-browser` and set it to `bin/agent-browser`. Run it before a bump of `AGENT_BROWSER_VERSION`, since orchestrator writes a file of agent-browser's own. `desktop` embeds `frontend/dist`; build it first, or run with `-tags stub`, which embeds a small page instead. The host and the tools' server in the tests run the `orchestrator` command built from `./desktop` with that tag: the app's binary is the command. A thread's state follows its host's stream, so a test of the state sends the stand-in a message that makes the stream it needs (`ask`, `tool`, `question`, `work`, `background`, `fails`, `exit`; the comment on `ClaudeDir` lists them); `Harness.Observe` gives a thread a change of state in the names of Claude's hooks, for a test that is not about the stream. The `subcommand` tests answer from a fake app on a temp control socket. The release check never reaches GitHub: the harness answers curl with `Options.Releases` (the release notes of a version with `Options.Notes`, a failed read with `Options.NotesErr`) and refuses every other download.

## Recorded agents

`internal/chat/testdata/` holds what the real Claude wrote on its stdout, and `README.md` there says how each file was made. `hooks.jsonl` is one run with every kind of turn, beside `hooks-payloads.jsonl`, what Claude's hooks got in the same run: the app no longer runs hooks, and `desktop/chat`'s `TestStateFollowsTheStream` follows the stream of that run to the state each point of it shows. After an update of `claude`, record the files again as the README says, with the isolation env above set and `claude` on a cheap model in a throwaway folder; the diff shows what the agent changed, and the tests what that breaks.

## The real front end, in a browser

`ORCHESTRATOR_HTTP` serves the embedded front end over HTTP with a shim for `window.go` and `window.runtime`: `POST /__call/<pkg>.Service/<Method>` with a JSON array of arguments and the header `X-Orchestrator: 1`, and server-sent events at `/__events`. The address must be a loopback one (`127.0.0.1:PORT`, `[::1]:PORT` or `localhost:PORT`); any other is refused at start. The server answers only requests addressed to its own listen address and refuses a foreign `Origin` with 403. Each run makes a random token and prints it once on stderr, as the address to open (`http://127.0.0.1:PORT/?k=<token>`). `/__call/`, `/__events` and `/term/` answer 403 without it, in the header `X-Orchestrator-Token` or the query value `k`; the page reads it from its address and the shim sends it. A thread that opens the plain address in its own browser gets a page that cannot call anything.

```sh
make -o desktop-frontend desktop-app          # or: make desktop-app (rebuilds the front end)
cp bin/orchestrator-dev.app/Contents/MacOS/orchestrator $S/orchestrator   # a bare copy: it is never the .app, so nothing opens a second app
(cd / && ORCHESTRATOR_HTTP=127.0.0.1:18772 $S/orchestrator 2> $S/app.log) &
APP=$!
sleep 2; K=$(sed -n 's/.*[?]k=\([0-9a-f]*\).*/\1/p' $S/app.log | head -1)
curl -s -XPOST -H 'X-Orchestrator: 1' -H "X-Orchestrator-Token: $K" 127.0.0.1:18772/__call/projects.Service/Snapshot -d '[]'
```

A thread's tools reach the same app as `orchestrate` requests on its control socket, each with the thread's token (see [The agent tools](#the-agent-tools)). A shell (`ShellNew`) runs `$SHELL -l`; set `SHELL=/bin/sh` and a throwaway `HOME` in tests so no profile of the owner runs. Stop it with `kill $APP` (SIGTERM) so it stops its browsers, and never by name. Use a port of your own, from 18000 to 18999.

## The agent tools

A thread calls the orchestrator tools (`note`, `issue`, `new_thread`, `show`, `track`, `refresh`, `browser` and the orchestration tools) through the `orchestrator mcp` server its `<id>.mcp.json` starts, which sends each call as `orchestrate` with the thread's token. The stand-in `claude` calls no tool, so the tests in `desktop/orchestration` make the calls themselves: `useTool` sends `orchestrate` with the token read from the thread's MCP config, and `TestMCPServerServesTheAgentToolsAsTheThread` runs the built `orchestrator mcp` from that config and speaks JSON-RPC to it. A test of a tool checks the effect as the caller (the note in the caller's project, the branch on the caller's row) and that a call whose request or input names another thread still acts as the caller. `internal/host` tests which tool asks the owner: a Read thread asks for `show` and `track`.

Claude signs in through its own config folder, so a check with a real Claude writes its transcript into `~/.claude`; a throwaway `CLAUDE_CONFIG_DIR` is not signed in. To check with one, run the app in a browser as above, with the isolation env, and a demo project whose `.claude/settings.local.json` is `{"model": "haiku"}`; start a Read thread and send "note that the build is slow". The chat shows an `mcp__orchestrator__note` call with no approval card, and the note shows in the Notes view. Ask the same thread to show some text as evidence: the chat shows an approval card for `mcp__orchestrator__show`.

## Driving the page

`scripts/cdp.mjs` opens a page in its own headless Chrome for Testing (its own temp profile, never the owner's) and runs steps: wait, click at a point, type, key, eval JavaScript, screenshot. It starts that Chrome on the fixed DevTools port 9333 with one profile folder, so two runs at once, yours and another agent's, would share them. Run a private copy with a port and a profile name of your own:

```sh
sed -e 's/const port = 9333/const port = 9411/' -e "s/orchestrator-cdp-profile/orchestrator-cdp-profile-9411/" scripts/cdp.mjs > $S/cdp.mjs
ORCHESTRATOR_HTTP_TOKEN=$K node $S/cdp.mjs http://127.0.0.1:18772/ 1400 900 \
  '[{"wait":1500},{"eval":"document.title"},{"click":[700,400]},{"wait":300},{"shot":"orc-shot.png"}]'
```

A shot path that is not absolute lands in `$TMPDIR`. Read the screenshot to check what the page shows. Resize with the width and height arguments to check the layouts.

## Releases

`make release VERSION=v1.2.3` builds `bin/orchestrator-darwin-arm64.zip` and its `.sha256` the way the release workflow does, with the version stamped in the bundle's Info.plist. To try the installer or the app's update without a real release, serve `bin/` over HTTP and point them at it with a throwaway `HOME`; never run either against the owner's `~/Applications` or `~/.local/bin`.

## What needs a human

The real Wails window: the title bar, drag and resize, native menus and shortcuts, the folder picker, the dock icon, clipboard through the system, and a launch from Finder (its PATH and locale). The browser mode covers everything else.
