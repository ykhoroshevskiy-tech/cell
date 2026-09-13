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
	if err := cli.Execute(); err != nil {
		println("error:", err.Error())
		os.Exit(1)
	}
}
