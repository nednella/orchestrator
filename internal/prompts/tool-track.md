<!-- placeholders: none -->

Tell orchestrator the branch this thread works on. `branch` is its name: the branch checked out in your folder (`git branch --show-current`). It makes the branch this thread's, so the app cleans this thread up when the branch's pull request merges. Call it only for a branch that you made and work on, not for the one the thread started on. orchestrator refuses the default branch, a branch another thread works on, a fork, a child that works in its parent's folder and a Read thread.
