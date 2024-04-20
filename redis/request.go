package redis

import "fmt"

// Req - convenient wrapper to create Request.
func Req(cmd string, args ...interface{}) Request {
	return Request{cmd, nil, args, nil}
}

// ReqFromByteArgs creates a Request from a command, list of args in byte form, and
// uses the provided buf to store the Request.EncodedBytes
func ReqFromByteArgs(cmd string, args [][]byte, buf []byte) Request {
	space := -1
	for i, c := range cmd {
		if c == ' ' {
			space = i
			break
		}
	}

	if space == -1 {
		buf = appendHead(buf, '*', len(args)+1)
		buf = appendHead(buf, '$', len(cmd))
		buf = append(buf, cmd...)
		buf = append(buf, '\r', '\n')
	} else {
		buf = appendHead(buf, '*', len(args)+2)
		buf = appendHead(buf, '$', space)
		buf = append(buf, cmd[:space]...)
		buf = append(buf, '\r', '\n')
		buf = appendHead(buf, '$', len(cmd)-space-1)
		buf = append(buf, cmd[space+1:]...)
		buf = append(buf, '\r', '\n')
	}

	for _, arg := range args {
		buf = appendHead(buf, '$', len(arg))
		buf = append(buf, arg...)
		buf = append(buf, '\r', '\n')
	}

	if cmd == "RANDOMKEY" {
		return Request{Cmd: cmd, Args: nil, EncodedBytes: buf}
	}

	var n int
	switch cmd {
	case "FCALL", "FCALL_RO":
		n = 2
	case "EVAL", "EVALSHA":
		n = 2
	case "BITOP":
		n = 1
	default:
		n = 0
	}

	return Request{Cmd: cmd, KeyBytes: args[n], Args: nil, EncodedBytes: buf}
}

// Request represents request to be passed to redis.
type Request struct {
	// Cmd is a redis command to be sent.
	// It could contain single space, then it will be split, and last part will be serialized as an argument.
	Cmd      string
	KeyBytes []byte

	// Either specify Args to encode or pre-encoded request via EncodedBytes
	Args         []interface{}
	EncodedBytes []byte
}

func (r Request) String() string {
	args := r.Args
	if len(args) > 5 {
		args = args[:5]
	}
	argss := make([]string, 0, 1+len(args))
	for _, arg := range args {
		argStr := fmt.Sprintf("%v", arg)
		if len(argStr) > 32 {
			argStr = argStr[:32] + "..."
		}
		argss = append(argss, argStr)
	}
	if len(r.Args) > 5 {
		argss = append(argss, "...")
	}
	return fmt.Sprintf("Req(%q, %q)", r.Cmd, argss)
}

// Key returns first field of request that should be used as a key for redis cluster.
func (r Request) Key() (string, bool) {
	if r.Cmd == "RANDOMKEY" {
		return "RANDOMKEY", false
	}

	if r.KeyBytes != nil {
		return string(r.KeyBytes), true
	}

	var n int
	switch r.Cmd {
	case "FCALL", "FCALL_RO":
		n = 2
	case "EVAL", "EVALSHA":
		n = 2
	case "BITOP":
		n = 1
	default:
		n = 0
	}
	if len(r.Args) <= n {
		return "", false
	}
	return ArgToString(r.Args[n])
}

var rKey = []byte("RANDOMKEY")
var eKey = []byte("")

func (r Request) KeyByte() ([]byte, bool) {
	if r.Cmd == "RANDOMKEY" {
		return rKey, false
	}

	if r.KeyBytes != nil {
		return r.KeyBytes, true
	}

	var n int
	switch r.Cmd {
	case "FCALL", "FCALL_RO":
		n = 2
	case "EVAL", "EVALSHA":
		n = 2
	case "BITOP":
		n = 1
	default:
		n = 0
	}
	if len(r.Args) <= n {
		return eKey, false
	}

	key := r.Args[n]
	if k, ok := key.([]byte); ok {
		return k, true
	}

	ks, ok := ArgToString(key)
	return []byte(ks), ok
}

// Future is interface accepted by Sender to signal request completion.
type Future interface {
	// Resolve is called by sender to pass result (or error) for particular request.
	// Single future could be used for accepting multiple results.
	// n argument is used then to distinguish request this result is for.
	Resolve(res interface{}, n uint64)
	// Cancelled method could inform sender that request is abandoned.
	// It is called usually before sending request, and if Cancelled returns non-nil error,
	// then Sender calls Resolve with ErrRequestCancelled error wrapped around returned error.
	Cancelled() error
}

// FuncFuture simple wrapper that makes Future from function.
type FuncFuture func(res interface{}, n uint64)

// Cancelled implements Future.Cancelled (always false)
func (f FuncFuture) Cancelled() error { return nil }

// Resolve implements Future.Resolve (by calling wrapped function).
func (f FuncFuture) Resolve(res interface{}, n uint64) { f(res, n) }
