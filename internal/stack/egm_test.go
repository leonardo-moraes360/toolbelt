package stack

import (
	"reflect"
	"testing"
)

func TestEgmLast(t *testing.T) {
	got := egmLast([]Op{OpEgm, OpVerify, OpGuard, OpEgm, OpReview})
	want := []Op{OpVerify, OpGuard, OpReview, OpEgm}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("egmLast = %v, want %v", got, want)
	}
	if got := egmLast([]Op{OpVerify}); !reflect.DeepEqual(got, []Op{OpVerify}) {
		t.Fatalf("no egm: %v", got)
	}
	if _, err := ParseOp("egm"); err != nil {
		t.Fatal(err)
	}
	if isParallelOp(OpEgm) {
		t.Fatal("egm must not be parallel")
	}
}
