package redisclusterutil

import (
	"bytes"
	"math/rand"

	"github.com/joomcode/redispipe/redis"
)

var rKey = []byte("RANDOMKEY")
var eKey = []byte("")

// ReqSlot returns slot number targeted by this command.
func ReqSlot(req redis.Request) (uint16, bool) {
	key, ok := req.KeyByte()
	if bytes.Equal(rKey, key) && !ok {
		return uint16(rand.Intn(NumSlots)), true
	}
	return ByteSlot(key), ok
}

// BatchSlot returns slot common for all requests in batch (if there is such common slot).
func BatchSlot(reqs []redis.Request) (uint16, bool) {
	var slot uint16
	var set bool
	for _, req := range reqs {
		s, ok := ReqSlot(req)
		if !ok {
			continue
		}
		if !set {
			slot = s
			set = true
		} else if slot != s {
			return 0, false
		}
	}
	return slot, set
}

// BatchKey returns first key from a batch that is targeted to common slot.
func BatchKey(reqs []redis.Request) ([]byte, bool) {
	var key []byte
	var slot uint16
	var set bool
	for _, req := range reqs {
		k, ok := req.KeyByte()
		if !ok {
			continue
		}
		s := ByteSlot(k)
		if !set {
			key, slot = k, s
			set = true
		} else if slot != s {
			return eKey, false
		}
	}
	return key, set
}
