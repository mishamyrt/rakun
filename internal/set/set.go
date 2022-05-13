package set

// String represents String Map
type String map[string]bool

func (s *String) Contains(key string) bool {
	_, v := map[string]bool(*s)[key]
	return v
}

func (s *String) Append(key string) {
	map[string]bool(*s)[key] = true
}

func (s *String) Remove(key string) {
	map[string]bool(*s)[key] = false
}

func (s *String) Values() []string {
	values := []string{}
	for key := range map[string]bool(*s) {
		values = append(values, key)
	}
	return values
}

func CreateString(initialData []string) String {
	set := String{}
	for _, value := range initialData {
		set.Append(value)
	}
	return set
}
