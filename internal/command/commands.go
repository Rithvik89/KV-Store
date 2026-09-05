package command

import (
	"strconv"
	"strings"

	"memkv/internal/info"
	"memkv/internal/proto"
	"memkv/internal/store"
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
	if err := e.store.Set(key, value); err != nil {
		e.logger().Error("SET failed: %v", err)
		return proto.Errorf("ERR %v", err)
	}
	return proto.Simple("OK")
}

func cmdGet(e *Executor, args []string) proto.Value {
	value, err := e.store.Get(args[1])
	if err == store.ErrKeyNotFound {
		return proto.Null()
	}
	if err != nil {
		e.logger().Error("GET failed: %v", err)
		return proto.Errorf("ERR %v", err)
	}
	return proto.Bulk(value)
}

func cmdDel(e *Executor, args []string) proto.Value {
	if err := e.store.Delete(args[1]); err != nil {
		if err == store.ErrKeyNotFound {
			return proto.Integer(0)
		}
		e.logger().Error("DELETE failed: %v", err)
		return proto.Errorf("ERR %v", err)
	}
	return proto.Integer(1)
}

func cmdExists(e *Executor, args []string) proto.Value {
	if e.store.Exists(args[1]) {
		return proto.Integer(1)
	}
	return proto.Integer(0)
}

func cmdKeys(e *Executor, _ []string) proto.Value {
	keys := e.store.Keys()
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
	return proto.Integer(int64(e.store.Expire(args[1], seconds)))
}

func cmdTTL(e *Executor, args []string) proto.Value {
	return proto.Integer(e.store.TTL(args[1]))
}

func cmdPersist(e *Executor, args []string) proto.Value {
	return proto.Integer(int64(e.store.Persist(args[1])))
}

func cmdInfo(e *Executor, args []string) proto.Value {
	section := ""
	if len(args) > 1 {
		section = args[1]
	}
	walBytes, err := e.store.WALBytes()
	if err != nil {
		e.logger().Error("INFO wal size: %v", err)
		walBytes = 0
	}
	body := e.metrics.Format(section, info.Snapshot{
		Keys:     e.store.Size(),
		WALBytes: walBytes,
	})
	return proto.Bulk(body)
}
