package set

// String represents String Map
type String map[string]struct{}

func (s String) Contains(key string) bool {
	_, v := (s)[key]
	return v
}

func (s String) Append(key string) {
	s[key] = struct{}{}
}

func (s String) Remove(key string) {
	delete(s, key)
}

func (s String) Values() []string {
	values := make([]string, 0, len(s))
	for key := range s {
		values = append(values, key)
	}
	return values
}

func NewString(initialData []string) String {
	set := make(String, len(initialData))
	for _, value := range initialData {
		set.Append(value)
	}
	return set
}
