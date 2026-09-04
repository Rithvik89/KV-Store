package executor

import (
	"fmt"
	"strings"

	"memkv/internal/logger"
	"memkv/internal/proto"
	"memkv/internal/storage"
)

// HandlerFunc runs one command. args[0] is the verb; remaining are arguments.
type HandlerFunc func(e *Executor, args []string) proto.Value

// Command is one registered verb in the dispatch table.
type Command struct {
	Name    string
	MinArgs int // inclusive, counting the verb
	MaxArgs int // inclusive; -1 means no upper bound
	Fn      HandlerFunc
}

// Executor handles command execution via a verb → handler registry.
type Executor struct {
	storage  storage.Storage
	commands map[string]Command
}

// New creates an executor with persistent in-memory storage and the built-in commands.
func New(walPath string) (*Executor, error) {
	store, err := storage.NewPersistentStorage(walPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}
	logger.Info("Recovered %d keys from WAL", store.Size())
	return NewWithStorage(store), nil
}

// NewWithStorage builds an executor around an existing store (tests, alternate backends).
func NewWithStorage(store storage.Storage) *Executor {
	e := &Executor{
		storage:  store,
		commands: make(map[string]Command),
	}
	e.registerBuiltins()
	return e
}

// Close closes the underlying storage.
func (e *Executor) Close() error {
	return e.storage.Close()
}

// register adds a command under its Name (upper-cased). Extra aliases share the same Command.
func (e *Executor) register(cmd Command, aliases ...string) {
	cmd.Name = strings.ToUpper(cmd.Name)
	e.commands[cmd.Name] = cmd
	for _, a := range aliases {
		e.commands[strings.ToUpper(a)] = cmd
	}
}

func (e *Executor) registerBuiltins() {
	e.register(Command{Name: "PING", MinArgs: 1, MaxArgs: 2, Fn: cmdPing})
	e.register(Command{Name: "ECHO", MinArgs: 2, MaxArgs: 2, Fn: cmdEcho})
	e.register(Command{Name: "SET", MinArgs: 3, MaxArgs: -1, Fn: cmdSet})
	e.register(Command{Name: "GET", MinArgs: 2, MaxArgs: 2, Fn: cmdGet})
	e.register(Command{Name: "DEL", MinArgs: 2, MaxArgs: 2, Fn: cmdDel}, "DELETE")
	e.register(Command{Name: "EXISTS", MinArgs: 2, MaxArgs: 2, Fn: cmdExists})
	e.register(Command{Name: "KEYS", MinArgs: 1, MaxArgs: 1, Fn: cmdKeys})
}

// Exec looks up args[0] in the dispatch table, checks arity, and runs the handler.
func (e *Executor) Exec(args []string) proto.Value {
	if len(args) == 0 {
		return proto.ErrorMsg("ERR empty command")
	}

	name := strings.ToUpper(args[0])
	cmd, ok := e.commands[name]
	if !ok {
		return proto.Errorf("ERR unknown command '%s'", name)
	}
	if len(args) < cmd.MinArgs || (cmd.MaxArgs >= 0 && len(args) > cmd.MaxArgs) {
		return proto.Errorf("ERR wrong number of arguments for '%s'", cmd.Name)
	}
	return cmd.Fn(e, args)
}
