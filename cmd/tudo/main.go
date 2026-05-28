//go:build !(js && wasm)

package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FiyZou/tudo/internal/config"
	"github.com/FiyZou/tudo/internal/storage"
	"github.com/FiyZou/tudo/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	store, err := storage.Open(cfg.DatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	program := tea.NewProgram(tui.New(store, cfg), tea.WithAltScreen(), tea.WithoutSignals())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
