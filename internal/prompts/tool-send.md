<!-- placeholders: none -->

Send a message to a thread. `thread` is its id and `text` is the message. By default the thread takes it in at its next step. With `steer` true, the thread stops its running turn and gets the message now; use that only when the thread is on the wrong path. You can send to a thread whose access is not above your own, never to one above it. A message cannot answer a thread's approval request.
