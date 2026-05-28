//go:build js && wasm

package storage

import "fmt"

type Store struct{}

func Open(path string) (*Store, error) {
	return nil, fmt.Errorf("sqlite storage is not supported on js/wasm: %s", path)
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) ListTodos() ([]Todo, error) {
	return nil, fmt.Errorf("sqlite storage is not supported on js/wasm")
}

func (s *Store) AddTodo(title string) error {
	return fmt.Errorf("sqlite storage is not supported on js/wasm")
}

func (s *Store) UpdateTodo(id int64, title string) error {
	return fmt.Errorf("sqlite storage is not supported on js/wasm")
}

func (s *Store) DeleteTodo(id int64) error {
	return fmt.Errorf("sqlite storage is not supported on js/wasm")
}

func (s *Store) SetCompleted(id int64, completed bool) error {
	return fmt.Errorf("sqlite storage is not supported on js/wasm")
}
