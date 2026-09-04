package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"memkv/internal/proto"
)

const (
	defaultHost = "localhost"
	defaultPort = 9573
)

func main() {
	host := flag.String("host", defaultHost, "server hostname")
	port := flag.Int("port", defaultPort, "server port")
	raw := flag.Bool("raw", false, "print CSP wire bytes for replies")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Println("Cinder CLI")
	fmt.Printf("Connecting to %s... ", addr)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		fmt.Println("failed")
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("ok")
	printHelp()

	client := &cspClient{conn: conn, raw: *raw}
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("cinder> ")
		line, err := in.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println()
				return
			}
			fmt.Fprintf(os.Stderr, "read: %v\n", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		upper := strings.ToUpper(line)
		switch upper {
		case "EXIT", "QUIT":
			fmt.Println("bye")
			return
		case "HELP":
			printHelp()
			continue
		}

		args := splitLine(line)
		if len(args) == 0 {
			continue
		}
		// SET key rest-of-line-is-value (matches server ≥3 arity with joined value).
		if strings.EqualFold(args[0], "SET") && len(args) >= 3 {
			args = []string{args[0], args[1], strings.Join(args[2:], " ")}
		}

		reply, err := client.roundTrip(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		fmt.Println(formatReply(reply, *raw))
	}
}

func printHelp() {
	fmt.Println(`Commands (CSP to server):
  PING [msg]
  ECHO <msg>
  SET <key> <value>   (value may contain spaces)
  GET <key>
  DEL <key>
  EXISTS <key>
  KEYS
Local:
  HELP   EXIT / QUIT
Flags: -host -port -raw`)
}

func splitLine(line string) []string {
	return strings.Fields(line)
}

type cspClient struct {
	conn    net.Conn
	raw     bool
	pending []byte
}

func (c *cspClient) roundTrip(args []string) (proto.Value, error) {
	items := make([]proto.Value, len(args))
	for i, a := range args {
		items[i] = proto.Bulk(a)
	}
	req := proto.EncodeValue(proto.Array(items...))
	if _, err := c.conn.Write(req); err != nil {
		return proto.Value{}, err
	}
	return c.readValue()
}

func (c *cspClient) readValue() (proto.Value, error) {
	buf := make([]byte, 4096)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := c.conn.SetReadDeadline(deadline); err != nil {
			return proto.Value{}, err
		}
		v, rest, ok, err := proto.Decode(c.pending)
		if err != nil {
			c.pending = nil
			return proto.Value{}, err
		}
		if ok {
			c.pending = rest
			return v, nil
		}
		n, rerr := c.conn.Read(buf)
		if n > 0 {
			c.pending = append(c.pending, buf[:n]...)
			continue
		}
		if rerr != nil {
			return proto.Value{}, rerr
		}
	}
}

func formatReply(v proto.Value, raw bool) string {
	if raw {
		return string(proto.EncodeValue(v))
	}
	switch v.Kind {
	case proto.KindSimple:
		return v.Str
	case proto.KindError:
		return v.Str
	case proto.KindInteger:
		return fmt.Sprintf("%d", v.Int)
	case proto.KindBulk:
		return v.Str
	case proto.KindNull:
		return "(nil)"
	case proto.KindArray:
		if len(v.Arr) == 0 {
			return "(empty array)"
		}
		var b strings.Builder
		for i, item := range v.Arr {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(formatReply(item, false))
		}
		return b.String()
	default:
		return string(proto.EncodeValue(v))
	}
}
