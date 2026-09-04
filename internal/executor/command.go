package executor

import (
	"strings"

	"memkv/internal/logger"
	"memkv/internal/proto"
	"memkv/internal/storage"
)

// Exec runs one command given as CSP-decoded verb + args and returns a CSP value.
func (e *Executor) Exec(args []string) proto.Value {
	if len(args) == 0 {
		return proto.ErrorMsg("ERR empty command")
	}

	cmd := strings.ToUpper(args[0])
	switch cmd {
	case "PING":
		if len(args) == 1 {
			return proto.Simple("PONG")
		}
		if len(args) == 2 {
			return proto.Bulk(args[1])
		}
		return proto.ErrorMsg("ERR wrong number of arguments for PING")
	case "ECHO":
		if len(args) != 2 {
			return proto.ErrorMsg("ERR wrong number of arguments for ECHO")
		}
		return proto.Bulk(args[1])
	case "SET":
		return e.handleSet(args)
	case "GET":
		return e.handleGet(args)
	case "DELETE", "DEL":
		return e.handleDelete(args)
	case "EXISTS":
		return e.handleExists(args)
	case "KEYS":
		return e.handleKeys()
	default:
		return proto.Errorf("ERR unknown command '%s'", cmd)
	}
}

func (e *Executor) handleSet(args []string) proto.Value {
	if len(args) < 3 {
		return proto.ErrorMsg("ERR wrong number of arguments for SET")
	}
	key := args[1]
	value := strings.Join(args[2:], " ")
	if err := e.storage.Set(key, value); err != nil {
		logger.Error("SET failed: %v", err)
		return proto.Errorf("ERR %v", err)
	}
	return proto.Simple("OK")
}

func (e *Executor) handleGet(args []string) proto.Value {
	if len(args) != 2 {
		return proto.ErrorMsg("ERR wrong number of arguments for GET")
	}
	value, err := e.storage.Get(args[1])
	if err == storage.ErrKeyNotFound {
		return proto.Null()
	}
	if err != nil {
		logger.Error("GET failed: %v", err)
		return proto.Errorf("ERR %v", err)
	}
	return proto.Bulk(value)
}

func (e *Executor) handleDelete(args []string) proto.Value {
	if len(args) != 2 {
		return proto.ErrorMsg("ERR wrong number of arguments for DEL")
	}
	if err := e.storage.Delete(args[1]); err != nil {
		if err == storage.ErrKeyNotFound {
			return proto.Integer(0)
		}
		logger.Error("DELETE failed: %v", err)
		return proto.Errorf("ERR %v", err)
	}
	return proto.Integer(1)
}

func (e *Executor) handleExists(args []string) proto.Value {
	if len(args) != 2 {
		return proto.ErrorMsg("ERR wrong number of arguments for EXISTS")
	}
	if e.storage.Exists(args[1]) {
		return proto.Integer(1)
	}
	return proto.Integer(0)
}

func (e *Executor) handleKeys() proto.Value {
	keys := e.storage.Keys()
	items := make([]proto.Value, len(keys))
	for i, k := range keys {
		items[i] = proto.Bulk(k)
	}
	return proto.Array(items...)
}
