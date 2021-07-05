package bloomfilter

import (
	"encoding/binary"
	"math"
	"sync"
)

type BloomFilter struct {
	m       uint32
	k       int
	buckets []uint32
	lock    sync.RWMutex
}

// New creates a new bloom filter. m should specify the number of bits.
// m is rounded up to the nearest multiple of 32.
// k specifies the number of hashing functions.
func New(m, k int) *BloomFilter {
	var n = uint32(math.Ceil(float64(m) / 32))
	return &BloomFilter{
		m:       n * 32,
		k:       k,
		buckets: make([]uint32, n),
	}
}

// NewFromBytes creates a new bloom filter from a byte slice.
// b is a byte slice exported from another bloomfilter.
// k specifies the number of hashing functions.
func NewFromBytes(bb []byte, k int) *BloomFilter {
	ii := make([]uint32, len(bb)/4)
	for i := range ii {
		ii[i] = binary.BigEndian.Uint32(bb[i*4 : (i+1)*4])
	}
	return &BloomFilter{
		m:       uint32(len(ii) * 32),
		k:       k,
		buckets: ii,
	}
}

// EstimateParameters estimates requirements for m and k.
// https://github.com/willf/bloom
func EstimateParameters(n int, p float64) (m int, k int) {
	m = int(math.Ceil(-1 * float64(n) * math.Log(p) / math.Pow(math.Log(2), 2)))
	k = int(math.Ceil(math.Log(2) * float64(m) / float64(n)))
	if m%32 > 0 {
		m += 32 - m%32
	}
	return
}

func (bf *BloomFilter) locations(v []byte) []uint32 {
	var r = make([]uint32, bf.k)
	var a = fnv_1a(v, 0)
	var b = fnv_1a(v, 1576284489)
	var x = a % uint32(bf.m)
	for i := range r {
		r[i] = x
		x = (x + b) % bf.m
	}
	return r
}

// Add adds a byte array to the bloom filter
func (bf *BloomFilter) Add(v []byte) {
	bf.lock.Lock()
	defer bf.lock.Unlock()
	var loc = bf.locations(v)
	for _, l := range loc {
		bf.buckets[l/32] |= 1 << (l % 32)
	}
}

// AddInt adds an int to the bloom filter
func (bf *BloomFilter) AddInt(v int) {
	var a = make([]byte, 4)
	binary.BigEndian.PutUint32(a, uint32(v))
	bf.Add(a)
}

// Test evaluates a byte array to determine whether it is (probably) in the bloom filter
func (bf *BloomFilter) Test(v []byte) bool {
	bf.lock.RLock()
	defer bf.lock.RUnlock()
	var loc = bf.locations(v)
	for _, l := range loc {
		if (bf.buckets[l/32] & (1 << (l % 32))) == 0 {
			return false
		}
	}
	return true
}

// TestInt evaluates an int to determine whether it is (probably) in the bloom filter
func (bf *BloomFilter) TestInt(v int) bool {
	bf.lock.RLock()
	defer bf.lock.RUnlock()
	var a = make([]byte, 4)
	binary.BigEndian.PutUint32(a, uint32(v))
	return bf.Test(a)
}

// ToBytes returns the bloom filter as a byte slice
func (bf *BloomFilter) ToBytes() []byte {
	bf.lock.RLock()
	defer bf.lock.RUnlock()
	var bb = []byte{}
	for _, bucket := range bf.buckets {
		var a = make([]byte, 4)
		binary.BigEndian.PutUint32(a, bucket)
		bb = append(bb, a...)
	}
	return bb
}

// Fowler/Noll/Vo hashing.
// Nonstandard variation: this function optionally takes a seed value that is incorporated
// into the offset basis. According to http://www.isthe.com/chongo/tech/comp/fnv/index.html
// "almost any offset_basis will serve so long as it is non-zero".
func fnv_1a(v []byte, seed int) uint32 {
	var a = uint32(2166136261 ^ seed)
	for _, b := range v {
		var c = uint32(b)
		var d = c & 0xff00
		if d != 0 {
			a = fnv_multiply(a ^ d>>8)
		}
		a = fnv_multiply(a ^ c&0xff)
	}
	return fnv_mix(a)
}

// a * 16777619 mod 2**32
func fnv_multiply(a uint32) uint32 {
	return a + (a << 1) + (a << 4) + (a << 7) + (a << 8) + (a << 24)
}

// See https://web.archive.org/web/20131019013225/http://home.comcast.net/~bretm/hash/6.html
func fnv_mix(a uint32) uint32 {
	a += a << 13
	a ^= a >> 7
	a += a << 3
	a ^= a >> 17
	a += a << 5
	return a & 0xffffffff
}
