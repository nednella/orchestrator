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
projects:
  - name: storefront
    directory: /Users/me/code/storefront
    # everything below is optional; the values shown are the defaults
    queue_sections: []         # the groups of the queue, in order; see below
    note_session_command: ""   # typed into a session started from a note; {text} is the note; "" types the note itself
    session_prompt_send: auto  # auto sends what a session starts with at once (Claude gets it as a message through its host); manual leaves it as a draft in the thread's chat, and so does each PR wake (`pr_review_command`, `pr_checks_command`, `pr_conflict_command`). The settings panel writes it
    session_branch_fallback: "" # branch of an issue's work, for an issue session that reports none; {n} is the issue number; "" names none
    session_cleanup_command: "" # shell command that cleans up the git side of a session; {branch}, {worktree}, {dir}, {force} (--force when forced, else empty), e.g. "{ [ -z {worktree} ] || git worktree remove {force} {worktree}; } && git branch -D {branch}"; "" leaves branches and folders alone
    session_cleanup_mode:      # does the app clean up by itself? The settings panel writes it
      merge: auto              # auto or manual: after the pull request merged; auto needs the session to have seen it open
      close: manual            # auto or manual: after the pull request closed unmerged; auto needs the session to have seen it open
    browser_start_url: ""      # page a session's browser opens first
    browser_enabled: true      # give sessions the browser tools, and tell them about the browser. The settings panel writes it
    digest_schedule: weekly    # weekly or off. The settings panel writes it
    keep_mac_awake: true       # this project's choice; unset follows the top-level one
    pr_watch_method: ""        # webhook or poll; "" tries gh webhook forward (needs the gh extension and admin rights on the repo) and polls when it does not work
    pr_poll_interval: 10s      # how often to poll the pull requests; at least 1s
    pr_review_command: ""      # typed into a session whose PR got a review or comment, {n} the PR number, e.g. "/address-review {n}"; "" sends nothing. An approval and the gh user's own comments and replies are not counted; a session in the middle of a turn gets it when the turn ends
    pr_review_drafts: false    # send pr_review_command while the PR is a draft too; when false, comments on a draft count as seen
    pr_checks_command: ""      # typed into a session whose PR has failing checks, {n} the PR number; "" sends nothing
    pr_conflict_command: ""    # typed into a session whose PR conflicts with its base branch, {n} the PR number; "" sends nothing
    setup_dismissed: false     # saved by DismissSetup; the UI no longer writes it
```

`session_cleanup_command` runs through `sh -c` in the project folder, once per branch of the session that has a worktree or a local branch, after the safety checks. `{branch}`, `{worktree}` (the folder the branch is checked out in, `''` if none) and `{dir}` (the project folder) are shell-quoted. `{force}` is `--force` only when you force a clean-up, so `git worktree remove {force} {worktree}` removes a worktree with changes only then. A branch with no worktree has `{worktree}` `''`, which `git worktree remove` rejects; the command that *Set up* writes skips that step then. A command that fails blocks the clean-up with its error, unless the branch is already gone.

A session that works on more than one branch is cleaned up a branch at a time. When the PR of a branch it moved on from merges or closes, the command runs for that branch only and the session keeps working; `session_cleanup_mode` decides as for any PR. When the PR of its newest branch merges or closes, its other branches and the session go too, unless one of those branches still has an open PR.

The text that the app types on its own, an issue action's `command`, the set-up session's start and the `pr_*_command`s, starts with `APPLICATION_PROMPT: `, so the agent knows it is not from you. A slash command is typed as it is, since Claude Code runs one only at the start of the prompt. A session gets this text as a message through its host, a pull request action's sentence too.

### Queue sections and actions

`queue_sections` groups the queue and says what a session can start from each group. Each section matches issues by label and lists actions.

```yaml
queue_sections:
  - name: Inbox
    labels: []                  # issues with no labels
    actions:
      - {name: Plan, command: "/plan {n}", model: opus, effort: high, kind: chat}
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
- In a command, `{n}` is the issue number, `{title}` its title and `{branch}` its `session_branch_fallback`. The `Start` action names the branch when the project sets one. An empty `command` starts the agent with nothing typed. `model` and `effort` are optional and are checked by claude, as below; a config with a value claude would not take does not load.
- `kind` is `task` (the default) or `chat`. A chat action starts a read-only chat for the issue: it gets `AGENTOS_ISSUE`, but no branch, no pull request commands and no clean-up. An issue can have one live task and one live chat; starting an action of a kind that already runs on the issue opens that session. The queue row links to the task.
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

### Keeping the Mac awake

While at least one session is working, in any project whose `keep_mac_awake` is on (the project's own setting, else the top-level one, else on), the app runs `caffeinate -i -w <app pid>`: the Mac does not idle-sleep and the display still may. It does not stop a closed lid from sleeping the Mac unless the Mac is in clamshell mode (external display and power connected), and it does not stop a manual sleep or a sleep from low battery.

### Where things are stored

`<key>` is the project name, lower-cased, with other characters as `-`.

| What | Where |
|---|---|
| notes, note images, evidence, digest | `<data_dir>/<key>/` (`data_dir` defaults to `~/.local/share/agentos`) |
| the event log, which holds all stats: one file of events per machine per day, gzipped from the day before yesterday | `<data_dir>/<key>/events/YYYY-MM-DD.<machine>.jsonl[.gz]` |
| PR tracking, clean-up log, browser profile, session temp folders, chat logs (`chats/`) | `~/.local/share/agentos/<key>/` |
| state files, sockets (each Claude session's host under `hosts/<first 8 hex characters of the key's SHA-256>/`), tmux config, last project, release check, this machine's id (`machine-id`) | `~/.local/state/agentos` |

## Command line

`agentos` with no command opens the app. `agentos --help` lists the commands by group. Every command below except `settle` without a number, `browser help`, `version` and `update` talks to the running app, and exits 1 with "agentos is not running: open the app and try again" when there is none. The project is `AGENTOS_PROJECT` (set in the app's shell), else the asking session's, else the current one; the app switches to it.

| Group | Command | Does |
|---|---|---|
| Work | `issue <n...> [--prompt <text>\|-]` | starts a session per issue; an issue with a live session is reported as already running. `--prompt` takes one issue: its text (`-` reads stdin), at most 6 KB, is added to the new session's system prompt as a handover from the session that asked, and a running session does not get it |
| Work | `new [title] [--chat] [--prompt <text>\|-]`, `settle <n>`, `open <n\|title>`, `next` | start, settle, show a session, or show the one that needs you most. `--chat` starts a chat instead of a task. `--prompt` gives its text (`-` reads stdin), at most 6 KB, to the new session, sent unless `session_prompt_send` is `manual`; from a session, the text is framed as that session's handover |
| Work | `refresh`, `pr [n]`, `cleanup [n]` | reload issues and PRs, show PRs, clean up or list what waits |
| Views | `queue`, `notes`, `evidence`, `chat`, `browser`, `digest`, `stats --open`, `filter [query]` | show that view; `digest --run` starts a run |
| Projects | `project [name]`, `project add [path]`, `project remove <name>` | list, switch, add (the current folder by default), forget |
| From inside a session | `issue <n> --prompt -`, `new <title> [--chat] --prompt -`, `browser open <url>`, `browser screenshot`, `browser close`, `show <file> [--caption …] \| --text …`, `note <text>`, `track --branch <name>` (not the default branch) | `issue` and `new` hand work to a new task, or a chat with `--chat`, as under Work; `agentos browser help` explains the browser |

`stats [--days N] [--json]` prints the interruption tally. `agentos settle` without a number settles this project's threads directly through tmux, with the app closed too, and keeps their state files, so the app shows them as settled; `--all` does it for every project and stops the shells. Neither touches a session that ran in a terminal before the host; the command says how many it left. `agentos version` prints the version. `agentos update` installs the latest release over the installed app, then asks a running app to relaunch. Every command typed at a terminal says on stderr when a newer release is out, asking GitHub at most once a day.

## Environment

| Variable | Does |
|---|---|
| `AGENTOS_CONFIG` | the config file to use |
| `AGENTOS_BROWSER` | the browser to drive, as a path; by default Chrome for Testing in `~/Applications` or `/Applications`, which `install.sh` installs. agentos never falls back to another browser: macOS sends clicked links to any running copy of your default browser's app, and one that agentos runs has no window to show them |
| `AGENTOS_AGENT_BROWSER` | the `agent-browser` that sessions drive their browser tab with, as a path; by default the one in the app bundle's `Contents/Helpers`. Without one, sessions start with no browser |
| `AGENTOS_DIR` | the project folder to start in, instead of the current folder |
| `AGENTOS_TMUX_SOCKET`, `AGENTOS_STATE_DIR`, `AGENTOS_DEV_DATA_DIR` | the tmux socket, the state folder, and one folder in place of both data folders; for tests |
