package state

type State struct {
	s uint64
}

func (s *State) SetState(state uint64) {
	s.s = s.s | state
}

func (s *State) IsState(state uint64) bool {
	return (s.s & state) == state
}

func (s *State) RemState(state uint64) {
	s.s = s.s & (^state)
}
