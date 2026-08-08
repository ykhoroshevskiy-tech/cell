package main

import (
	"os"
	"runtime"

	"github.com/ykhoroshevskiy-tech/cell/internal/cli"
)

func main() {
	if runtime.GOOS != "linux" {
		println("cell requires Linux with KVM")
		os.Exit(1)
	}
	if len(os.Args) < 2 || (os.Args[1] != "version" && os.Args[1] != "help" && os.Args[1] != "--help" && os.Args[1] != "-h") {
		if os.Geteuid() != 0 {
			println("cell must be run as root — use: sudo cell")
			os.Exit(1)
		}
	}
	if err := cli.Execute(); err != nil {
		println("error:", err.Error())
		os.Exit(1)
	}
}
