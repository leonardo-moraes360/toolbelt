package ticket

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestViewJSONCancelledKey(t *testing.T) {
	want := "cancel\u006ced"
	b, err := json.Marshal(View{Canceled: []Ticket{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"`+want+`":`) {
		t.Fatalf("json missing %s key: %s", want, b)
	}
	if string(StatusCancelled) != want {
		t.Fatalf("status=%q", StatusCancelled)
	}
}
