package redis

import (
	"bufio"
	"bytes"
	"github.com/joomcode/redispipe/redis/byteslice"
	"io"
	"strings"

	"github.com/joomcode/errorx"
)

var readerPool byteslice.Pool

type ByteResponse struct {
	Val []byte
}

// Release puts Val byte slice back to pool for reuse. You must not use Val after this.
func (br *ByteResponse) Release() {
	readerPool.Put(br.Val)
}

// ReadResponse reads single RESP answer from bufio.Reader
func ReadResponse(b *bufio.Reader, wrapBytes bool) interface{} {
	line, isPrefix, err := b.ReadLine()
	if err != nil {
		return ErrIO.WrapWithNoMessage(err)
	}

	if isPrefix {
		return ErrHeaderlineTooLarge.NewWithNoMessage().WithProperty(EKLine, line)
	}

	if len(line) == 0 {
		return ErrHeaderlineEmpty.NewWithNoMessage()
	}

	var v int64
	switch line[0] {
	case '+':
		return string(line[1:])
	case '-':
		// detect MOVED and ASK
		txt := string(line[1:])
		moved := strings.HasPrefix(txt, "MOVED ")
		ask := strings.HasPrefix(txt, "ASK ")
		if moved || ask {
			parts := bytes.Split(line, []byte(" "))
			if len(parts) < 3 {
				return ErrResponseFormat.NewWithNoMessage().WithProperty(EKLine, line)
			}
			slot, err := parseInt(parts[1])
			if err != nil {
				return err.WithProperty(EKLine, line)
			}
			kind := ErrAsk
			if moved {
				kind = ErrMoved
			}
			return kind.New(txt).WithProperty(EKMovedTo, string(parts[2])).WithProperty(EKSlot, slot)
		}
		if strings.HasPrefix(txt, "LOADING") {
			return ErrLoading.New(txt)
		}
		if strings.HasPrefix(txt, "EXECABORT") {
			return ErrExecAbort.New(txt)
		}
		if strings.HasPrefix(txt, "TRYAGAIN") {
			return ErrTryAgain.New(txt)
		}
		return ErrResult.New(txt)
	case ':':
		v, err := parseInt(line[1:])
		if err != nil {
			return err.WithProperty(EKLine, line)
		}
		return v
	case '$':
		var rerr *errorx.Error
		if v, rerr = parseInt(line[1:]); rerr != nil {
			return rerr.WithProperty(EKLine, line)
		}
		if v < 0 {
			return nil
		}

		buf := readerPool.Get(int(v + 2))
		if _, err = io.ReadFull(b, buf); err != nil {
			return ErrIO.WrapWithNoMessage(err)
		}

		if buf[v] != '\r' || buf[v+1] != '\n' {
			readerPool.Put(buf)
			return ErrNoFinalRN.NewWithNoMessage()
		}

		if wrapBytes {
			return ByteResponse{Val: buf[:v]}
		}

		return buf[:v]
	case '*':
		var rerr *errorx.Error
		if v, rerr = parseInt(line[1:]); rerr != nil {
			return rerr.WithProperty(EKLine, line)
		}
		if v < 0 {
			return nil
		}
		result := make([]interface{}, v)
		for i := int64(0); i < v; i++ {
			result[i] = ReadResponse(b, false)
			if e, ok := result[i].(*errorx.Error); ok && !e.IsOfType(ErrResult) {
				return e
			}
		}
		return result
	default:
		return ErrUnknownHeaderType.NewWithNoMessage()
	}
}

func parseInt(buf []byte) (int64, *errorx.Error) {
	if len(buf) == 0 {
		return 0, ErrIntegerParsing.New("empty buffer")
	}

	neg := buf[0] == '-'
	if neg {
		buf = buf[1:]
	}
	v := int64(0)
	for _, b := range buf {
		if b < '0' || b > '9' {
			return 0, ErrIntegerParsing.New("contains non-digit")
		}
		v *= 10
		v += int64(b - '0')
	}
	if neg {
		v = -v
	}
	return v, nil
}
