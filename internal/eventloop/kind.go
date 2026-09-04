package eventloop

// Kind is a bitset of readiness flags for one file descriptor.
//
// Each bit is an independent yes/no: can we read, can we write, or both.
// Combine flags with |, for example Readable|Writable. Check a flag with Has.
type Kind uint8

const (
	// Readable means the descriptor has data to read (or a listening socket
	// has a connection ready to accept).
	Readable Kind = 1 << iota

	// Writable means the descriptor's send buffer has room for more bytes.
	Writable
)

// Has reports whether flag is set in k.
//
// Example: event.Kind.Has(Readable) is true when the fd is ready to read.
func (k Kind) Has(flag Kind) bool { return k&flag != 0 }

// Event is one readiness notification from the poller.
//
// FD is the file descriptor that became ready. Kind says whether it is
// readable, writable, or both. The loop turns each Event into Handler calls.
type Event struct {
	FD   int
	Kind Kind
}
