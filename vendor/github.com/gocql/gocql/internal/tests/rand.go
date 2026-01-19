package tests

import (
	"math/rand"
	"sync"
	"time"
)

// RandInterface defines the thread-safe random number generator interface.
// It abstracts all methods provided by ThreadSafeRand.
type RandInterface interface {
	Uint64() uint64
	Uint32() uint32
	Int() int
	Intn(n int) int
	Int63() int64
	Int63n(n int64) int64
	Int31() int32
	Int31n(n int32) int32
	Float64() float64
	Float32() float32
	ExpFloat64() float64
	NormFloat64() float64
	Shuffle(n int, swap func(i, j int))
	Read(p []byte) (n int, err error)
}

// RandomTokens generates a slice of n random int64 tokens using a thread-safe random number generator.
//
// Parameters:
//
//	n - the number of random tokens to generate.
//
// Returns:
//
//	A slice of n randomly generated int64 tokens.
func RandomTokens(rnd RandInterface, n int) []int64 {
	var tokens []int64
	for i := 0; i < n; i++ {
		tokens = append(tokens, rnd.Int63())
	}
	return tokens
}

// ShuffledIndexes returns a slice containing integers from 0 to n-1 in random order.
//
// It uses a thread-safe random number generator to perform an in-place shuffle.
//
// Parameters:
//
//	n - the number of elements to include in the shuffled list.
//
// Returns:
//
//	A randomly shuffled slice of integers from 0 to n-1.
func ShuffledIndexes(rnd RandInterface, n int) []int {
	indexes := make([]int, n)
	for i := range indexes {
		indexes[i] = i
	}
	rnd.Shuffle(n, func(i, j int) {
		indexes[i], indexes[j] = indexes[j], indexes[i]
	})
	return indexes
}

// ThreadSafeRand provides a concurrency-safe wrapper around math/rand.Rand.
// It allows safe usage of random number generation methods from multiple goroutines.
// All access to the underlying rand.Rand is synchronized via a mutex.
type ThreadSafeRand struct {
	r   *rand.Rand
	mux sync.Mutex
}

// NewThreadSafeRand creates and returns a new instance of ThreadSafeRand,
// initialized with the given seed. The resulting generator is safe for concurrent use.
func NewThreadSafeRand(seed int64) *ThreadSafeRand {
	return &ThreadSafeRand{
		r: rand.New(rand.NewSource(seed)),
	}
}

func (r *ThreadSafeRand) Uint64() uint64 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Uint64()
}

func (r *ThreadSafeRand) Uint32() uint32 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Uint32()
}

func (r *ThreadSafeRand) Int() int {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Int()
}

func (r *ThreadSafeRand) Intn(n int) int {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Intn(n)
}

func (r *ThreadSafeRand) Int63() int64 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Int63()
}

func (r *ThreadSafeRand) Int63n(n int64) int64 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Int63n(n)
}

func (r *ThreadSafeRand) Int31() int32 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Int31()
}

func (r *ThreadSafeRand) Int31n(n int32) int32 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Int31n(n)
}

func (r *ThreadSafeRand) Float64() float64 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Float64()
}

func (r *ThreadSafeRand) Float32() float32 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Float32()
}

func (r *ThreadSafeRand) ExpFloat64() float64 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.ExpFloat64()
}

func (r *ThreadSafeRand) NormFloat64() float64 {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.NormFloat64()
}

func (r *ThreadSafeRand) Shuffle(n int, swap func(i, j int)) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.r.Shuffle(n, swap)
}

func (r *ThreadSafeRand) Read(p []byte) (n int, err error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.r.Read(p)
}

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

const randCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func RandomText(size int) string {
	result := make([]byte, size)
	for i := range result {
		result[i] = randCharset[rand.Intn(len(randCharset))]
	}
	return string(result)
}
