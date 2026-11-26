package terminal

import (
	"net/http"
	"strings"
)

// Prefix is where a shell's output is served: GET /term/<shell id>.
const Prefix = "/term/"

// appHeader marks a read the front end sends. A page loads a same-origin <img> or <script> without it, so such a tag
// cannot take the stream over, in the window or in the browser mode.
const appHeader = "X-Orchestrator"

// Handler serves the output of each open shell as one long response of raw bytes, written as it comes. A shell has
// one reader: a second GET ends the first. The body ends when the shell exits or its terminal closes.
func (t *Terms) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "only GET", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get(appHeader) != "1" {
			http.Error(w, "missing "+appHeader+" header", http.StatusForbidden)
			return
		}
		st := t.get(strings.TrimPrefix(r.URL.Path, Prefix))
		if st == nil {
			http.NotFound(w, r)
			return
		}
		rd, repaint := st.attach()
		defer st.detach(rd)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		flush(w)
		if repaint {
			t.repaint(st)
		}
		for {
			ended := false
			select {
			case <-rd.gone:
				return
			case <-r.Context().Done():
				return
			case <-st.ended:
				ended = true
			case <-rd.wake:
			}
			out, lost := st.take(rd)
			if lost {
				// What is left after a drop may start inside an escape sequence; the paint replaces it, as at attach.
				if !ended {
					t.repaint(st)
				}
			} else if len(out) > 0 {
				// The window's response never ends its request's context; a write to a request it stopped fails.
				if _, err := w.Write(out); err != nil {
					return
				}
				flush(w)
			}
			if ended {
				return
			}
		}
	})
}

// flush sends what was written at once. The window's response writer has no Flush: it hands each write on as it comes.
func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
