package itemgenerator

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/scylla-operator/pkg/pointer"
)

type channelProperties struct {
	Capacity int
	Size     int
}

func chanToChannelProperties[T any](ch chan T) *channelProperties {
	if ch == nil {
		return nil
	}

	return &channelProperties{
		Capacity: cap(ch),
		Size:     len(ch),
	}
}

type generatorProperties[T any] struct {
	Name              string
	GeneratedValue    *T
	GeneratedError    error
	UnlimitedBuffer   *channelProperties
	RateLimitedBuffer *channelProperties
	RateLimiterDelay  time.Duration
}

func generatorToGeneratorProperties[T any](g Generator[T]) generatorProperties[T] {
	v, err := g.generateFunc()
	return generatorProperties[T]{
		Name:              g.name,
		GeneratedValue:    v,
		GeneratedError:    err,
		UnlimitedBuffer:   chanToChannelProperties(g.unlimitedBuffer),
		RateLimitedBuffer: chanToChannelProperties(g.rateLimitedBuffer),
		RateLimiterDelay:  g.rateLimiterDelay,
	}
}

func TestNewGenerator(t *testing.T) {
	generateFunc := func() (*int, error) {
		return pointer.Ptr(2 * 42), nil
	}

	tt := []struct {
		name              string
		genName           string
		min               int
		max               int
		delay             time.Duration
		generateFunc      func() (*int, error)
		expectedGenerator *Generator[int]
		expectedErr       error
	}{
		{
			name:              "fails with zero min",
			min:               0,
			expectedGenerator: nil,
			expectedErr:       fmt.Errorf("min (0) can't be lower then 1"),
		},
		{
			name:              "fails with zero max",
			min:               1,
			max:               0,
			expectedGenerator: nil,
			expectedErr:       fmt.Errorf("max (0) can't be lower then 1"),
		},
		{
			name:              "fails when max < min",
			min:               2,
			max:               1,
			expectedGenerator: nil,
			expectedErr:       fmt.Errorf("max (1) can't be lower then min (2)"),
		},
		{
			name:              "fails when generate func is nil",
			min:               1,
			max:               1,
			generateFunc:      nil,
			expectedGenerator: nil,
			expectedErr:       fmt.Errorf("generateFunc can't be nil"),
		},
		{
			name:         "successfully creates a generator",
			genName:      "foo",
			min:          42,
			max:          43,
			delay:        1 * time.Millisecond,
			generateFunc: generateFunc,
			expectedGenerator: &Generator[int]{
				name:              "foo",
				generateFunc:      generateFunc,
				rateLimiterDelay:  1 * time.Millisecond,
				unlimitedBuffer:   make(chan *int, 41),
				rateLimitedBuffer: make(chan *int, 1),
			},
			expectedErr: nil,
		},
		{
			name:         "successfully creates a generator without rate limited buffer",
			genName:      "foo",
			min:          42,
			max:          42,
			delay:        1 * time.Millisecond,
			generateFunc: generateFunc,
			expectedGenerator: &Generator[int]{
				name:              "foo",
				generateFunc:      generateFunc,
				rateLimiterDelay:  1 * time.Millisecond,
				unlimitedBuffer:   make(chan *int, 41),
				rateLimitedBuffer: make(chan *int, 0),
			},
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewGenerator(tc.genName, tc.min, tc.max, tc.delay, tc.generateFunc)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("expected error %#+v, got %#+v", tc.expectedErr, err)
			}

			if err != nil {
				return
			}

			cmpOpts := cmp.Options{
				cmp.Transformer("generatorProperties", generatorToGeneratorProperties[int]),
			}
			if !cmp.Equal(got, tc.expectedGenerator, cmpOpts) {
				t.Errorf("expected and actual generator differ: %s", cmp.Diff(tc.expectedGenerator, got, cmpOpts))
			}
		})
	}
}
