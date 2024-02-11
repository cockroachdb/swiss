// Copyright 2024 The Cockroach Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package swiss

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toBuiltinMap returns the elements as a map[K]V. Useful for testing.
func (m *Map[K, V]) toBuiltinMap() map[K]V {
	r := make(map[K]V)
	m.All(func(k K, v V) bool {
		r[k] = v
		return true
	})
	return r
}

// TODO(peter): Extracting a random element might be generally useful. Should
// this be promoted to the public API? Note that the elements are not selected
// uniformly randomly. If we promote this method to the public API it should
// take a rand.Rand.
func (m *Map[K, V]) randElement() (key K, value V, ok bool) {
	// TODO(peter): fixme
	t := &m.bucket0
	if t.capacity > 0 {
		offset := uintptr(rand.Intn(int(t.capacity)))
		for i := uintptr(0); i <= t.capacity; i++ {
			j := (i + offset) & t.capacity
			if (*t.ctrls.At(j) & ctrlEmpty) != ctrlEmpty {
				s := t.slots.At(j)
				return s.key, s.value, true
			}
		}
	}
	return key, value, false
}

func TestLittleEndian(t *testing.T) {
	// The implementation of group h2 matching and group empty and deleted
	// masking assumes a little endian CPU architecture. Assert that we are
	// running on one.
	b := []uint8{0x1, 0x2, 0x3, 0x4}
	v := *(*uint32)(unsafe.Pointer(&b[0]))
	require.EqualValues(t, 0x04030201, v)
}

func TestProbeSeq(t *testing.T) {
	genSeq := func(n int, hash, mask uintptr) []uintptr {
		seq := makeProbeSeq(hash, mask)
		vals := make([]uintptr, n)
		for i := 0; i < n; i++ {
			vals[i] = seq.offset
			seq = seq.next()
		}
		return vals
	}
	genGroups := func(n int, start, count uintptr) []uintptr {
		var vals []uintptr
		for i := start % groupSize; i < count; i += groupSize {
			vals = append(vals, i)
		}
		sort.Slice(vals, func(i, j int) bool {
			return vals[i] < vals[j]
		})
		return vals
	}

	// The Abseil probeSeq test cases.
	expected := []uintptr{0, 8, 24, 48, 80, 120, 40, 96, 32, 104, 56, 16, 112, 88, 72, 64}
	require.Equal(t, expected, genSeq(16, 0, 127))
	require.Equal(t, expected, genSeq(16, 128, 127))

	// Verify that we touch all of the groups no matter what our start offset
	// within the group is.
	for i := uintptr(0); i < 128; i++ {
		vals := genSeq(16, i, 127)
		require.Equal(t, 16, len(vals))
		sort.Slice(vals, func(i, j int) bool {
			return vals[i] < vals[j]
		})
		require.Equal(t, genGroups(16, i, 128), vals)
	}
}

func TestMatchH2(t *testing.T) {
	ctrls := []ctrl{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8}
	for i := uintptr(1); i <= 8; i++ {
		match := makeCtrlBytes(ctrls).GroupAt(0).matchH2(i)
		bit := match.first()
		require.EqualValues(t, i-1, bit)
	}
}

func TestMatchEmpty(t *testing.T) {
	testCases := []struct {
		ctrls    []ctrl
		expected []uintptr
	}{
		{[]ctrl{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8}, nil},
		{[]ctrl{0x1, 0x2, 0x3, ctrlEmpty, 0x5, ctrlDeleted, 0x7, ctrlSentinel}, []uintptr{3}},
		{[]ctrl{0x1, 0x2, 0x3, ctrlEmpty, 0x5, 0x6, ctrlEmpty, 0x8}, []uintptr{3, 6}},
	}
	for _, c := range testCases {
		t.Run("", func(t *testing.T) {
			match := makeCtrlBytes(c.ctrls).GroupAt(0).matchEmpty()
			var results []uintptr
			for match != 0 {
				idx := match.first()
				results = append(results, idx)
				match = match.remove(idx)
			}
			require.Equal(t, c.expected, results)
		})
	}
}

func TestMatchEmptyOrDeleted(t *testing.T) {
	testCases := []struct {
		ctrls    []ctrl
		expected []uintptr
	}{
		{[]ctrl{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8}, nil},
		{[]ctrl{0x1, 0x2, ctrlEmpty, ctrlDeleted, 0x5, 0x6, 0x7, ctrlSentinel}, []uintptr{2, 3}},
	}
	for _, c := range testCases {
		t.Run("", func(t *testing.T) {
			match := makeCtrlBytes(c.ctrls).GroupAt(0).matchEmptyOrDeleted()
			var results []uintptr
			for match != 0 {
				idx := match.first()
				results = append(results, idx)
				match = match.remove(idx)
			}
			require.Equal(t, c.expected, results)
		})
	}
}

func TestConvertNonFullToEmptyAndFullToDeleted(t *testing.T) {
	ctrls := make([]ctrl, groupSize)
	expected := make([]ctrl, groupSize)
	for i := 0; i < 100; i++ {
		for j := 0; j < groupSize; j++ {
			switch rand.Intn(4) {
			case 0: // 25% empty
				ctrls[j] = ctrlEmpty
				expected[j] = ctrlEmpty
			case 1: // 25% deleted
				ctrls[j] = ctrlDeleted
				expected[j] = ctrlEmpty
			case 2: // 25% sentinel
				ctrls[j] = ctrlSentinel
				expected[j] = ctrlEmpty
			default: // 25% full
				ctrls[j] = ctrl(rand.Intn(127))
				expected[j] = ctrlDeleted
			}
		}

		makeCtrlBytes(ctrls).GroupAt(0).convertNonFullToEmptyAndFullToDeleted()
		require.EqualValues(t, expected, ctrls)
	}
}

func TestAbsentAt(t *testing.T) {
	testCases := []struct {
		set   string
		start uintptr
		end   uintptr
	}{
		{set: "01001100", start: 1, end: 2},
		{set: "11001000", start: 0, end: 3},
		{set: "00001001", start: 4, end: 0},
		{set: "10001001", start: 0, end: 0},
		{set: "00000000", start: 8, end: 8},
	}
	for _, c := range testCases {
		t.Run("", func(t *testing.T) {
			b := bitsetFromString(t, c.set)
			require.Equal(t, c.start, b.absentAtStart())
			require.Equal(t, c.end, b.absentAtEnd())
		})
	}
}

func bitsetFromString(t *testing.T, str string) bitset {
	require.Equal(t, 8, len(str))
	var b bitset
	for i := 0; i < 8; i++ {
		require.True(t, str[i] == '0' || str[i] == '1')
		if str[i] == '1' {
			b |= 0x80 << (i * 8)
		}
	}
	return b
}

func TestWasNeverFull(t *testing.T) {
	b := &bucket[int, int]{
		capacity: 15,
		ctrls:    makeCtrlBytes(make([]ctrl, 16)),
	}

	testCases := []struct {
		emptyIndexes []uintptr
		expected     bool
	}{
		{[]uintptr{}, false},
		{[]uintptr{0}, false},
		{[]uintptr{0, 15}, true},
		{[]uintptr{1, 15}, true},
		{[]uintptr{2, 15}, true},
		{[]uintptr{3, 15}, true},
		{[]uintptr{4, 15}, true},
		{[]uintptr{5, 15}, true},
		{[]uintptr{6, 15}, true},
		{[]uintptr{7, 15}, true},
		{[]uintptr{8, 15}, false},
		{[]uintptr{0, 14}, true},
		{[]uintptr{0, 13}, true},
		{[]uintptr{0, 12}, true},
		{[]uintptr{0, 11}, true},
		{[]uintptr{0, 10}, true},
		{[]uintptr{0, 9}, true},
		{[]uintptr{0, 8}, true},
		{[]uintptr{0, 7}, false},
	}
	for _, c := range testCases {
		t.Run("", func(t *testing.T) {
			for i := uintptr(0); i < 16; i++ {
				*b.ctrls.At(i) = 0
			}
			for _, i := range c.emptyIndexes {
				*b.ctrls.At(i) = ctrlEmpty
			}
			require.Equal(t, c.expected, b.wasNeverFull(0))
		})
	}
}

func TestInitialCapacity(t *testing.T) {
	testCases := []struct {
		initialCapacity  int
		expectedCapacity int
	}{
		{0, 0},
		{1, 7},
		{7, 7},
		{8, 7},
		{1000, 1023},
		{1024, 1023},
	}
	for _, c := range testCases {
		t.Run("", func(t *testing.T) {
			m := New[int, int](c.initialCapacity)
			require.EqualValues(t, c.expectedCapacity, m.capacity())
		})
	}
}

func TestBasic(t *testing.T) {
	goodHash := New[int, int](0)
	badHash := New[int, int](0, WithHash[int, int](func(key *int, seed uintptr) uintptr {
		return 0
	}))

	for _, m := range []*Map[int, int]{goodHash, badHash} {
		t.Run("", func(t *testing.T) {
			e := make(map[int]int)
			require.EqualValues(t, 0, m.Len())
			require.EqualValues(t, 0, m.bucket0.growthLeft)

			// Non-existent.
			for i := 0; i < 10; i++ {
				_, ok := m.Get(i)
				require.False(t, ok)
			}

			// Insert.
			for i := 0; i < 10; i++ {
				m.Put(i, i+10)
				e[i] = i + 10
				v, ok := m.Get(i)
				require.True(t, ok)
				require.EqualValues(t, i+10, v)
				require.EqualValues(t, i+1, m.Len())
				require.Equal(t, e, m.toBuiltinMap())
			}

			// Update.
			for i := 0; i < 10; i++ {
				m.Put(i, i+20)
				e[i] = i + 20
				v, ok := m.Get(i)
				require.True(t, ok)
				require.EqualValues(t, i+20, v)
				require.EqualValues(t, 10, m.Len())
				require.Equal(t, e, m.toBuiltinMap())
			}

			// Delete.
			for i := 0; i < 10; i++ {
				m.Delete(i)
				delete(e, i)
				require.EqualValues(t, 10-i-1, m.Len())
				_, ok := m.Get(i)
				require.False(t, ok)
				require.Equal(t, e, m.toBuiltinMap())
			}
		})
	}
}

func TestRandom(t *testing.T) {
	goodHash := New[int, int](0)
	badHash := New[int, int](0, WithHash[int, int](func(key *int, seed uintptr) uintptr {
		return 0
	}))

	for _, m := range []*Map[int, int]{goodHash, badHash} {
		t.Run("", func(t *testing.T) {
			e := make(map[int]int)
			for i := 0; i < 10000; i++ {
				switch r := rand.Float64(); {
				case r < 0.5: // 50% inserts
					k, v := rand.Int(), rand.Int()
					if debug {
						fmt.Printf("insert %d: %d\n", k, v)
					}
					m.Put(k, v)
					e[k] = v
				case r < 0.65: // 15% updates
					if k, _, ok := m.randElement(); !ok {
						require.EqualValues(t, 0, m.Len(), e)
					} else {
						v := rand.Int()
						if debug {
							fmt.Printf("update %d: %d\n", k, v)
						}
						m.Put(k, v)
						e[k] = v
					}
				case r < 0.80: // 15% deletes
					if k, _, ok := m.randElement(); !ok {
						require.EqualValues(t, 0, m.Len(), e)
					} else {
						if debug {
							fmt.Printf("delete %d\n", k)
						}
						m.Delete(k)
						delete(e, k)
					}
				case r < 0.95: // 25% lookups
					if k, v, ok := m.randElement(); !ok {
						require.EqualValues(t, 0, m.Len(), e)
					} else {
						if debug {
							fmt.Printf("lookup %d: %d vs %d\n", k, e[k], v)
						}
						require.EqualValues(t, e[k], v)
					}
				default: // 5% rehash in place and iterate
					m.bucket0.rehashInPlace(m)
					require.Equal(t, e, m.toBuiltinMap())
				}
				require.EqualValues(t, len(e), m.Len())
			}
		})
	}
}

func TestIterateMutate(t *testing.T) {
	m := New[int, int](0)
	for i := 0; i < 100; i++ {
		m.Put(i, i)
	}
	e := m.toBuiltinMap()
	require.EqualValues(t, 100, m.Len())
	require.EqualValues(t, 100, len(e))

	// Iterate over the map, resizing it periodically. We should see all of
	// the elements that were originally in the map because All takes a
	// snapshot of the ctrls and slots before iterating.
	vals := make(map[int]int)
	m.All(func(k, v int) bool {
		if (k % 10) == 0 {
			m.bucket0.resize(m, 2*m.bucket0.capacity+1)
		}
		vals[k] = v
		return true
	})
	require.EqualValues(t, e, vals)
}

type countingAllocator[K comparable, V any] struct {
	allocSlots int
	allocCtrls int
	freeSlots  int
	freeCtrls  int
}

func (a *countingAllocator[K, V]) AllocSlots(n int) []Slot[K, V] {
	a.allocSlots++
	return make([]Slot[K, V], n)
}

func (a *countingAllocator[K, V]) AllocControls(n int) []uint8 {
	a.allocCtrls++
	return make([]uint8, n)
}

func (a *countingAllocator[K, V]) FreeSlots(_ []Slot[K, V]) {
	a.freeSlots++
}

func (a *countingAllocator[K, V]) FreeControls(_ []uint8) {
	a.freeCtrls++
}

func TestAllocator(t *testing.T) {
	a := &countingAllocator[int, int]{}
	m := New[int, int](0, WithAllocator[int, int](a))

	for i := 0; i < 100; i++ {
		m.Put(i, i)
	}

	// 8 -> 16 -> 32 -> 64 -> 128
	const expected = 5
	require.EqualValues(t, expected, a.allocSlots)
	require.EqualValues(t, expected, a.allocCtrls)
	require.EqualValues(t, expected-1, a.freeSlots)
	require.EqualValues(t, expected-1, a.freeCtrls)

	m.Close()

	require.EqualValues(t, expected, a.freeSlots)
	require.EqualValues(t, expected, a.freeCtrls)
}

func BenchmarkStringMap(b *testing.B) {
	const stringKeyLen = 8

	genStringData := func(count int) []string {
		src := rand.New(rand.NewSource(int64(stringKeyLen * count)))
		letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
		r := make([]rune, stringKeyLen*count)
		for i := range r {
			r[i] = letters[src.Intn(len(letters))]
		}
		keys := make([]string, count)
		for i := range keys {
			keys[i] = string(r[:stringKeyLen])
			r = r[stringKeyLen:]
		}
		return keys
	}

	sizes := []int{16, 128, 1024, 8192, 131072}
	for _, size := range sizes {
		// Test sizes that result in the max, min, and avg loads for swiss.Map.
		minLoad := size * 7 / 8
		maxLoad := minLoad - 1
		avgLoad := ((size/2)*7/8 + maxLoad) / 2

		loadType := []string{"avgLoad", "maxLoad", "minLoad"}
		for i, n := range []int{avgLoad, maxLoad, minLoad} {
			b.Run(fmt.Sprintf("%s,n=%d", loadType[i], n), func(b *testing.B) {
				b.Run("runtimeMap", func(b *testing.B) {
					benchmarkRuntimeMap(b, genStringData(n))
				})
				b.Run("swissMap", func(b *testing.B) {
					benchmarkSwissMap(b, genStringData(n))
				})
			})
		}
	}
}

func BenchmarkInt64Map(b *testing.B) {
	genInt64Data := func(n int) []int64 {
		keys := make([]int64, n)
		var x int64
		for i := range keys {
			x += rand.Int63n(128) + 1
			keys[i] = x
		}
		return keys
	}

	sizes := []int{16, 128, 1024, 8192, 131072}
	for _, size := range sizes {
		// Test sizes that result in the max, min, and avg loads for swiss.Map.
		minLoad := size * 7 / 8
		maxLoad := minLoad - 1
		avgLoad := ((size/2)*7/8 + maxLoad) / 2

		loadType := []string{"avgLoad", "maxLoad", "minLoad"}
		for i, n := range []int{avgLoad, maxLoad, minLoad} {
			b.Run(fmt.Sprintf("%s,n=%d", loadType[i], n), func(b *testing.B) {
				b.Run("runtimeMap", func(b *testing.B) {
					benchmarkRuntimeMap(b, genInt64Data(n))
				})
				b.Run("swissMap", func(b *testing.B) {
					benchmarkSwissMap(b, genInt64Data(n))
				})
			})
		}
	}
}

func benchmarkRuntimeMap[K comparable](b *testing.B, keys []K) {
	n := uint32(len(keys))

	// Go's builtin map has an optimization to avoid string comparisons if
	// there is pointer equality. Defeat this optimization to get a better
	// apples-to-apples comparison. This is reasonable to do because looking
	// up a value by a string key which shares the underlying string data with
	// the element in the map is a rare pattern.
	cloneKeys := func(keys []K) []K {
		var k K
		switch any(k).(type) {
		case string:
			cloned := make([]string, len(keys))
			for i := range keys {
				cloned[i] = fmt.Sprint(keys[i])
			}
			// Can't return cloned directly because it has type []string, not
			// []K. So jump through some unsafe hoops.
			return unsafe.Slice((*K)(unsafe.Pointer(unsafe.SliceData(cloned))), len(cloned))
		}
		return keys
	}

	b.Run("Get", func(b *testing.B) {
		m := make(map[K]K, n)
		for _, k := range keys {
			m[k] = k
		}
		lookupKeys := cloneKeys(keys)

		b.ResetTimer()
		var ok bool
		for i := 0; i < b.N; i++ {
			_, ok = m[lookupKeys[uint32(i)%n]]
		}
		assert.True(b, ok)
	})

	// Rather than benchmark puts and deletes separately, we benchmark them
	// together so the map stays a constant size.
	b.Run("PutDelete", func(b *testing.B) {
		m := make(map[K]K, n)
		for _, k := range keys {
			m[k] = k
		}
		deleteKeys := cloneKeys(keys)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			j := uint32(i) % n
			delete(m, deleteKeys[j])
			m[keys[j]] = keys[j]
		}
	})
}

func benchmarkSwissMap[K comparable](b *testing.B, keys []K) {
	n := uint32(len(keys))

	b.Run("Get", func(b *testing.B) {
		m := New[K, K](len(keys))
		for _, k := range keys {
			m.Put(k, k)
		}

		b.ResetTimer()
		var ok bool
		for i := 0; i < b.N; i++ {
			_, ok = m.Get(keys[uint32(i)%n])
		}
		b.StopTimer()

		assert.True(b, ok)
		b.ReportMetric(float64(m.Len())/float64(m.capacity()), "load-factor")
	})

	// Rather than benchmark puts and deletes separately, we benchmark them
	// together so the map stays a constant size.
	b.Run("PutDelete", func(b *testing.B) {
		m := New[K, K](len(keys))
		for _, k := range keys {
			m.Put(k, k)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			j := uint32(i) % n
			m.Delete(keys[j])
			m.Put(keys[j], keys[j])
		}
		b.StopTimer()

		b.ReportMetric(float64(m.Len())/float64(m.capacity()), "load-factor")
	})
}
