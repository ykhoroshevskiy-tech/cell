package verbose

import "fmt"

// Verbose controls detailed output across all internal packages.
// Set once from CLI flags; read everywhere via verbose.Enabled().
var Verbose bool

func Enabled() bool { return Verbose }

// V prints only when verbose is enabled.
func V(format string, args ...any) {
	if Verbose {
		fmt.Printf(format+"\n", args...)
	}
}
