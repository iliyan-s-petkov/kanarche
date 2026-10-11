// Command netprobe is a test-only binary for the docparse sandbox tests
// (invariant I1). It is built into the probe stage of Dockerfile.docparse,
// never into the published image. -listen serves a token, -dial exits 0
// only when it reads that token back.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const token = "kanarche-netprobe\n"

const dialTimeout = 3 * time.Second

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

// run returns 0 when the dial reached a netprobe listener, 1 when it did
// not, 2 on usage errors. -listen does not return while it serves.
func run(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("netprobe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dial := fs.String("dial", "", "host:port of a netprobe listener")
	listen := fs.String("listen", "", "address to serve the token on")
	if err := fs.Parse(args); err != nil || (*dial == "") == (*listen == "") || fs.NArg() > 0 {
		fmt.Fprintln(out, "usage: netprobe -dial host:port | -listen addr")
		return 2
	}
	if *listen != "" {
		ln, err := net.Listen("tcp", *listen)
		if err != nil {
			fmt.Fprintln(out, "listen failed")
			return 1
		}
		serve(ln)
		return 1
	}
	if !reached(*dial) {
		fmt.Fprintln(out, "unreachable")
		return 1
	}
	fmt.Fprintln(out, "reached")
	return 0
}

// serve writes the token to every connection until ln is closed.
func serve(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		c.SetDeadline(time.Now().Add(dialTimeout))
		io.WriteString(c, token)
		c.Close()
	}
}

func reached(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return false
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(dialTimeout))
	line, err := bufio.NewReader(io.LimitReader(c, 64)).ReadString('\n')
	return err == nil && line == token
}
