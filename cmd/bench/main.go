package main

import (
	"flag"
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"memkv/internal/proto"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9573", "server address")
	n := flag.Int("n", 1000, "total operations")
	c := flag.Int("c", 4, "concurrent clients")
	workload := flag.String("workload", "mixed", "get | set | mixed")
	flag.Parse()

	if *n <= 0 || *c <= 0 {
		fmt.Fprintln(os.Stderr, "-n and -c must be positive")
		os.Exit(1)
	}
	wl := strings.ToLower(*workload)
	switch wl {
	case "get", "set", "mixed":
	default:
		fmt.Fprintln(os.Stderr, "-workload must be get, set, or mixed")
		os.Exit(1)
	}

	latencies := make([]time.Duration, *n)
	var next atomic.Int64
	var errs atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < *c; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", *addr, 3*time.Second)
			if err != nil {
				fmt.Fprintf(os.Stderr, "dial: %v\n", err)
				errs.Add(int64(*n)) // fail loudly
				return
			}
			defer conn.Close()

			for {
				i := int(next.Add(1) - 1)
				if i >= *n {
					return
				}
				op := pickOp(wl, i)
				key := fmt.Sprintf("bench:%d", i%1000)
				t0 := time.Now()
				if err := roundTrip(conn, op, key); err != nil {
					errs.Add(1)
					continue
				}
				latencies[i] = time.Since(t0)
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// Drop zero latencies from failed ops for percentile calc.
	samples := make([]time.Duration, 0, *n)
	for _, d := range latencies {
		if d > 0 {
			samples = append(samples, d)
		}
	}
	if len(samples) == 0 {
		fmt.Fprintln(os.Stderr, "no successful operations")
		os.Exit(1)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })

	p50 := percentile(samples, 50)
	p99 := percentile(samples, 99)
	var sum time.Duration
	for _, d := range samples {
		sum += d
	}
	mean := sum / time.Duration(len(samples))

	fmt.Printf("addr=%s workload=%s n=%d c=%d errors=%d\n", *addr, wl, *n, *c, errs.Load())
	fmt.Printf("wall=%s ops/s=%.0f\n", elapsed.Round(time.Millisecond), float64(len(samples))/elapsed.Seconds())
	fmt.Printf("latency mean=%s p50=%s p99=%s\n", mean, p50, p99)
}

func pickOp(workload string, i int) string {
	switch workload {
	case "get":
		return "GET"
	case "set":
		return "SET"
	default:
		if i%2 == 0 {
			return "SET"
		}
		return "GET"
	}
}

func roundTrip(conn net.Conn, op, key string) error {
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	var req proto.Value
	switch op {
	case "SET":
		req = proto.Array(proto.Bulk("SET"), proto.Bulk(key), proto.Bulk("v"))
	case "GET":
		req = proto.Array(proto.Bulk("GET"), proto.Bulk(key))
	default:
		return fmt.Errorf("unknown op %s", op)
	}
	if _, err := conn.Write(proto.EncodeValue(req)); err != nil {
		return err
	}
	buf := make([]byte, 0, 256)
	tmp := make([]byte, 256)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if _, _, ok, derr := proto.Decode(buf); derr != nil {
				return derr
			} else if ok {
				return nil
			}
		}
		if err != nil {
			return err
		}
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := (p / 100) * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	weight := rank - float64(lo)
	return time.Duration(float64(sorted[lo])*(1-weight) + float64(sorted[hi])*weight)
}
