# Thread composer

## What exists

Under a hosted thread's chat sits its composer, the same editor as Home's. Enter sends. While a turn runs, a message is queued and shows above the box with "Send now", and with nothing typed the send button is a red Stop that turns grey ("Stopping") until the turn ends or 10 s pass. While an approval, a question or a plan waits, the placeholder says what a message does to it, and the button reads "Deny and send". While a send is unanswered the button spins, for at most 15 s; a send that fails puts its text back, after anything typed since. Esc in the chat or the composer stops a running turn once; on an idle thread it leaves the composer. The model and effort show read-only at the right. The text is kept per thread in local storage. A PR button on a hosted thread's PR tab puts its sentence in the composer, unsent, and focuses it; agentos drafts (`manual` sending, PR wakes) land there too. A settled thread shows "This thread has settled" and Resume instead of the box.

## How a user reaches it

Click a hosted thread in the left sidebar. The PR buttons are on the Details panel's PR tab of a thread whose PR needs attention (`#444 checkout route` in the mock).

## How to drive it

`npx vite` in `desktop/frontend`, with `?chatcases` so `Where do we set the cache headers?` is an idle hosted thread. In the mock a message that says "slow" streams its reply over 8 s, which leaves time to queue another, press Esc or click Stop; a message that is just "exit" ends the thread. `#444 checkout route` waits on a Bash approval. Click into the editor (`.composer-editor`) before typing; the Stop button is `[data-stop]`. To hold or fail a send, replace `chatSend` on the `api` object from `await import('/src/api.ts')`.

## What goes wrong

- The mock ends a turn at once on a stop, so "Stopping" lasts a frame: read the button right after the click.
- The editor itself marks every Esc handled, so a handler that skips handled keys never sees it.
- A settled row cannot be selected from the sidebar; a thread that settles while open shows the settled view.
- Check all three themes: the Stop fill is the danger colour.
