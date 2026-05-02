package set

import "testing"

func TestSetAppendContainsAndRemove(t *testing.T) {
	values := New[string]()

	values.Append("alpha", "beta", "alpha")
	if !values.Contains("alpha") {
		t.Fatal("expected alpha to be present")
	}
	if !values.Contains("beta") {
		t.Fatal("expected beta to be present")
	}

	values.Remove("alpha")
	if values.Contains("alpha") {
		t.Fatal("expected alpha to be removed")
	}
	if !values.Contains("beta") {
		t.Fatal("expected beta to remain present")
	}
}

func TestSetValuesReturnsUniqueEntries(t *testing.T) {
	values := New[int]()
	values.Append(1, 2, 2, 3)

	got := values.Values()
	if len(got) != 3 {
		t.Fatalf("unexpected values length: %d", len(got))
	}

	seen := make(map[int]bool, len(got))
	for _, value := range got {
		seen[value] = true
	}

	for _, want := range []int{1, 2, 3} {
		if !seen[want] {
			t.Fatalf("expected %d to be returned, got %#v", want, got)
		}
	}
}
