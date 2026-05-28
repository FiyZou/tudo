//go:build !(js && wasm)

package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	s := &Store{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) init() error {
	_, err := s.db.Exec(`
		PRAGMA journal_mode = WAL;
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			completed INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	return nil
}

func (s *Store) ListTodos() ([]Todo, error) {
	rows, err := s.db.Query(`
		SELECT id, title, completed, created_at, updated_at
		FROM todos
		ORDER BY completed ASC, updated_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var t Todo
		var completed int
		var createdRaw, updatedRaw string
		if err := rows.Scan(&t.ID, &t.Title, &completed, &createdRaw, &updatedRaw); err != nil {
			return nil, fmt.Errorf("scan todo: %w", err)
		}

		t.Completed = completed == 1
		t.CreatedAt = parseSQLiteTime(createdRaw)
		t.UpdatedAt = parseSQLiteTime(updatedRaw)
		todos = append(todos, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate todos: %w", err)
	}

	return todos, nil
}

func (s *Store) AddTodo(title string) error {
	_, err := s.db.Exec(`
		INSERT INTO todos (title, completed, created_at, updated_at)
		VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, title)
	if err != nil {
		return fmt.Errorf("add todo: %w", err)
	}
	return nil
}

func (s *Store) UpdateTodo(id int64, title string) error {
	result, err := s.db.Exec(`
		UPDATE todos
		SET title = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, title, id)
	if err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	return requireAffected(result, "todo not found")
}

func (s *Store) DeleteTodo(id int64) error {
	result, err := s.db.Exec(`DELETE FROM todos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	return requireAffected(result, "todo not found")
}

func (s *Store) SetCompleted(id int64, completed bool) error {
	value := 0
	if completed {
		value = 1
	}

	result, err := s.db.Exec(`
		UPDATE todos
		SET completed = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, value, id)
	if err != nil {
		return fmt.Errorf("set todo status: %w", err)
	}
	return requireAffected(result, "todo not found")
}

func requireAffected(result sql.Result, message string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New(message)
	}
	return nil
}

func parseSQLiteTime(value string) time.Time {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
	} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}
