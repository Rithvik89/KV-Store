package command

import (
	"strconv"
	"strings"

	"memkv/internal/logger"
	"memkv/internal/proto"
	"memkv/internal/storage"
)

func cmdPing(_ *Executor, args []string) proto.Value {
	if len(args) == 1 {
		return proto.Simple("PONG")
	}
	return proto.Bulk(args[1])
}

func cmdEcho(_ *Executor, args []string) proto.Value {
	return proto.Bulk(args[1])
}

func cmdSet(e *Executor, args []string) proto.Value {
	key := args[1]
	value := strings.Join(args[2:], " ")
	if err := e.storage.Set(key, value); err != nil {
		logger.Error("SET failed: %v", err)
		return proto.Errorf("ERR %v", err)
	}
	return proto.Simple("OK")
}

func cmdGet(e *Executor, args []string) proto.Value {
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

func cmdDel(e *Executor, args []string) proto.Value {
	if err := e.storage.Delete(args[1]); err != nil {
		if err == storage.ErrKeyNotFound {
			return proto.Integer(0)
		}
		logger.Error("DELETE failed: %v", err)
		return proto.Errorf("ERR %v", err)
	}
	return proto.Integer(1)
}

func cmdExists(e *Executor, args []string) proto.Value {
	if e.storage.Exists(args[1]) {
		return proto.Integer(1)
	}
	return proto.Integer(0)
}

func cmdKeys(e *Executor, _ []string) proto.Value {
	keys := e.storage.Keys()
	items := make([]proto.Value, len(keys))
	for i, k := range keys {
		items[i] = proto.Bulk(k)
	}
	return proto.Array(items...)
}

func cmdExpire(e *Executor, args []string) proto.Value {
	seconds, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return proto.ErrorMsg("ERR value is not an integer or out of range")
	}
	return proto.Integer(int64(e.storage.Expire(args[1], seconds)))
}

func cmdTTL(e *Executor, args []string) proto.Value {
	return proto.Integer(e.storage.TTL(args[1]))
}

func cmdPersist(e *Executor, args []string) proto.Value {
	return proto.Integer(int64(e.storage.Persist(args[1])))
}
