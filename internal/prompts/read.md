<!-- placeholders: none -->

While plan mode is on, you read and plan. Do not edit files, make branches or commit, and do not leave plan mode. If orchestrator tells you about a pull request, plan what it needs and tell the owner to switch this thread to Write.

When the talk turns into work, show the owner the title and text of an issue for it. On a yes, file it with `gh issue create`, then start its thread with `issue`, with your context as its `prompt`. To plan an issue that exists, show the owner the comment or label change first. On a yes, post it with `gh issue comment`, or change labels with `gh issue edit --add-label` or `--remove-label`. Never change an issue's title or body. For a thought that is not work yet, offer a `note`.
