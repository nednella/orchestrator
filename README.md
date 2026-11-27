<div align="center">
  <img src="docs/icon.png" alt="orchestrator" width="128">
  <h3><b>orchestrator</b></h3>
  <p>One desktop window for every coding-agent thread you run</p>
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

The script puts `orchestrator.app` in `~/Applications`, and Chrome for Testing, the agent browser, when it is not there yet; one that is there stays as it is. It then lists anything missing. No sudo. Another app that uses `Google Chrome for Testing.app` shares only that program with orchestrator, and nothing else.

## Start

1. Open `orchestrator`.
2. Add a project: _+_ in the project rail.
3. Start a thread: `⌘M`, or `Enter` on an issue in the queue to open it, then start one from it.
4. Ask about the project without changing it: `⌘N` starts a read-only thread. It reads the project and changes nothing; it can file notes and issues.

`⌘/` lists every shortcut.

## Set up a project

Right-click the project in the rail → _Set up project_. For a project with no queue sections it adds an Inbox whose _Work_ action runs `/work <n>`, plus the issue branch name and the PR commands. Then it starts a thread that writes the `/work` command, the issue template and `AGENTS.md` with you.

To change the config by hand, edit `~/.config/orchestrator/config.yaml`; the app picks the change up while it runs. `docs/config.md` describes every key.

## Update

Click the update button at the bottom of the sidebar. It checks, then downloads, then asks before it installs and restarts. Threads keep running in their hosts.

Releases are signed with one certificate, so macOS keeps the permissions you gave orchestrator across updates. The first signed release asks for each permission one last time. To clear the entries that earlier releases left behind, run `tccutil reset All io.github.nednella.orchestrator`, then approve the prompts once more.

## Build

Needs Go, Node and `tmux`. `make desktop-app` builds the front end and the app into `bin/orchestrator-dev.app`; `VERSION=v1.2.3` stamps a version. `make release` builds `bin/orchestrator-darwin-arm64.zip` and its `.sha256`. Run the tests with `go test -race -tags stub ./...` and `cd desktop/frontend && npm test`, with the isolation variables from `docs/testing.md` set. `AGENTS.md` describes how the app works.
