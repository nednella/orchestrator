<div align="center">
  <img src="docs/icon.png" alt="orchestrator" width="128">
  <h3><b>orchestrator</b></h3>
  <p>One desktop window for every coding-agent session you run</p>
  <p>
    <a href="https://github.com/nednella/orchestrator/releases/latest"><img src="https://img.shields.io/github/v/release/nednella/orchestrator?style=flat-square&color=a78bfa" alt="Latest release"></a>
    <img src="https://img.shields.io/badge/macOS-Apple%20silicon-191d25?style=flat-square&logo=apple" alt="macOS on Apple silicon">
  </p>
</div>

<br>

![orchestrator](docs/screenshot.png)

<br>

## Install

Needs macOS on Apple silicon, plus:

- [tmux](https://github.com/tmux/tmux): `brew install tmux`
- [GitHub CLI](https://cli.github.com), logged in: `brew install gh && gh auth login`; 2.99 or later to attach pictures when a note becomes an issue
- [Claude Code](https://docs.claude.com/en/docs/claude-code)

```sh
curl -fsSL https://raw.githubusercontent.com/nednella/orchestrator/main/install.sh | bash
```

The script puts `orchestrator.app` and Chrome for Testing, the agent browser, in `~/Applications`, then lists anything missing. No sudo.

## Start

1. Open `orchestrator`.
2. Add a project: _+_ in the project rail.
3. Start a task: `⌘M`, or `Enter` on an issue in the queue to open it, then start one from it.
4. Ask about the project without starting work: `⌘N` starts a chat. A chat reads the project and changes nothing; it files an issue or hands off to a task when the talk turns into work. Chats stay out of the stats.

`⌘/` lists every shortcut.

## Set up a project

Right-click the project in the rail → _Set up project_. For a project with no queue sections it adds an Inbox whose _Work_ action runs `/work <n>`, plus branch, clean-up and PR commands. Then it starts a session that writes the `/work` command, the issue template and `AGENTS.md` with you.

To change the config by hand, edit `~/.config/orchestrator/config.yaml`; the app picks the change up while it runs. `docs/config.md` describes every key.

## Update

Click `Update` in the top bar. Sessions keep running.

Releases are signed with one certificate, so macOS keeps the permissions you gave orchestrator across updates. The first signed release asks for each permission one last time. To clear the entries that earlier releases left behind, run `tccutil reset All io.github.nednella.orchestrator`, then approve the prompts once more.
