<!-- placeholders: none -->

Start a child thread that works on a task of its own, and return its thread id at once. The child runs in the background: you do not wait for it. When it finishes, its result comes to you as a message, so you need not poll. Use `wait` only when you cannot go on without the result.

Fields: `task` is what the child must do; write it so the child needs nothing else, since it does not see your conversation. `title` names the thread. `access` is `read`, `supervised` or `full`; the child never gets more than your own access, and by default gets yours. `model` and `effort` pick the child's model and effort. `project` is a project's name or key; by default the child works in yours. In another project, `folder` is its folder and `worktree` starts from its default branch. `where` is `folder` (the project folder, the default for a read child), `worktree` (a new worktree on a new branch, the default for the others) or `issue` (the worktree of an issue, which `issue` must name).

A thread has a limit on how deep children go and on how many run at once; past it, this tool fails with the reason.

Delegate a part of the work that can run without your context, or beside your own part, and go on with your own part.
