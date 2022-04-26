package redis

import "fmt"

var (
	rKey = []byte("RANDOMKEY")
	eKey = []byte("")
)

// Req - convenient wrapper to create Request.
func Req(cmd string, args ...interface{}) Request {
	return Request{Cmd: cmd, Raw: nil, RawAppends: 0, Args: args}
}

// Request represents request to be passed to redis.
type Request struct {
	// Cmd is a redis command to be sent.
	// It could contain single space, then it will be split, and last part will be serialized as an argument.
	Cmd        string
	Args       []interface{}
	Raw        []byte
	RawAppends int
	Key        []byte
}

func (r *Request) SetKey(key []byte) {
	r.Key = key
	r.AppendBytes(key)
}

func (r *Request) AppendBytes(arg []byte) {
	r.RawAppends++
	r.Raw = appendHead(r.Raw, '$', len(arg))
	r.Raw = append(r.Raw, arg...)
	r.Raw = append(r.Raw, '\r', '\n')
}

func (r *Request) AppendInt(arg int) {
	r.RawAppends++
	r.Raw = appendBulkInt(r.Raw, int64(arg))
	r.Raw = append(r.Raw, '\r', '\n')
}

func (r *Request) AppendInt64(arg int64) {
	r.RawAppends++
	r.Raw = appendBulkInt(r.Raw, arg)
	r.Raw = append(r.Raw, '\r', '\n')
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

func (r *Request) KeyByte() ([]byte, bool) {
	if r.Key != nil {
		return r.Key, true
	}

	if r.Cmd == "RANDOMKEY" {
		return rKey, false
	}
	var n int
	switch r.Cmd {
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

	ks, ok := ArgToString(r.Args[n])
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
