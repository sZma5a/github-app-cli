package main

import "os"

func main() {
	os.Exit(run(os.Args, os.Stdin, os.Stderr))
}
