package update

import "context"

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

// ReleaseNotes reads the notes of the running version from its release.
func (s *Service) ReleaseNotes() (Release, error) { return s.u.ReleaseNotes(s.ctx()) }

// NewReleaseNotes is the notes of the running version, once, when the app has not shown them before; else empty.
func (s *Service) NewReleaseNotes() []Release { return s.u.NewReleaseNotes(s.ctx()) }
