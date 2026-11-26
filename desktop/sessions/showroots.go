package sessions

import (
	"path/filepath"

	"github.com/nednella/agentos/internal/session"
)

// ShowRoots are the folders a thread may file evidence from: its own folder, its project's folder and its own temp
// folder, where its screenshots land. A folder that is not known is left out.
func (s *Sessions) ShowRoots(id string) []string {
	name, err := session.ParseName(id)
	if err != nil {
		return nil
	}
	var roots []string
	if dir, err := s.threadFolder(id); err == nil {
		roots = append(roots, dir)
	}
	s.mu.Lock()
	in, ok := s.infoOf(id)
	projs := s.projects()
	s.mu.Unlock()
	if ok {
		if proj, found := projectIn(projs, in); found && filepath.IsAbs(proj.Dir) {
			roots = append(roots, proj.Dir)
		}
	}
	return append(roots, s.tmpDir(name))
}
