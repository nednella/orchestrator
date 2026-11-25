<!-- placeholders: none -->

Stop a child thread that you started, or a child of one of your children. `thread` is its id. It interrupts the running turn of that thread and of each of its own children, and drops the messages held for them, so none of them starts a new turn for one. It returns the ids of the threads it stopped and how many messages it dropped. You cannot stop any other thread.
