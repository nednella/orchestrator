# Thread chat

## What exists

A thread whose session runs under a host (`session.hosted`) shows its chat in the centre instead of its terminal: a column of the user's messages (right, in a raised box), Claude's replies as markdown, and small centred lines for notices, work, approvals, questions and plans (the cards for these come later). The list is virtualised. A thread opens at the bottom, follows a streaming reply while the reader is at the bottom (within 48 px), and stops following when the reader scrolls up; a "Jump to latest" pill then shows once new items arrive below. Each thread keeps where it was left, in memory, for the latest 20 threads. A session without the mark, an old terminal session, has no chat: its thread view shows `This thread ran in a terminal before agentos showed chats` as its empty state.

## How a user reaches it

Click a hosted thread in the left sidebar. In the mock only `#444 checkout route` is hosted. Selecting it puts the focus in the chat's scroll area, so PageUp, PageDown and the arrow keys scroll it.

## How to drive it

`npx vite` in `desktop/frontend` serves the front end on `mock.ts`; add `?longchat` for a thread of about 500 items, and `?chatcases` to make `Where do we set the cache headers?` a hosted thread with an empty chat and `#326 cart rounding` one whose chat log cannot be read. The scroll element is `section[data-panel="terminal"] .overflow-y-auto`, its rows carry `data-index`, the pill is `.chat-jump`. In `vite dev`, `(await import('/src/api.ts')).api.chatSend(id, text)` streams a mock reply over a second (it denies the waiting approval first).

## What goes wrong

- The start-up toast at the bottom can cover the pill: hide or close it before a screenshot.
- Opening a thread under strict mode must still land at the bottom: the virtualizer writes its old offset when it takes its scroll element back.
- A scroll set from code fires a scroll event like a user's, so setting `scrollTop` up stops the follow.
- Check all three themes.
