package runner

import "testing"

func TestNew(t *testing.T) {
	if New() == nil {
		t.Fatal("expected non-nil Runner from New()")
	}
}
