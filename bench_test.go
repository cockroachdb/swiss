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
	"io"
	"strconv"
	"testing"
)

func BenchmarkMapIter(b *testing.B) {
	b.Run("runtimeMap", func(b *testing.B) {
		b.Run("Int", benchSizes(benchmarkRuntimeMapIter[int64], genKeys[int64]))
	})
	b.Run("swissMap", func(b *testing.B) {
		b.Run("Int", benchSizes(benchmarkSwissMapIter[int64], genKeys[int64]))
	})
}

func BenchmarkMapGetHit(b *testing.B) {
	b.Run("runtimeMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkRuntimeMapGetHit[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkRuntimeMapGetHit[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkRuntimeMapGetHit[string], genKeys[string]))
	})
	b.Run("swissMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkSwissMapGetHit[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkSwissMapGetHit[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkSwissMapGetHit[string], genKeys[string]))
	})
}

func BenchmarkMapGetMiss(b *testing.B) {
	b.Run("runtimeMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkRuntimeMapGetMiss[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkRuntimeMapGetMiss[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkRuntimeMapGetMiss[string], genKeys[string]))
	})
	b.Run("swissMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkSwissMapGetMiss[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkSwissMapGetMiss[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkSwissMapGetMiss[string], genKeys[string]))
	})
}

func BenchmarkMapPutGrow(b *testing.B) {
	b.Run("runtimeMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkRuntimeMapPutGrow[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkRuntimeMapPutGrow[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkRuntimeMapPutGrow[string], genKeys[string]))
	})
	b.Run("swissMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkSwissMapPutGrow[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkSwissMapPutGrow[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkSwissMapPutGrow[string], genKeys[string]))
	})
}

func BenchmarkMapPutPreAllocate(b *testing.B) {
	b.Run("runtimeMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkRuntimeMapPutPreAllocate[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkRuntimeMapPutPreAllocate[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkRuntimeMapPutPreAllocate[string], genKeys[string]))
	})
	b.Run("swissMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkSwissMapPutPreAllocate[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkSwissMapPutPreAllocate[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkSwissMapPutPreAllocate[string], genKeys[string]))
	})
}

func BenchmarkMapPutReuse(b *testing.B) {
	b.Run("runtimeMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkRuntimeMapPutReuse[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkRuntimeMapPutReuse[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkRuntimeMapPutReuse[string], genKeys[string]))
	})
	b.Run("swissMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkSwissMapPutReuse[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkSwissMapPutReuse[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkSwissMapPutReuse[string], genKeys[string]))
	})
}

func BenchmarkMapPutDelete(b *testing.B) {
	b.Run("runtimeMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkRuntimeMapPutDelete[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkRuntimeMapPutDelete[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkRuntimeMapPutDelete[string], genKeys[string]))
	})
	b.Run("swissMap", func(b *testing.B) {
		b.Run("Int64", benchSizes(benchmarkSwissMapPutDelete[int64], genKeys[int64]))
		b.Run("Int32", benchSizes(benchmarkSwissMapPutDelete[int32], genKeys[int32]))
		b.Run("String", benchSizes(benchmarkSwissMapPutDelete[string], genKeys[string]))
	})
}

type benchTypes interface {
	int32 | int64 | string
}

func benchSizes[T benchTypes](
	f func(b *testing.B, n int, genKeys func(start, end int) []T), genKeys func(start, end int) []T,
) func(*testing.B) {
	var cases = []int{
		6, 12, 18, 24, 30,
		64,
		128,
		256,
		512,
		1024,
		2048,
		4096,
		8192,
		1 << 15,
		1 << 16,
		1 << 17,
		1 << 18,
		1 << 19,
		1 << 20,
		1 << 21,
		1 << 22,
		// 1 << 23,
		// 1 << 24,
	}

	return func(b *testing.B) {
		for _, n := range cases {
			b.Run(strconv.Itoa(n), func(b *testing.B) { f(b, n, genKeys) })
		}
	}
}

func genKeys[T benchTypes](start, end int) []T {
	var t T
	switch any(t).(type) {
	case int32:
		keys := make([]int32, end-start)
		for i := range keys {
			keys[i] = int32(start + i)
		}
		return unsafeConvertSlice[T](keys)
	case int64:
		keys := make([]int64, end-start)
		for i := range keys {
			keys[i] = int64(start + i)
		}
		return unsafeConvertSlice[T](keys)
	case string:
		keys := make([]string, end-start)
		for i := range keys {
			keys[i] = strconv.Itoa(start + i)
		}
		return unsafeConvertSlice[T](keys)
	default:
		panic("not reached")
	}
}

func benchmarkRuntimeMapIter[T benchTypes](b *testing.B, n int, genKeys func(start, end int) []T) {
	m := make(map[T]T, n)
	keys := genKeys(0, n)
	for _, k := range keys {
		m[k] = k
	}
	b.ResetTimer()
	var tmp T
	for i := 0; i < b.N; i++ {
		for k, v := range m {
			tmp += k + v
		}
	}
}

func benchmarkSwissMapIter[T benchTypes](b *testing.B, n int, genKeys func(start, end int) []T) {
	m := New[T, T](n)
	keys := genKeys(0, n)
	for _, k := range keys {
		m.Put(k, k)
	}
	b.ResetTimer()
	var tmp T
	for i := 0; i < b.N; i++ {
		m.All(func(k, v T) bool {
			tmp += k + v
			return true
		})
	}
}

func benchmarkRuntimeMapGetMiss[T benchTypes](
	b *testing.B, n int, genKeys func(start, end int) []T,
) {
	m := make(map[T]T)
	keys := genKeys(0, n)
	miss := genKeys(-n, 0)
	for _, k := range keys {
		m[k] = k
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m[miss[i%len(miss)]]
	}
}

func benchmarkSwissMapGetMiss[T comparable](b *testing.B, n int, genKeys func(start, end int) []T) {
	m := New[T, T](0)
	keys := genKeys(0, n)
	miss := genKeys(-n, 0)
	for j := range keys {
		m.Put(keys[j], keys[j])
	}
	b.ResetTimer()
	var ok bool
	for i := 0; i < b.N; i++ {
		_, ok = m.Get(miss[i%len(miss)])
	}
	b.StopTimer()
	fmt.Fprint(io.Discard, ok)

	b.ReportMetric(float64(m.Len())/float64(m.capacity()), "load")

	var fullGroups uint32
	var groupsCount uint32
	m.buckets(0, func(b *bucket[T, T]) bool {
		fullGroups += b.fullGroups()
		groupsCount += b.groupMask + 1
		return true
	})
	b.ReportMetric(100*float64(fullGroups)/float64(groupsCount), "%full")
}

func benchmarkRuntimeMapGetHit[T benchTypes](
	b *testing.B, n int, genKeys func(start, end int) []T,
) {
	m := make(map[T]T, n)
	keys := genKeys(0, n)
	for _, k := range keys {
		m[k] = k
	}

	// Go's builtin map has an optimization to avoid string comparisons if
	// there is pointer equality. Defeat this optimization to get a better
	// apples-to-apples comparison. This is reasonable to do because looking
	// up a value by a string key which shares the underlying string data with
	// the element in the map is a rare pattern.
	keys = genKeys(0, n)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m[keys[i%n]]
	}
}

func benchmarkSwissMapGetHit[T benchTypes](b *testing.B, n int, genKeys func(start, end int) []T) {
	m := New[T, T](n)
	keys := genKeys(0, n)
	for _, k := range keys {
		m.Put(k, k)
	}
	b.ResetTimer()
	var ok bool
	for i := 0; i < b.N; i++ {
		_, ok = m.Get(keys[i%n])
	}
	b.StopTimer()
	fmt.Fprint(io.Discard, ok)
}

func benchmarkRuntimeMapPutGrow[T benchTypes](
	b *testing.B, n int, genKeys func(start, end int) []T,
) {
	keys := genKeys(0, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := make(map[T]T)
		for _, k := range keys {
			m[k] = k
		}
	}
}

func benchmarkSwissMapPutGrow[T benchTypes](b *testing.B, n int, genKeys func(start, end int) []T) {
	var m Map[T, T]
	keys := genKeys(0, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Init(0)
		for _, k := range keys {
			m.Put(k, k)
		}
	}
}

func benchmarkRuntimeMapPutPreAllocate[T benchTypes](
	b *testing.B, n int, genKeys func(start, end int) []T,
) {
	keys := genKeys(0, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := make(map[T]T, n)
		for _, k := range keys {
			m[k] = k
		}
	}
}

func benchmarkSwissMapPutPreAllocate[T benchTypes](
	b *testing.B, n int, genKeys func(start, end int) []T,
) {
	var m Map[T, T]
	keys := genKeys(0, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Init(n)
		for _, k := range keys {
			m.Put(k, k)
		}
	}
}

func benchmarkRuntimeMapPutReuse[T benchTypes](
	b *testing.B, n int, genKeys func(start, end int) []T,
) {
	m := make(map[T]T, n)
	keys := genKeys(0, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, k := range keys {
			m[k] = k
		}
		for k := range m {
			delete(m, k)
		}
	}
}

func benchmarkSwissMapPutReuse[T benchTypes](
	b *testing.B, n int, genKeys func(start, end int) []T,
) {
	m := New[T, T](n)
	keys := genKeys(0, n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, k := range keys {
			m.Put(k, k)
		}
		m.Clear()
	}
}

func benchmarkRuntimeMapPutDelete[T benchTypes](
	b *testing.B, n int, genKeys func(start, end int) []T,
) {
	m := make(map[T]T, n)
	keys := genKeys(0, n)
	for _, k := range keys {
		m[k] = k
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := i % n
		delete(m, keys[j])
		m[keys[j]] = keys[j]
	}
}

func benchmarkSwissMapPutDelete[T benchTypes](
	b *testing.B, n int, genKeys func(start, end int) []T,
) {
	m := New[T, T](n)
	keys := genKeys(0, n)
	for _, k := range keys {
		m.Put(k, k)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := i % n
		m.Delete(keys[j])
		m.Put(keys[j], keys[j])
	}
}
