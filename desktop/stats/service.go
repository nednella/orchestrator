package stats

import "time"

// Service is bound to the front end.
type Service struct {
	books *Books
}

func NewService(books *Books) *Service {
	return &Service{books: books}
}

// Ledger is what the agents did over the last days days, 0 for all time, in the project whose key is
// given, or in every project for "".
func (s *Service) Ledger(days int, key string) (Ledger, error) {
	return s.books.Ledger(days, key, time.Now())
}
