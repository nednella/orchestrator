package digest

import (
	"github.com/nednella/orchestrator/desktop/notes"
)

// Service is bound to the front end.
type Service struct{ m *Manager }

func NewService(m *Manager) *Service { return &Service{m: m} }

// Digest is the current project's digest.
func (s *Service) Digest() Digest { return s.m.view() }

// RunDigest starts a run in the background; progress arrives as digest events.
func (s *Service) RunDigest() error { return s.m.runCurrent() }

// DigestToNote saves the item as a note and marks it saved.
func (s *Service) DigestToNote(itemID string) (notes.Note, error) { return s.m.toNote(itemID) }

// DismissDigestItem removes an item.
func (s *Service) DismissDigestItem(itemID string) error { return s.m.dismiss(itemID) }
