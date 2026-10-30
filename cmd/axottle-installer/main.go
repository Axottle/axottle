package main

import (
	"fmt"
	"os"

	"github.com/axottle/axottle/internal/installui"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: axottle-installer <verified installer script> [installer options]")
		os.Exit(2)
	}
	if err := installui.Run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
