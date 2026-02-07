package main

import (
	"fmt"
	"os"

	"github.com/the-kernel-panics/k8s-pods-debug/pkg/plugin"
)

func main() {
	if err := plugin.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
