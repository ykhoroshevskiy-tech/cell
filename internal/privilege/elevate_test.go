package privilege

import (
	"reflect"
	"testing"
)

func TestSudoArgList(t *testing.T) {
	got := sudoArgList("/usr/bin/cell",
		[]string{"HOME=/root", "PATH=/usr/bin", "CELL_DATA_DIR=/data", "CELL_QUIET=1"},
		[]string{"bootstrap", "--force"})
	want := []string{"env", "CELL_DATA_DIR=/data", "CELL_QUIET=1", "/usr/bin/cell", "bootstrap", "--force"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSudoArgListNoCellEnv(t *testing.T) {
	got := sudoArgList("/usr/bin/cell", []string{"HOME=/root"}, []string{"rescue"})
	want := []string{"env", "/usr/bin/cell", "rescue"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSudoArgListNoArgs(t *testing.T) {
	got := sudoArgList("/usr/bin/cell", nil, nil)
	want := []string{"env", "/usr/bin/cell"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
