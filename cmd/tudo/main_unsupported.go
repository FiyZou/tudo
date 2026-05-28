//go:build js && wasm

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "tudo is a terminal CLI and does not support js/wasm builds")
	os.Exit(1)
}
