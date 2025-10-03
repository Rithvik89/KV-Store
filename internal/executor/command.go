package executor

import (
	"fmt"
	"strings"

	"memkv/internal/logger"
	"memkv/internal/storage"

	"github.com/tidwall/resp"
)

func (e *Executor) ProcessKVSPCommand(v resp.Value) resp.Value {
	if v.Type() != resp.Array {
		return resp.ErrorValue(fmt.Errorf("ERROR: expected array of command and arguments"))
	}

	command := v.Array()[0].String()
	command = strings.ToUpper(command)

	switch command {
	case "SET":
		return e.handleSet(v)
	case "GET":
		return e.handleGet(v)
	case "DELETE":
		return e.handleDelete(v)
	case "PING":
		return resp.SimpleStringValue("PONG")
	default:
		return resp.ErrorValue(fmt.Errorf("ERROR: unknown command '%s'", command))
	}
}

func (e *Executor) handleSet(v resp.Value) resp.Value {
	if len(v.Array()) < 3 {
		return resp.ErrorValue(fmt.Errorf("ERROR: SET requires key and value"))
	}

	key := v.Array()[1].String()
	value := v.Array()[2].String()

	if err := e.storage.Set(key, value); err != nil {
		logger.Error("SET failed: %v", err)
		return resp.ErrorValue(fmt.Errorf("ERROR: %v", err))
	}

	return resp.SimpleStringValue("OK")
}

func (e *Executor) handleGet(v resp.Value) resp.Value {
	if len(v.Array()) < 2 {
		return resp.ErrorValue(fmt.Errorf("ERROR: GET requires key"))
	}

	key := v.Array()[1].String()
	value, err := e.storage.Get(key)
	if err == storage.ErrKeyNotFound {
		return resp.NullValue()
	}
	if err != nil {
		logger.Error("GET failed: %v", err)
		return resp.ErrorValue(fmt.Errorf("ERROR: %v", err))
	}

	return resp.StringValue(value)
}

func (e *Executor) handleDelete(v resp.Value) resp.Value {
	if len(v.Array()) < 2 {
		return resp.ErrorValue(fmt.Errorf("ERROR: DELETE requires key"))
	}

	key := v.Array()[1].String()
	if err := e.storage.Delete(key); err != nil {
		if err == storage.ErrKeyNotFound {
			return resp.IntegerValue(0)
		}
		logger.Error("DELETE failed: %v", err)
		return resp.ErrorValue(fmt.Errorf("ERROR: %v", err))
	}

	return resp.IntegerValue(1)
}
