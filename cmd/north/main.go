package main

import (
	"fmt"
	"os"

	"github.com/SBakolis/north/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Run(os.Stdin, os.Stdout, os.Stderr, os.Args[1:], version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
