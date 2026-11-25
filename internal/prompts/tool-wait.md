<!-- placeholders: none -->

Wait until a thread finishes its turn, then return its last reply, as text. `thread` is its id. `timeout_seconds` is how long to wait, 1 to 3600 (default 300); when it passes, the tool returns a short sentence that says the thread still runs, and no reply. A child's result comes to you as a message when it finishes, so use `wait` only when you have nothing else to do until then. You cannot wait on yourself or on a thread that started you.
