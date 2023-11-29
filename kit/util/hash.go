package util

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"hash/fnv"
	"sync"
)

var (
	fnv1aPool = &sync.Pool{
		New: func() interface{} {
			return fnv.New32a()
		},
	}
)

func SHA256(str string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(str)))
}

type ConsistentHash struct { // TODO
	Hasher hash.Hash32

	// lock protects Hasher while calculating the hash code.  It is assumed that
	// the Hasher field is read-only once the Balancer is created, so as a
	// performance optimization, reads of the field are not protected.
	lock sync.Mutex
}

func (h *ConsistentHash) Get(key string, partitionsLen int) int {
	hasher := h.Hasher
	if hasher != nil {
		h.lock.Lock()
		defer h.lock.Unlock()
	} else {
		hasher = fnv1aPool.Get().(hash.Hash32)
		defer fnv1aPool.Put(hasher)
	}

	hasher.Reset()
	if _, err := hasher.Write([]byte(key)); err != nil {
		panic(err)
	}

	// uses same algorithm that Sarama's hashPartitioner uses
	// note the type conversions here.  if the uint32 hash code is not cast to
	// an int32, we do not get the same result as sarama.
	partition := int32(hasher.Sum32()) % int32(partitionsLen)
	if partition < 0 {
		partition = -partition
	}

	return int(partition)
}
