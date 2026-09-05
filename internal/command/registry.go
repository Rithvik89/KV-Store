package command

import (
	"fmt"
	"strings"

	"memkv/internal/info"
	"memkv/internal/logger"
	"memkv/internal/proto"
	"memkv/internal/store"
	"memkv/internal/wal"
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

// Executor is the command registry: verb → handler, plus the backing store.
//
// Named Executor for the Exec entry point (run this command). The package is
// command so call sites read as command.New / commands.Exec.
type Executor struct {
	store    store.IStore
	metrics  *info.Metrics
	log      *logger.Logger
	commands map[string]Command
}

// New creates a command registry with a durable Store and the built-in verbs.
func New(walPath string, opts ...wal.Option) (*Executor, error) {
	m := info.New()
	s, err := store.Open(walPath, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}
	s.AttachMetrics(m)
	e := NewWithStore(s, m)
	e.logger().Info("Recovered %d keys from WAL", s.Size())
	return e, nil
}

// NewWithStore builds a registry around an existing store (tests, alternate backends).
// metrics may be nil.
func NewWithStore(s store.IStore, metrics ...*info.Metrics) *Executor {
	var m *info.Metrics
	if len(metrics) > 0 {
		m = metrics[0]
	}
	if m == nil {
		m = info.New()
	}
	if st, ok := s.(*store.Store); ok {
		st.AttachMetrics(m)
	}
	e := &Executor{
		store:    s,
		metrics:  m,
		log:      logger.Default(),
		commands: make(map[string]Command),
	}
	e.registerBuiltins()
	return e
}

// SetLogger injects a logger (nil → Default).
func (e *Executor) SetLogger(l *logger.Logger) {
	if l == nil {
		l = logger.Default()
	}
	e.log = l
}

func (e *Executor) logger() *logger.Logger {
	if e.log != nil {
		return e.log
	}
	return logger.Default()
}

// Metrics returns the shared INFO counters.
func (e *Executor) Metrics() *info.Metrics {
	return e.metrics
}

// Close closes the underlying store.
func (e *Executor) Close() error {
	return e.store.Close()
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
	e.register(Command{Name: "EXPIRE", MinArgs: 3, MaxArgs: 3, Fn: cmdExpire})
	e.register(Command{Name: "TTL", MinArgs: 2, MaxArgs: 2, Fn: cmdTTL})
	e.register(Command{Name: "PERSIST", MinArgs: 2, MaxArgs: 2, Fn: cmdPersist})
	e.register(Command{Name: "INFO", MinArgs: 1, MaxArgs: 2, Fn: cmdInfo})
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
	v := cmd.Fn(e, args)
	e.metrics.IncrOp(cmd.Name)
	return v
}
