package indexedheap

import "testing"

func TestQueueUpsertPeekAndLen(t *testing.T) {
	q := New[string, string]()

	q.Upsert("later", 20, "later-value")
	q.Upsert("first", 10, "first-value")

	if got := q.Len(); got != 2 {
		t.Fatalf("expected len 2, got %d", got)
	}
	key, priority, value, ok := q.Peek()
	if !ok {
		t.Fatal("expected queue to have a head item")
	}
	if key != "first" || priority != 10 || value != "first-value" {
		t.Fatalf("unexpected head item key=%q priority=%d value=%q", key, priority, value)
	}
}

func TestQueueUpsertExistingKeyUpdatesPriorityAndValue(t *testing.T) {
	q := New[string, string]()

	q.Upsert("a", 30, "old")
	q.Upsert("b", 20, "b")
	q.Upsert("a", 10, "new")

	key, priority, value, ok := q.Peek()
	if !ok {
		t.Fatal("expected queue to have a head item")
	}
	if key != "a" || priority != 10 || value != "new" {
		t.Fatalf("expected updated key a at head, got key=%q priority=%d value=%q", key, priority, value)
	}
	if got := q.Len(); got != 2 {
		t.Fatalf("upsert of existing key must not grow queue, got len %d", got)
	}
}

func TestQueueRemoveDeletesIndexedItem(t *testing.T) {
	q := New[string, string]()
	q.Upsert("a", 10, "a")
	q.Upsert("b", 5, "b")

	if !q.Remove("b") {
		t.Fatal("expected remove to report true for existing key")
	}
	if q.Remove("missing") {
		t.Fatal("expected remove to report false for missing key")
	}
	key, _, value, ok := q.Peek()
	if !ok {
		t.Fatal("expected queue to have remaining item")
	}
	if key != "a" || value != "a" {
		t.Fatalf("expected remaining key a, got key=%q value=%q", key, value)
	}
	if _, ok := q.Get("b"); ok {
		t.Fatal("removed key must not be returned by Get")
	}
}

func TestQueueValuesAtOrBeforeDoesNotRemoveItems(t *testing.T) {
	q := New[string, string]()
	q.Upsert("future", 30, "future")
	q.Upsert("past", 10, "past")
	q.Upsert("due", 20, "due")

	values := q.ValuesAtOrBefore(20)
	want := []string{"past", "due"}
	if !sameStrings(values, want) {
		t.Fatalf("expected values %v, got %v", want, values)
	}
	if got := q.Len(); got != 3 {
		t.Fatalf("ValuesAtOrBefore must not remove items, got len %d", got)
	}
}

func TestQueueValuesAtOrBeforeReturnsPriorityOrder(t *testing.T) {
	q := New[string, string]()
	q.Upsert("first", 1, "first")
	q.Upsert("third", 3, "third")
	q.Upsert("second", 2, "second")

	values := q.ValuesAtOrBefore(3)
	want := []string{"first", "second", "third"}
	if !sameStrings(values, want) {
		t.Fatalf("expected priority ordered values %v, got %v", want, values)
	}
}

func TestQueueValuesAtOrBeforePreservesInsertionOrderForSamePriority(t *testing.T) {
	q := New[string, string]()
	q.Upsert("first", 10, "first")
	q.Upsert("second", 10, "second")
	q.Upsert("third", 10, "third")

	values := q.ValuesAtOrBefore(10)
	want := []string{"first", "second", "third"}
	if !sameStrings(values, want) {
		t.Fatalf("expected stable values %v, got %v", want, values)
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
