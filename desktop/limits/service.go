package limits

import (
	"context"
	"errors"
)

// Service is the plan limits for the front end, which hears of each change as the limits event.
type Service struct {
	t   *Tracker
	ctx func() context.Context
}

func NewService(t *Tracker, ctx func() context.Context) *Service { return &Service{t: t, ctx: ctx} }

// Limits are the plan limits as last read.
func (s *Service) Limits() Limits { return s.t.Limits() }

// RefreshLimits reads the plan limits now, as the owner asked: from a live thread, else by a probe when the values are
// over 5 minutes old. A read that fails is the answer's error, not the call's.
func (s *Service) RefreshLimits() (Limits, error) {
	l, err := s.t.Refresh(s.ctx())
	if err != nil {
		return l, errors.New("The app is stopping")
	}
	return l, nil
}
