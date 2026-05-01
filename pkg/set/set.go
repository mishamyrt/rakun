package set

// Set represents unique set of unique values
type Set[T comparable] map[T]struct{}

// Contains returns true if the set contains the given key
func (s Set[T]) Contains(key T) bool {
	_, v := s[key]
	return v
}

// Append adds the given keys to the set
func (s Set[T]) Append(keys ...T) {
	for _, key := range keys {
		s[key] = struct{}{}
	}
}

// Remove deletes the given key from the set
func (s Set[T]) Remove(key T) {
	delete(s, key)
}

// Values returns a slice of all the values in the set
func (s Set[T]) Values() []T {
	values := make([]T, 0, len(s))
	for key := range s {
		values = append(values, key)
	}
	return values
}

// New returns a new empty set
func New[T comparable]() Set[T] {
	return make(Set[T])
}
