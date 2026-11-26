package update

import (
	"context"

	"github.com/nednella/agentos"
)

// Service is bound to the front end.
type Service struct {
	u   *Updater
	ctx func() context.Context
}

func NewService(u *Updater, ctx func() context.Context) *Service { return &Service{u: u, ctx: ctx} }

// UpdateState is where the update stands; each change also comes as the update event.
func (s *Service) UpdateState() State { return s.u.State() }

// CheckUpdate asks GitHub for the latest release.
func (s *Service) CheckUpdate() error { return s.u.Check(s.ctx()) }

// DownloadUpdate fetches the newer release the last check found.
func (s *Service) DownloadUpdate() error { return s.u.Download(s.ctx()) }

// InstallUpdate installs the newest release and relaunches the app.
func (s *Service) InstallUpdate() error { return s.u.Install(s.ctx()) }

// Update is InstallUpdate for the front end that still calls it by this name.
func (s *Service) Update() error { return s.u.Install(s.ctx()) }

// PatchNotes is the releases since the version the app last ran; it returns them once.
func (s *Service) PatchNotes() ([]Release, error) { return s.u.PatchNotes() }

// Changelog is every release the app shipped with, newest first.
func (s *Service) Changelog() []Release { return ParseChangelog(agentos.Changelog) }
