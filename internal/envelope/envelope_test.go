package envelope

import (
	"encoding/json"
	"testing"
)

func TestStampObjectPutsVFirst(t *testing.T) {
	raw, err := Stamp(Guard, map[string]any{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"v":"guard/1.0","a":1}` {
		t.Fatalf("got %s", raw)
	}
}

func TestStampEmptyObject(t *testing.T) {
	raw, err := Stamp(Guard, struct{}{})
	if err != nil || string(raw) != `{"v":"guard/1.0"}` {
		t.Fatalf("got %s, %v", raw, err)
	}
}

func TestStampKeepsExistingV(t *testing.T) {
	raw, err := Stamp(Guard, map[string]any{"v": "bus/0.1"})
	if err != nil || string(raw) != `{"v":"bus/0.1"}` {
		t.Fatalf("got %s, %v", raw, err)
	}
}

func TestStampWrapsLists(t *testing.T) {
	raw, err := Stamp(TicketList, []int{1, 2})
	if err != nil || string(raw) != `{"v":"ticket-list/1.0","tickets":[1,2]}` {
		t.Fatalf("got %s, %v", raw, err)
	}
	raw, err = Stamp(TicketList, []int(nil))
	if err != nil || string(raw) != `{"v":"ticket-list/1.0","tickets":[]}` {
		t.Fatalf("nil list: got %s, %v", raw, err)
	}
	if _, err := Stamp(Guard, []int{1}); err == nil {
		t.Fatal("a list with no list key must be an error")
	}
}

func TestToolKindsAreValid(t *testing.T) {
	for tool, kind := range ToolKinds {
		if !ValidKind(kind) {
			t.Errorf("%s: invalid kind %q", tool, kind)
		}
	}
}

func TestNewErrorShape(t *testing.T) {
	b, _ := json.Marshal(NewError("boom", "usage", 2))
	if string(b) != `{"v":"error/1.0","error":"boom","reason":"usage","exit_code":2}` {
		t.Fatalf("got %s", b)
	}
}
