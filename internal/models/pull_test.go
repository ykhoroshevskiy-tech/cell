package models_test

import (
	"encoding/json"
	"testing"

	"github.com/ykhoroshevskiy-tech/cell/internal/models"
)

func TestPullOptionsJSON(t *testing.T) {
	opts := models.PullOptions{Dest: "/tmp/out", DryRun: true, Delete: false}
	data, err := json.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	var back models.PullOptions
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Dest != opts.Dest || !back.DryRun || back.Delete {
		t.Fatalf("roundtrip failed: %+v", back)
	}
}
