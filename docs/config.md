# Config and command line

## Config file

`~/.config/agentos/config.yaml` (`AGENTOS_CONFIG` overrides the path). The app writes it when projects are added or removed, when a setting changes and when a project is set up, and keeps every key below, its comments and its `~` paths. It checks the file every 2 seconds and takes in a hand edit while it runs; a file that does not load shows a warning and leaves the config as it was, and the app does not save over it until it loads again. Before each save the app reads the file again, so a setting changed in the app keeps a hand edit. `data_dir` still takes effect only when the app starts.

```yaml
data_dir: ~/Library/Mobile Documents/com~apple~CloudDocs/agentos   # optional; default ~/.local/share/agentos. The settings panel writes it; the app reads it when it starts
keep_mac_awake: true           # stop the Mac idle-sleeping while a session works; a project may set its own. The settings panel writes it
app_theme: dark                # light or dark; unset follows the macOS appearance. The settings panel writes it
app_theme_dark: black          # the theme the dark appearance shows: black (the default) or violet. The settings panel writes it
app_theme_light: light         # the theme the light appearance shows: light (the default)
app_text_scale: 1.2            # text size as a multiple of the default, 0.48 to 2.08; unset is 1. The settings panel writes it
editor_app: Visual Studio Code # the macOS app that "Open in VS Code" opens a thread's folder with, through open -a; unset is Visual Studio Code
new_thread_place: worktree     # where a thread with no issue starts, at every access: worktree (a new worktree, the default) or project (the project folder, on the branch checked out there); see Where a thread runs. The settings panel writes it
new_thread_access: supervised  # the access the composer starts a new thread with: read, supervised (the default) or full; a thread can change it in the model menu. The settings panel writes it
projects:
  - name: storefront
    directory: /Users/me/code/storefront
    # everything below is optional; the values shown are the defaults
    queue_sections: []         # the groups of the queue, in order; see below
    note_session_command: ""   # typed into a session started from a note; {text} is the note; "" types the note itself
    session_prompt_send: auto  # auto sends what a session starts with at once (Claude gets it as a message through its host); manual leaves it as a draft in the thread's chat, and so does each PR command (`pr_review_command`, `pr_checks_command`, `pr_conflict_command`) and the action's command to an issue's sleeping thread that a start wakes. A sleeping thread is woken under its own id before it gets either. The settings panel writes it
    session_branch_fallback: "" # branch agentos makes for an issue thread; {n} is the issue number; "" means issue-{n}
    session_cleanup_command: "" # shell command that removes the worktree agentos made, the branch stays; {worktree}, {branch}, {dir}, {force} (--force when forced, else empty); "" means git worktree remove {force} {worktree}
    session_worktree_command: "" # shell command that makes a thread's worktree on a new branch; {branch}, {worktree}, {dir}, {base}; "" means git worktree add -b {branch} {worktree} {base}
    session_cleanup_mode:      # does the app clean up by itself? Either way a merge or close of the PR of a thread's branch puts the thread to sleep, and a clean-up keeps the thread, asleep, and its branch. The settings panel writes it
      merge: auto              # auto or manual: after the pull request merged, also while the app was closed
      close: manual            # auto or manual: after the pull request closed unmerged, also while the app was closed
    browser_start_url: ""      # page a session's browser opens first
    browser_enabled: true      # give sessions the browser tools, and tell them about the browser. The settings panel writes it
    digest_schedule: weekly    # weekly or off. The settings panel writes it
    keep_mac_awake: true       # this project's choice; unset follows the top-level one
    pr_watch_method: ""        # webhook or poll; "" tries gh webhook forward (needs the gh extension and admin rights on the repo) and polls when it does not work
    pr_poll_interval: 10s      # how often to poll the pull requests; at least 1s
    pr_review_command: ""      # typed into a session whose PR got a review or comment, {n} the PR number, e.g. "/address-review {n}"; "" sends nothing. An approval and the gh user's own comments and replies are not counted; a session in the middle of a turn gets it when the turn ends, and a settled issue thread is woken for it
    pr_review_drafts: false    # send pr_review_command while the PR is a draft too; when false, comments on a draft count as seen
    pr_checks_command: ""      # typed into a session whose PR has failing checks, {n} the PR number; "" sends nothing
    pr_conflict_command: ""    # typed into a session whose PR conflicts with its base branch, {n} the PR number; "" sends nothing
    setup_dismissed: false     # saved by DismissSetup; the UI no longer writes it
```

`editor_app` is the app the thread menu's and the palette's *Open in VS Code* opens a thread's folder with: `open -a <editor_app> <folder>`, run without a shell. The folder is the thread's own worktree when it has one, else the project folder; a thread whose worktree is gone opens nothing and says so.

`session_worktree_command` runs through `sh -c` in the project folder when agentos starts a thread in a worktree on a branch that is not there yet (see [Where a thread runs](#where-a-thread-runs)). `{branch}` (the issue's branch from `session_branch_fallback`, or one named from the thread's title), `{worktree}` (the folder agentos picks, `<project>/trees/<branch>`), `{dir}` (the project folder) and `{base}` (`origin/<default branch>`, fetched just before, or the local default branch when origin has none) are shell-quoted. The command may put the worktree in another folder: the thread runs in the one git then lists on the branch. A command that fails, or that leaves no worktree on the branch, starts no thread, and the error gives its first line of output. A woken thread whose worktree is gone gets it back at the same path on its own branch with plain `git worktree add`, never with this command.

`session_cleanup_command` runs through `sh -c` in the project folder after the safety checks and removes the worktree agentos made for the thread; the branch stays. It also runs, whatever `session_cleanup_mode` says, when a child thread goes to sleep in a worktree agentos made for it that holds no change and no commit that origin lacks; the child stays, asleep, and its wake puts the worktree back. Every clean-up, and the checks of a delete with the worktree, measures a worktree from the commit it started at (`baseOid` in `worktrees.json`, the parent's commit for a child): one with no commit since then has only its changes to lose, so origin is not fetched, and commits from before it, as a parent's unpushed work, do not keep it. It runs once per worktree, and only while the folder recorded in `worktrees.json` still holds the worktree agentos made for that thread: never on the project folder, git's main worktree, a worktree a human made (also one at a path agentos used before) or one a thread only ran in, forced or not. A thread with no worktree agentos made runs no command and loses only its temp folder and browser window. `{worktree}` (that recorded folder), `{branch}` and `{dir}` are shell-quoted. `{force}` is `--force` only when you force a clean-up, so `git worktree remove {force} {worktree}` removes a worktree with changes only then. The worktree counts as removed once git no longer lists it and its folder is gone, whatever the command's exit code; a command that fails and leaves it blocks the clean-up with `the clean-up command failed: <first line of its error>`, and one that exits 0 and leaves it with `the clean-up command left <path>`.

A clean-up the checks kept after the PR of a sleeping thread's branch merged or closed is tried again, with every check, every hour, and at each start of the app when `session_cleanup_mode` cleans that PR up by itself (`auto`; under `manual` the ask comes back after a restart and stays an ask), so a worktree kept for a running turn, a shell or work not yet pushed goes once that has ended; a clean-up that asks you first is never tried by itself, nor is the clean-up of a thread you woke. *Clean up now* in a thread's right-click menu runs the clean-up at once, with every check: on every kept clean-up, whatever kept it, and on a thread whose worktree is still there while no clean-up runs or asks. *Clean up anyway* forces a kept one.

A `session_cleanup_command` that still holds `git branch -D {branch}` deletes the branch, so remove that part from your configs: the branch now stays.

With `session_branch_fallback` unset, issue threads now work on `issue-<n>`.

The text that the app types on its own, an issue action's `command`, the set-up session's start and the `pr_*_command`s, starts with `APPLICATION_PROMPT: `, so the agent knows it is not from you. A slash command is typed as it is, since Claude Code runs one only at the start of the prompt. A session gets this text as a message through its host, a pull request action's sentence too.

### Queue sections and actions

`queue_sections` groups the queue and says what a session can start from each group. Each section matches issues by label and lists actions.

```yaml
queue_sections:
  - name: Inbox
    labels: []                  # issues with no labels
    actions:
      - {name: Plan, command: "/plan {n}", model: opus, effort: high, access: read}
      - {name: Work, command: "/work {n}"}
  - name: Needs plan
    labels: [plan]
    draggable_from: [Inbox]     # only issues of these sections can be dropped here; unset takes any
    actions:
      - {name: Plan, command: "/plan {n}", model: opus, effort: high}
  - name: Ready
    labels: [ready]
    draggable_from: []          # takes no drops: only a session adds "ready"
    actions:
      - {name: Work, command: "/work {n}"}
  - name: Review
    labels: [review]
    draggable: false            # its issues can not be dragged out; on unless false
    actions:
      - {name: Work, command: "/work {n}"}
  - name: Rest
    labels: ["*"]               # every issue the sections above did not take
    actions:
      - {name: Open, command: ""}
```

- An issue goes in the first section it matches: it has any of the section's `labels`, or the section's `labels` is empty and the issue has none, or the section's `labels` holds `"*"`, which matches every issue.
- So a `"*"` section placed last takes every issue the sections above it did not. A section after it would never be reached, so a config with one does not load.
- An issue no section matches goes under one folded row below the last section, `<n> more issues match no section`, with the `Start` action. A last `"*"` section is only needed to give those issues a name or other actions.
- In a command, `{n}` is the issue number, `{title}` its title and `{branch}` its `session_branch_fallback`, `issue-{n}` when unset. The `Start` action always names the branch. An empty `command` starts the agent with nothing typed. `model` and `effort` are optional and are checked by claude, as below; a config with a value claude would not take does not load.
- `access` is what the action's thread may do without asking: `read` (Claude plans and asks before it changes anything), `supervised` (the default: your Claude allow rules apply and other tools ask in the chat) or `full` (every tool runs without asking). Every thread follows its branch and pull request, whatever its access. An issue has one thread: starting any action of an issue whose thread runs opens that thread, and one whose thread is asleep wakes it with its own access. The queue row links to it. The old `kind` is gone: a config whose action sets `kind` does not load, and says to use `access: read` for a read-only action, so an old `kind: chat` action never starts with write access.
- A session started by an action is titled `#<n> <title>`, with the action's name as a badge beside it.
- The first action of a section is its default. A section needs a name, an action needs a name, and each is unique in its list; a config that breaks this does not load.
- Dragging an issue to another section relabels it on GitHub: it loses the labels of the section it left and gains the first label of the section it enters. No session starts. A section with no labels takes an issue only when no label is left.
- A move can be undone from the app; the app remembers the last move of each issue until it quits.
- `draggable: false` keeps the issues of a section from being dragged out. `draggable_from` lists the sections whose issues may be dropped in; each must be a section other than this one. Issues no section takes count as one section that can be dragged out, and `draggable_from` never lists it.
- A `"*"` section takes no drops, so it can not have `draggable_from`; a config that sets it does not load.
- With no `queue_sections` the queue is one list, and each issue has one action, `Start`, which types `Work on issue #<n>: <title>`. A section with no actions has that same action.

The old `lanes`, `commands`, `models`, `session_model` and `session_effort` keys are gone. They are not read, and the app removes `session_model` and `session_effort` from the file the next time it saves a setting.

The old `agent_command`, an action's `agent` and an issue's `agent:` labels are ignored: a config or an issue that has them still loads, and every thread runs `claude`.

### How a session picks its model

claude gets `--model` and `--effort` from the first of these that sets a value, model and effort each on their own:

1. the issue's `model:<x>` or `effort:<y>` label (a value claude would not accept is ignored, and the queue row shows the label in red, with the reason when you point at it);
2. the action's `model` and `effort`;
3. the agent's own settings, read when the session starts. For claude: `model`, and the effort in `modelSettings.<model>.effortLevel` for the model chosen, else `effortLevel`, from the project folder's `.claude/settings.local.json`, then its `.claude/settings.json`, then `settings.json` in `$CLAUDE_CONFIG_DIR` or `~/.claude`.

claude takes the aliases `default`, `best`, `fable`, `opus`, `sonnet`, `haiku` and `opusplan`, and any full id that starts with `claude-`, each with or without `[1m]`, at an effort of `low`, `medium`, `high`, `xhigh` or `max`. claude has no command that lists its models, so the aliases are fixed in the app and a full id is taken without a list.

A new session, or one started from a note, has no action and no labels. When nothing sets a value, no flag is passed and the agent uses its own default.

The row shows that choice until the agent's first turn. From then on it shows what the agent used last: for claude, the model and effort on the newest line of its transcript, read at each Stop hook, so a `/model` change inside the session shows. The state file keeps them, so a restart of the app shows them without a new turn. The usage events keep the model each turn ran on.

### What a session runs

Every thread runs `claude` under `agentos host`, with hooks that call the `agentos` command on PATH. Both must be on PATH: without `agentos` a thread does not start, with the error "The agentos command is not on PATH, so a thread's hooks cannot report its state", and without `claude` it does not start with "claude is not installed: it is not on PATH". While either is missing the app shows a warning that names it, and clears it once it is found. An app started from Finder gets a short PATH; when it lacks `tmux`, `gh`, `agentos` or `claude`, the app takes the PATH of the login shell.

### Where a thread runs

A thread runs in one folder from its start, and a wake starts it there again.

- An issue thread, at every access, runs in its own worktree on the issue's branch, `issue-<n>` unless `session_branch_fallback` names it otherwise.
- A thread with no issue runs where `new_thread_place` says, at every access. Unset or `worktree`, it runs in its own worktree on a branch named from its title: `Fix the login button!` is `fix-the-login-button`, at most 40 characters, `thread` for a title with no letters or digits, and `fix-the-login-button-2`, `-3`… while a local or origin branch has the name. `project`, it runs in the project folder, on the branch checked out there. This holds for the composer, `agentos new` and a thread started from a note alike. A child runs where `delegate`'s `where` puts it.
- The set-up session runs in the project folder.

The worktree is `<project>/trees/<branch>`. Before it makes the first one, agentos adds `/trees/` to the repository's `.git/info/exclude`, which is never committed, so the worktrees stay out of `git status` and `git add -A`; it never edits a `.gitignore`. A branch that exists, locally or on origin (fetched first when this clone has no copy), is checked out there as it is and never reset; a new one comes from `session_worktree_command`. A branch that a worktree already has checked out, such as one you made by hand, runs the thread in that worktree, and agentos never removes it. A branch that another thread works on, running or asleep, is refused with `Thread <n> works on <branch>`. A folder already at `<project>/trees/<branch>` that is no worktree of the branch stops the start, and agentos leaves it as it is. A project that is not a git repository runs every thread in its folder.

A start that fails after agentos made the worktree removes that worktree with `git worktree remove`, never forced, so one with changes stays; the branch stays too. A fork runs in its thread's folder and has no branch of its own.

A thread's branch is the one agentos made its worktree on, or the one it reported with `agentos track --branch <name>`; the folder the agent works in never changes it. A thread in the project folder has no branch, and so no pull request and no clean-up, until it runs `agentos track`. `track` refuses the default branch, a branch another thread works on (`Thread <n> works on <branch>`), a fork and a Read thread (`A Read thread works on no branch`). When the thread's own worktree has the branch checked out, the record of the worktree follows it.

### Keeping the Mac awake

While at least one session is working, in any project whose `keep_mac_awake` is on (the project's own setting, else the top-level one, else on), the app runs `caffeinate -i -w <app pid>`: the Mac does not idle-sleep and the display still may. It does not stop a closed lid from sleeping the Mac unless the Mac is in clamshell mode (external display and power connected), and it does not stop a manual sleep or a sleep from low battery.

### Where things are stored

`<key>` is the project name, lower-cased, with other characters as `-`.

| What | Where |
|---|---|
| notes, note images, evidence, digest | `<data_dir>/<key>/` (`data_dir` defaults to `~/.local/share/agentos`) |
| the event log, which holds all stats: one file of events per machine per day, gzipped from the day before yesterday | `<data_dir>/<key>/events/YYYY-MM-DD.<machine>.jsonl[.gz]` |
| PR tracking, clean-up log, the worktrees agentos made (`worktrees.json`), browser profile, session temp folders, chat logs (`chats/`) | `~/.local/share/agentos/<key>/` |
| state files, sockets (each Claude session's host under `hosts/<first 8 hex characters of the key's SHA-256>/`), tmux config, last project, release check, this machine's id (`machine-id`) | `~/.local/state/agentos` |

## Command line

`agentos` with no command opens the app. `agentos --help` lists the commands by group. Every command below except `settle` without a number, `browser help`, `version` and `update` talks to the running app, and exits 1 with "agentos is not running: open the app and try again" when there is none. The project is `AGENTOS_PROJECT` (set in the app's shell), else the asking session's, else the current one; the app switches to it.

| Group | Command | Does |
|---|---|---|
| Work | `issue <n...> [--prompt <text>\|-]` | starts a session per issue; an issue with a live session is reported as already running, and one whose thread is asleep has that thread woken and is reported as woken. `--prompt` takes one issue: its text (`-` reads stdin), at most 6 KB, is added to the new or woken thread's system prompt as a handover from the session that asked, in place of the one a woken thread had, and a running session does not get it. From inside a thread that is not Full access, an issue whose action is `access: full` or whose sleeping thread is Full access is refused (`A thread that is not Full access cannot start one that is`); from a shell or a terminal it is allowed |
| Work | `new [title] [--access read\|supervised\|full] [--prompt <text>\|-]`, `settle <n>`, `open <n\|title>`, `next` | start, put to sleep, show a session, or show the one that needs you most. `--access` sets the new thread's access, `supervised` by default; from inside a thread, `full` is refused unless that thread is a live Full access thread (`A thread that is not Full access cannot start one that is`), and from a shell or a terminal it is allowed. `--prompt` gives its text (`-` reads stdin), at most 6 KB, to the new session, sent unless `session_prompt_send` is `manual`; from a session, the text is framed as that session's handover |
| Work | `refresh`, `pr [n]`, `cleanup [n]` | reload issues and PRs, show PRs; `cleanup <n>` cleans up after thread n with every check and prints `Removed <what>`, or fails with `Kept: <reason>`; `cleanup` lists what waits: a clean-up that asks you, one the checks kept (`Kept: <reason>`), which the app tries again every hour only while the thread sleeps after the PR of its branch merged or closed, and one that runs |
| Views | `queue`, `notes`, `evidence`, `chat`, `browser`, `digest`, `stats --open`, `filter [query]` | show that view; `digest --run` starts a run |
| Projects | `project [name]`, `project add [path]`, `project remove <name>` | list, switch, add (the current folder by default), forget |
| From inside a session | `issue <n> --prompt -`, `new <title> [--access read\|supervised] --prompt -`, `browser open <url>`, `browser screenshot`, `browser close`, `show <file> [--caption …] \| --text …`, `note <text>`, `track --branch <name>` (makes it the thread's branch; see [Where a thread runs](#where-a-thread-runs)) | `issue` and `new` hand work to a new thread, a read-only one with `--access read`, as under Work; `agentos browser help` explains the browser |

`stats [--days N] [--json]` prints the interruption tally. `agentos settle` without a number puts this project's threads to sleep directly through tmux, with the app closed too, and keeps their state files, so the app shows them as sleeping and stops their servers, at once when it runs, else at its next start; `--all` does it for every project and stops the shells. Neither touches a session that ran in a terminal before the host; the command says how many it left. `agentos version` prints the version. `agentos update` installs the latest release over the installed app, then asks a running app to relaunch. Every command typed at a terminal says on stderr when a newer release is out, asking GitHub at most once a day.

## Environment

| Variable | Does |
|---|---|
| `AGENTOS_CONFIG` | the config file to use |
| `AGENTOS_BROWSER` | the browser to drive, as a path; by default Chrome for Testing in `~/Applications` or `/Applications`, which `install.sh` installs. agentos never falls back to another browser: macOS sends clicked links to any running copy of your default browser's app, and one that agentos runs has no window to show them |
| `AGENTOS_AGENT_BROWSER` | the `agent-browser` that sessions drive their browser tab with, as a path; by default the one in the app bundle's `Contents/Helpers`. Without one, sessions start with no browser |
| `AGENTOS_DIR` | the project folder to start in, instead of the current folder |
| `AGENTOS_TMUX_SOCKET`, `AGENTOS_STATE_DIR`, `AGENTOS_DEV_DATA_DIR` | the tmux socket, the state folder, and one folder in place of both data folders; for tests |
