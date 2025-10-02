package executor

import (
	"fmt"
	"strings"

	"memkv/internal/logger"
	"memkv/internal/storage"
)

// ProcessCommand processes a command and returns the response
func (e *Executor) ProcessCommand(input string) string {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "ERROR: empty command"
	}

	cmd := strings.ToUpper(parts[0])

	switch cmd {
	case "SET":
		return e.handleSet(parts)
	case "GET":
		return e.handleGet(parts)
	case "DELETE", "DEL":
		return e.handleDelete(parts)
	case "EXISTS":
		return e.handleExists(parts)
	case "KEYS":
		return e.handleKeys()
	case "PING":
		return "PONG"
	default:
		return fmt.Sprintf("ERROR: unknown command '%s'", cmd)
	}
}

func (e *Executor) handleSet(parts []string) string {
	if len(parts) < 3 {
		return "ERROR: SET requires key and value"
	}

	key := parts[1]
	value := strings.Join(parts[2:], " ")

	if err := e.storage.Set(key, value); err != nil {
		logger.Error("SET failed: %v", err)
		return fmt.Sprintf("ERROR: %v", err)
	}

	return "OK"
}

func (e *Executor) handleGet(parts []string) string {
	if len(parts) < 2 {
		return "ERROR: GET requires key"
	}

	key := parts[1]
	value, err := e.storage.Get(key)
	if err == storage.ErrKeyNotFound {
		return "(nil)"
	}
	if err != nil {
		logger.Error("GET failed: %v", err)
		return fmt.Sprintf("ERROR: %v", err)
	}

	return value
}

func (e *Executor) handleDelete(parts []string) string {
	if len(parts) < 2 {
		return "ERROR: DELETE requires key"
	}

	key := parts[1]
	if err := e.storage.Delete(key); err != nil {
		if err == storage.ErrKeyNotFound {
			return "0"
		}
		logger.Error("DELETE failed: %v", err)
		return fmt.Sprintf("ERROR: %v", err)
	}

	return "1"
}

func (e *Executor) handleExists(parts []string) string {
	if len(parts) < 2 {
		return "ERROR: EXISTS requires key"
	}

	key := parts[1]
	if e.storage.Exists(key) {
		return "1"
	}
	return "0"
}

func (e *Executor) handleKeys() string {
	keys := e.storage.Keys()
	if len(keys) == 0 {
		return "(empty list)"
	}
	return strings.Join(keys, "\n")
}
