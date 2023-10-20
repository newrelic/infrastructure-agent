package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintf(os.Stdout, "stdout line\n")
	fmt.Fprintf(os.Stderr, "%s\n", os.ExpandEnv("VERBOSE=${VERBOSE}"))
}
