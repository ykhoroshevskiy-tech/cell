package privilege

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestAmbientPropagationManual(t *testing.T) {
	if os.Getenv("CELL_CAPTEST") != "1" {
		t.Skip("manual captest only")
	}
	RaiseAmbientNetCaps()
	ambSum, effSum := 0, 0
	for i := 0; i < 10; i++ {
		out, err := exec.Command("/bin/sh", "-c", "grep -E 'Cap(Inh|Prm|Eff|Amb)' /proc/self/status").CombinedOutput()
		if err != nil {
			t.Fatalf("child: %v\n%s", err, out)
		}
		fmt.Printf("child %d:\n%s", i, out)
		var amb, eff uint64
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			if strings.HasPrefix(line, "CapAmb:") {
				amb, _ = strconv.ParseUint(fields[len(fields)-1], 16, 64)
			}
			if strings.HasPrefix(line, "CapEff:") {
				eff, _ = strconv.ParseUint(fields[len(fields)-1], 16, 64)
			}
		}
		if amb&0x3000 != 0x3000 || eff&0x3000 != 0x3000 {
			t.Fatalf("child %d missing net caps: CapAmb=%x CapEff=%x", i, amb, eff)
		}
		ambSum++
		effSum++
	}
	if ambSum != 10 || effSum != 10 {
		t.Fatalf("expected 10 children with ambient caps, got amb=%d eff=%d", ambSum, effSum)
	}
}
