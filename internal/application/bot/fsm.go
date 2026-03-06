package botapp

import (
	"sync"
)

type trackStep uint8

const (
	trackIdle trackStep = iota
	trackWaitURL
	trackWaitTags
)

type trackDraft struct {
	url string
}

type chatTrackState struct {
	step  trackStep
	draft trackDraft
}

type fsmStore struct {
	mu sync.Mutex
	m  map[int64]*chatTrackState
}

func newFSMStore() *fsmStore {
	return &fsmStore{m: make(map[int64]*chatTrackState)}
}

func (s *fsmStore) get(chatID int64) *chatTrackState {
	st, ok := s.m[chatID]
	if !ok {
		st = &chatTrackState{step: trackIdle}
		s.m[chatID] = st
	}
	return st
}

func (s *fsmStore) reset(chatID int64) {
	delete(s.m, chatID)
}
