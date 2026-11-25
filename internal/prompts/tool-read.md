<!-- placeholders: none -->

Read what a thread did. `thread` is its id. `view` is `messages` (what the user and the agent said, the default) or `activity` (the tools it ran). It returns the newest entries, in the order they happened. `before` is a sequence number: it returns the entries before it, so you can page back. `limit` is how many to return, 1 to 100 (default 20).
