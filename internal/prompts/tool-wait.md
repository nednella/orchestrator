<!-- placeholders: none -->

Wait until a thread finishes its turn, then return its last reply. `thread` is its id. `timeout_seconds` is how long to wait, 1 to 3600 (default 300); when it passes, the tool returns and says the thread still runs. A child's result comes to you as a message when it finishes, so use `wait` only when you have nothing else to do until then. You cannot wait on yourself or on a thread that started you.
