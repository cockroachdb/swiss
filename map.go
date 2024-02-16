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

// Package swiss is a Go implementation of Swiss Tables as described in
// https://abseil.io/about/design/swisstables. See also:
// https://faultlore.com/blah/hashbrown-tldr/.
//
// Google's C++ implementation:
//
//	https://github.com/abseil/abseil-cpp/blob/master/absl/container/internal/raw_hash_set.h
//
// # Swiss Tables
//
// Swiss tables are hash tables that map keys to values, similar to Go's
// builtin map type. Swiss tables use open-addressing rather than chaining to
// handle collisions. If you're not familiar with open-addressing see
// https://en.wikipedia.org/wiki/Open_addressing. A hybrid between linear and
// quadratic probing is used - linear probing within groups of small fixed
// size and quadratic probing at the group level. The key design choice of
// Swiss tables is the usage of a separate metadata array that stores 1 byte
// per slot in the table. 7-bits of this "control byte" are taken from
// hash(key) and the remaining bit is used to indicate whether the slot is
// empty, full, deleted, or a sentinel. The metadata array allows quick
// probes. The Google implementation of Swiss tables uses SIMD on x86 CPUs in
// order to quickly check 16 slots at a time for a match. Neon on arm64 CPUs
// is apparently too high latency, but the generic version is still able to
// compare 8 bytes at time through bit tricks (SWAR, SIMD Within A Register).
//
// A Swiss table's layout is N-1 slots where N is a power of 2 and N+groupSize
// control bytes. The [N:N+groupSize] control bytes mirror the first groupSize
// control bytes so that probe operations at the end of the control bytes
// array do not have to perform additional checks. The control byte for slot N
// is always a sentinel which is considered empty for the purposes of probing
// but is not available for storing an entry and is also not a deletion
// tombstone.
//
// Probing is done by taking the top 57 bits of hash(key)%N as the index into
// the control bytes and then performing a check of the groupSize control
// bytes at that index. Note that these groups are not aligned on a groupSize
// boundary (i.e. groups are conceptual, not physical, and they overlap) and
// an unaligned memory access is performed. According to
// https://lemire.me/blog/2012/05/31/data-alignment-for-speed-myth-or-reality/,
// data alignment for performance is a myth on modern CPUs. Probing walks
// through groups in the table using quadratic probing until it finds a group
// that has at least one empty slot or the sentinel control byte. See the
// comments on probeSeq for more details on the order in which groups are
// probed and the guarantee that every group is examined which means that in
// the worst case probing will end when the sentinel is encountered.
//
// Deletion is performed using tombstones (ctrlDeleted) with an optimization
// to mark a slot as empty if we can prove that doing so would not violate the
// probing behavior that a group of full slots causes probing to continue. It
// is invalid to take a group of full slots and mark one as empty as doing so
// would cause subsequent lookups to terminate at that group rather than
// continue to probe. We prove a slot was never part of a full group by
// looking for whether any of the groupSize-1 neighbors to the left and right
// of the deleting slot are empty which indicates that the slot was never part
// of a full group.
//
// # Extendible Hashing
//
// The Swiss table design has a significant caveat: resizing of the table is
// done all at once rather than incrementally. This can cause long-tail
// latency blips in some use cases. To address this caveat, extendible hashing
// (https://en.wikipedia.org/wiki/Extendible_hashing) is applied on top of the
// Swiss table foundation. In extendible hashing, there is a top-level
// directory containing entries pointing to buckets. In swiss.Map each bucket
// is a Swiss table as described above.
//
// The high bits of hash(key) are used to index into the bucket directory
// which is effectively a trie. The number of bits used is the globalDepth,
// resulting in 2^globalDepth directory entries. Adjacent entries in the
// directory are allowed to point to the same bucket which enables resizing to
// be done incrementally, one bucket at a time. Each bucket has a localDepth
// which is less than or equal to the globalDepth. If the localDepth for a
// bucket equals the globalDepth then only a single directory entry points to
// the bucket. Otherwise, more than one directory entry points to the bucket.
//
// The diagram below shows one possible scenario for the directory and
// buckets. With a globalDepth of 2 the directory contains 4 entries. The
// first 2 entries point to the same bucket which has a localDepth of 1, while
// the last 2 entries point to different buckets.
//
//	 dir(globalDepth=2)
//	+----+
//	| 00 | --\
//	+----+    +--> bucket[localDepth=1]
//	| 01 | --/
//	+----+
//	| 10 | ------> bucket[localDepth=2]
//	+----+
//	| 11 | ------> bucket[localDepth=2]
//	+----+
//
// The index into the directory is "hash(key) >> (64 - globalDepth)".
//
// When a bucket gets too large (specified by a configurable threshold) it is
// split. When a bucket is split its localDepth is incremented. If its
// localDepth is less than or equal to its globalDepth then the newly split
// bucket can be installed in the directory. If the bucket's localDepth is
// greater than the globalDepth then the globalDepth is incremented and the
// directory is reallocated at twice its current size. In the diagram above,
// consider what happens if the bucket at dir[3] is split:
//
//	 dir(globalDepth=3)
//	+-----+
//	| 000 | --\
//	+-----+    \
//	| 001 | ----\
//	+-----+      +--> bucket[localDepth=1]
//	| 010 | ----/
//	+-----+    /
//	| 011 | --/
//	+-----+
//	| 100 | --\
//	+-----+    +----> bucket[localDepth=2]
//	| 101 | --/
//	+-----+
//	| 110 | --------> bucket[localDepth=3]
//	+-----+
//	| 111 | --------> bucket[localDepth=3]
//	+-----+
//
// Note that the diagram above is very unlikely with a good hash function as
// the buckets will tend to fill at a similar rate.
//
// The split operation redistributes the records in a bucket into two buckets.
// This is done by walking over the records in the bucket to be split,
// computing hash(key) and using localDepth to extract the bit which
// determines whether to leave the record in the current bucket or to move it
// to the new bucket.
//
// Maps containing only a single bucket are optimized to avoid the directory
// indexing resulting in performance that is equivalent to a Swiss table
// without extendible hashing. A single bucket can be guaranteed by
// configuring a very large bucket size threshold via the
// WithMaxBucketCapacity option.
//
// # Implementation
//
// The implementation follows Google's Abseil implementation of Swiss Tables,
// and is heavily tuned, using unsafe and raw pointer arithmentic rather than
// Go slices to squeeze out every drop of performance. In order to support
// hashing of arbitrary keys, a hack is performed to extract the hash function
// from Go's implementation of map[K]struct{} by reaching into the internals
// of the type. (This might break in future version of Go, but is likely
// fixable unless the Go runtime does something drastic).
//
// # Performance
//
// A swiss.Map has similar or slightly better performance than Go's builtin
// map for small map sizes, and is much faster at large map sizes. See
// [README.md] for details.
//
// [README.md] https://github.com/cockroachdb/swiss/blob/main/README.md
package swiss

import (
	"fmt"
	"io"
	"math/bits"
	"strings"
	"unsafe"
)

const (
	groupSize       = 8
	maxAvgGroupLoad = 7

	ctrlEmpty    ctrl = 0b10000000
	ctrlDeleted  ctrl = 0b11111110
	ctrlSentinel ctrl = 0b11111111

	bitsetLSB     = 0x0101010101010101
	bitsetMSB     = 0x8080808080808080
	bitsetEmpty   = bitsetLSB * uint64(ctrlEmpty)
	bitsetDeleted = bitsetLSB * uint64(ctrlDeleted)

	minBucketCapacity        uintptr = 7
	defaultMaxBucketCapacity uintptr = 4095
)

// Slot holds a key and value.
type Slot[K comparable, V any] struct {
	key   K
	value V
}

// bucket implements Google's Swiss Tables hash table design. A Map is
// composed of 1 or more buckets that are addressed using extendible hashing.
type bucket[K comparable, V any] struct {
	// ctrls is capacity+groupSize in length. Ctrls[capacity] is always
	// ctrlSentinel which is used to stop probe iteration. A copy of the first
	// groupSize-1 elements of ctrls is mirrored into the remaining slots
	// which is done so that a probe sequence which picks a value near the end
	// of ctrls will have valid control bytes to look at.
	//
	// When the bucket is empty, ctrls points to emptyCtrls which will never
	// be modified and is used to simplify the Put, Get, and Delete code which
	// doesn't have to check for a nil ctrls.
	ctrls ctrlBytes
	// slots is capacity in length.
	slots unsafeSlice[Slot[K, V]]
	// The total number slots (always 2^N-1). The capacity is used as a mask
	// to quickly compute i%N using a bitwise & operation.
	capacity uintptr
	// The number of filled slots (i.e. the number of elements in the bucket).
	used int
	// The number of slots we can still fill without needing to rehash.
	//
	// This is stored separately due to tombstones: we do not include
	// tombstones in the growth capacity because we'd like to rehash when the
	// table is filled with tombstones as otherwise probe sequences might get
	// unacceptably long without triggering a rehash.
	growthLeft int
	// localDepth is the number of high bits from hash(key) used to generate
	// an index for the global directory to locate this bucket. If localDepth
	// is 0 this bucket is Map.bucket0.
	localDepth uint
	// The index of the bucket within Map.dir. If localDepth < globalDepth
	// then this is the index of the first entry in Map.dir which points to
	// this bucket and the following 1<<(globalDepth-localDepth) entries will
	// also point to this bucket.
	index uintptr
}

// Map is an unordered map from keys to values with Put, Get, Delete, and All
// operations. Map is inspired by Google's Swiss Tables design as implemented
// in Abseil's flat_hash_map, combined with extendible hashing. By default, a
// Map[K,V] uses the same hash function as Go's builtin map[K]V, though a
// different hash function can be specified using the WithHash option.
//
// A Map is NOT goroutine-safe.
type Map[K comparable, V any] struct {
	// The hash function to each keys of type K. The hash function is
	// extracted from the Go runtime's implementation of map[K]struct{}.
	hash hashFn
	seed uintptr
	// The allocator to use for the ctrls and slots slices.
	allocator Allocator[K, V]
	// bucket0 is always present and inlined in the Map to avoid a pointer
	// indirection during the common case that the map contains a single
	// bucket.
	bucket0 bucket[K, V]
	// The directory of buckets.
	dir unsafeSlice[*bucket[K, V]]
	// The number of filled slots across all buckets (i.e. the number of
	// elements in the map).
	used int
	// globalShift is the number of bits to right shift a hash value to
	// generate an index for the global directory. As a special case, if
	// globalShift==0 then bucket0 is used and the directory is not accessed.
	// Note that globalShift==(64-globalDepth). globalShift is used rather
	// than globalDepth because the shifting is the more common operation than
	// needing to compare globalDepth to a bucket's localDepth.
	globalShift uint
	// The maximum capacity a bucket is allowed to grow to before it will be
	// split.
	maxBucketCapacity uintptr
}

func normalizeCapacity(capacity uintptr) uintptr {
	return (uintptr(1) << bits.Len64(uint64(capacity)-1)) - 1
}

// New constructs a new Map with the specified initial capacity. If
// initialCapacity is 0 the map will start out with zero capacity and will
// grow on the first insert. The zero value for an M is not usable.
func New[K comparable, V any](initialCapacity int, options ...option[K, V]) *Map[K, V] {
	// The ctrls for an empty map points to emptyCtrls which simplifies
	// probing in Get, Put, and Delete. The emptyCtrls never match a probe
	// operation, but because growthLeft == 0 if we try to insert we'll
	// immediately rehash and grow.
	m := &Map[K, V]{
		hash:      getRuntimeHasher[K](),
		seed:      uintptr(fastrand64()),
		allocator: defaultAllocator[K, V]{},
		bucket0: bucket[K, V]{
			ctrls: emptyCtrls,
		},
		maxBucketCapacity: defaultMaxBucketCapacity,
	}

	for _, op := range options {
		op.apply(m)
	}

	if m.maxBucketCapacity < minBucketCapacity {
		m.maxBucketCapacity = minBucketCapacity
	}
	m.maxBucketCapacity = normalizeCapacity(m.maxBucketCapacity)

	if initialCapacity > 0 {
		// We consider initialCapacity to be an indication from the caller
		// about the number of records the map should hold. The realized
		// capacity of a map is 7/8 of the number of slots, so we set the
		// target capacity to initialCapacity*8/7.
		targetCapacity := uintptr((initialCapacity * groupSize) / maxAvgGroupLoad)
		if targetCapacity <= m.maxBucketCapacity {
			// Normalize targetCapacity to the smallest value of the form 2^k-1.
			m.bucket0.init(m, normalizeCapacity(targetCapacity))
		} else {
			// If targetCapacity is larger than maxBucketCapacity we need to
			// size the directory appropriately. We'll size each bucket to
			// maxBucketCapacity and create enough buckets to hold
			// initialCapacity.
			nBuckets := (targetCapacity + m.maxBucketCapacity - 1) / m.maxBucketCapacity
			globalDepth := uint(bits.Len64(uint64(nBuckets) - 1))
			m.growDirectory(globalDepth)

			n := m.bucketCount()
			buckets := make([]bucket[K, V], n)

			*m.dir.At(0) = &m.bucket0
			for i := uintptr(1); i < n; i++ {
				*m.dir.At(i) = &buckets[i]
			}

			for i := uintptr(0); i < n; i++ {
				b := *m.dir.At(i)
				b.init(m, m.maxBucketCapacity)
				b.localDepth = globalDepth
				b.index = i
			}

			m.checkInvariants()
		}
	}

	m.buckets(0, func(b *bucket[K, V]) bool {
		b.checkInvariants(m)
		return true
	})
	return m
}

// Close closes the map, releasing any memory back to its configured
// allocator. It is unnecessary to close a map using the default allocator. It
// is invalid to use a Map after it has been closed, though Close itself is
// idempotent.
func (m *Map[K, V]) Close() {
	m.buckets(0, func(b *bucket[K, V]) bool {
		b.close(m.allocator)
		return true
	})

	m.allocator = nil
}

// Put inserts an entry into the map, overwriting an existing value if an
// entry with the same key already exists.
func (m *Map[K, V]) Put(key K, value V) {
	// Put is find composed with uncheckedPut. We perform find to see if the
	// key is already present. If it is, we're done and overwrite the existing
	// value. If the value isn't present we perform an uncheckedPut which
	// inserts an entry known not to be in the table (violating this
	// requirement will cause the table to behave erratically).
	h := m.hash(noescape(unsafe.Pointer(&key)), m.seed)
	b := m.bucket(h)

	// NB: Unlike the abseil swiss table implementation which uses a common
	// find routine for Get, Put, and Delete, we have to manually inline the
	// find routine for performance.
	seq := makeProbeSeq(h1(h), b.capacity)
	for ; ; seq = seq.next() {
		g := b.ctrls.GroupAt(seq.offset)
		match := g.matchH2(h2(h))

		for match != 0 {
			slotIdx := match.first()
			i := seq.offsetAt(slotIdx)
			slot := b.slots.At(i)
			if key == slot.key {
				slot.value = value
				b.checkInvariants(m)
				return
			}
			match = match.remove(slotIdx)
		}

		match = g.matchEmpty()
		if match != 0 {
			// Before performing the insertion we may decide the bucket is
			// getting overcrowded (i.e. the load factor is greater than 7/8
			// for big tables; small tables use a max load factor of 1).
			if b.growthLeft == 0 {
				b.rehash(m)
				// We may have split the bucket in which case we have to
				// re-determine which bucket the key resides on. This
				// determination is quick in comparison to rehashing,
				// resizing, and splitting, so just always do it. Note that we
				// don't have to restart the entire Put process as we know the
				// key doesn't exist in the map.
				b = m.bucket(h)
			}
			b.uncheckedPut(h, key, value)
			b.used++
			m.used++
			b.checkInvariants(m)
			return
		}
	}
}

// Get retrieves the value from the map for the specified key, return ok=false
// if the key is not present.
func (m *Map[K, V]) Get(key K) (value V, ok bool) {
	h := m.hash(noescape(unsafe.Pointer(&key)), m.seed)
	b := m.bucket(h)

	// NB: Unlike the abseil swiss table implementation which uses a common
	// find routine for Get, Put, and Delete, we have to manually inline the
	// find routine for performance.

	// To find the location of a key in the table, we compute hash(key). From
	// h1(hash(key)) and the capacity, we construct a probeSeq that visits every
	// group of slots in some interesting order.
	//
	// We walk through these indices. At each index, we select the entire group
	// starting with that index and extract potential candidates: occupied slots
	// with a control byte equal to h2(hash(key)). If we find an empty slot in the
	// group, we stop and return an error. The key at candidate slot y is compared
	// with key; if key == m.slots[y].key we are done and return y; otherwise we
	// continue to the next probe index. Tombstones (ctrlDeleted) effectively
	// behave like full slots that never match the value we're looking for.
	//
	// The h2 bits ensure when we compare a key we are likely to have actually
	// found the object. That is, the chance is low that keys compare false. Thus,
	// when we search for an object, we are unlikely to call == many times. This
	// likelyhood can be analyzed as follows (assuming that h2 is a random enough
	// hash function).
	//
	// Let's assume that there are k "wrong" objects that must be examined in a
	// probe sequence. For example, when doing a find on an object that is in the
	// table, k is the number of objects between the start of the probe sequence
	// and the final found object (not including the final found object). The
	// expected number of objects with an h2 match is then k/128. Measurements and
	// analysis indicate that even at high load factors, k is less than 32,
	// meaning that the number of false positive comparisons we must perform is
	// less than 1/8 per find.
	seq := makeProbeSeq(h1(h), b.capacity)
	for ; ; seq = seq.next() {
		g := b.ctrls.GroupAt(seq.offset)
		match := g.matchH2(h2(h))

		for match != 0 {
			slotIdx := match.first()
			i := seq.offsetAt(slotIdx)
			slot := b.slots.At(i)
			if key == slot.key {
				return slot.value, true
			}
			match = match.remove(slotIdx)
		}

		match = g.matchEmpty()
		if match != 0 {
			return value, false
		}
	}
}

// Delete deletes the entry corresponding to the specified key from the map.
// It is a noop to delete a non-existent key.
func (m *Map[K, V]) Delete(key K) {
	// Delete is find composed with "deleted at": we perform find(key), and
	// then delete at the resulting slot if found.
	h := m.hash(noescape(unsafe.Pointer(&key)), m.seed)
	b := m.bucket(h)

	// NB: Unlike the abseil swiss table implementation which uses a common
	// find routine for Get, Put, and Delete, we have to manually inline the
	// find routine for performance.
	seq := makeProbeSeq(h1(h), b.capacity)
	for ; ; seq = seq.next() {
		g := b.ctrls.GroupAt(seq.offset)
		match := g.matchH2(h2(h))

		for match != 0 {
			slotIdx := match.first()
			i := seq.offsetAt(slotIdx)
			s := b.slots.At(i)
			if key == s.key {
				b.used--
				m.used--
				*s = Slot[K, V]{}

				// Given an offset to delete we simply create a tombstone and
				// destroy its contents and mark the ctrl as deleted. If we
				// can prove that the slot would not appear in a probe
				// sequence we can mark the slot as empty instead. We can
				// prove this by checking to see if the slot is part of any
				// group that could have been full (assuming we never create
				// an empty slot in a group with no empties which this
				// heuristic guarantees we never do). If the slot is always
				// parts of groups that could never have been full then find
				// would stop at this slot since we do not probe beyond groups
				// with empties.
				if b.wasNeverFull(i) {
					b.setCtrl(i, ctrlEmpty)
					b.growthLeft++
				} else {
					b.setCtrl(i, ctrlDeleted)
				}
				b.checkInvariants(m)
				return
			}
			match = match.remove(slotIdx)
		}

		match = g.matchEmpty()
		if match != 0 {
			b.checkInvariants(m)
			return
		}
	}
}

// Clear deletes all entries from the map resulting in an empty map.
func (m *Map[K, V]) Clear() {
	m.buckets(0, func(b *bucket[K, V]) bool {
		for i := uintptr(0); i < b.capacity; i++ {
			b.setCtrl(i, ctrlEmpty)
			*b.slots.At(i) = Slot[K, V]{}
		}

		b.used = 0
		b.resetGrowthLeft()
		return true
	})

	// Reset the hash seed to make it more difficult for attackers to
	// repeatedly trigger hash collisions. See issue
	// https://github.com/golang/go/issues/25237.
	m.seed = uintptr(fastrand64())
	m.used = 0
}

// All calls yield sequentially for each key and value present in the map. If
// yield returns false, range stops the iteration. The map can be mutated
// during iteration, though there is no guarantee that the mutations will be
// visible to the iteration.
//
// TODO(peter): The naming of All and its signature are meant to conform to
// the range-over-function Go proposal. When that proposal is accepted (which
// seems likely), we'll be able to iterate over the map by doing:
//
//	for k, v := range m.All {
//	  fmt.Printf("%v: %v\n", k, v)
//	}
//
// See https://github.com/golang/go/issues/61897.
func (m *Map[K, V]) All(yield func(key K, value V) bool) {
	// Randomize iteration order by starting iteration at a random bucket and
	// within each bucket at a random offset.
	offset := uintptr(fastrand64())
	m.buckets(offset>>32, func(b *bucket[K, V]) bool {
		if b.used == 0 {
			return true
		}

		// Snapshot the capacity, controls, and slots so that iteration remains
		// valid if the map is resized during iteration.
		capacity := b.capacity
		ctrls := b.ctrls
		slots := b.slots

		for i := uintptr(0); i <= capacity; i++ {
			// Match full entries which have a high-bit of zero.
			j := (i + offset) & capacity
			if (ctrls.Get(j) & ctrlEmpty) != ctrlEmpty {
				s := slots.At(j)
				if !yield(s.key, s.value) {
					return false
				}
			}
		}
		return true
	})
}

// GoString implements the fmt.GoStringer interface which is used when
// formatting using the "%#v" format specifier.
func (m *Map[K, V]) GoString() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "used=%d  global-depth=%d\n", m.used, m.globalDepth())
	m.buckets(0, func(b *bucket[K, V]) bool {
		fmt.Fprintf(&buf, "bucket %d: local-depth=%d  ", b.index, b.localDepth)
		b.goFormat(&buf)
		return true
	})
	return buf.String()
}

// Len returns the number of entries in the map.
func (m *Map[K, V]) Len() int {
	return m.used
}

// capacity returns the total capacity of all map buckets.
func (m *Map[K, V]) capacity() int {
	var capacity int
	m.buckets(0, func(b *bucket[K, V]) bool {
		capacity += int(b.capacity)
		return true
	})
	return capacity
}

const (
	// ptrSize and shiftMask are used to optimize code generation for
	// Map.bucket(), Map.bucketCount(), and bucketStep(). This technique was
	// lifted from the Go runtime's runtime/map.go:bucketShift() routine. Note
	// that ptrSize will be either 4 on 32-bit archs or 8 on 64-bit archs.
	ptrSize   = 4 << (^uintptr(0) >> 63)
	shiftMask = ptrSize*8 - 1
)

// bucket returns the bucket corresponding to hash value h.
func (m *Map[K, V]) bucket(h uintptr) *bucket[K, V] {
	// NB: It is faster to check for the single bucket case using a
	// conditional than to to index into the directory.
	if m.globalShift == 0 {
		return &m.bucket0
	}
	// When shifting by a variable amount the Go compiler inserts overflow
	// checks that the shift is less than the maximum allowed (32 or 64).
	// Masking the shift amount allows overflow checks to be elided.
	return *m.dir.At(h >> (m.globalShift & shiftMask))
}

// buckets calls yield sequentially for each bucket in the map. If yield
// returns false, iteration stops. Offset specifies the bucket to start
// iteration at (used to randomize iteration order).
func (m *Map[K, V]) buckets(offset uintptr, yield func(b *bucket[K, V]) bool) {
	if m.globalShift == 0 {
		yield(&m.bucket0)
		return
	}

	// Loop termination is handled by remembering the start bucket and exiting
	// when it is reached again. Note that when a bucket is split the existing
	// bucket always ends up earlier in the directory so we'll reach it before
	// we reach any of the buckets that were split off.
	startBucket := *m.dir.At(offset & (m.bucketCount() - 1))
	for b := startBucket; ; {
		originalLocalDepth := b.localDepth

		if !yield(b) {
			break
		}

		// The size of the directory can grow if the yield function mutates
		// the map.  We want to iterate over each bucket once, and if a bucket
		// splits while we're iterating over it we want to skip over all of
		// the buckets newly split from the one we're iterating over. We do
		// this by snapshotting the bucket's local depth and using the
		// snapshotted local depth to compute the bucket step.
		//
		// Note that b.index will also change if the directory grows. Consider
		// the directory below with a globalDepth of 2 containing 4 buckets,
		// each of which has a localDepth of 2.
		//
		//    dir   b.index   b.localDepth
		//	+-----+---------+--------------+
		//	|  00 |       0 |            2 |
		//	+-----+---------+--------------+
		//	|  01 |       1 |            2 |
		//	+-----+---------+--------------+
		//	|  10 |       2 |            2 | <--- iteration point
		//	+-----+---------+--------------+
		//	|  11 |       3 |            2 |
		//	+-----+---------+--------------+
		//
		// If the directory grows during iteration, the index of the bucket
		// we're iterating over will change. If the bucket we're iterating
		// over split, then the local depth will have increased. Notice how
		// the bucket that was previously at index 1 now is at index 2 and is
		// pointed to by 2 directory entries: 010 and 011. The bucket being
		// iterated over which was previously at index 2 is now at index 4.
		// Iteration within a bucket takes a snapshot of the controls and
		// slots to make sure we don't miss keys during iteration or iterate
		// over keys more than once. But we also need to take care of the case
		// where the bucket we're iterating over splits. In this case, we need
		// to skip over the bucket at index 5 which can be done by computing
		// the bucketStep using the bucket's depth prior to calling yield
		// which in this example will be 1<<(3-2)==2.
		//
		//    dir   b.index   b.localDepth
		//	+-----+---------+--------------+
		//	| 000 |       0 |            2 |
		//	+-----+         |              |
		//	| 001 |         |              |
		//	+-----+---------+--------------+
		//	| 010 |       2 |            2 |
		//	+-----+         |              |
		//	| 011 |         |              |
		//	+-----+---------+--------------+
		//	| 100 |       4 |            3 |
		//	+-----+---------+--------------+
		//	| 101 |       5 |            3 |
		//	+-----+---------+--------------+
		//	| 110 |       6 |            2 |
		//	+-----+         |              |
		//	| 111 |         |              |
		//	+-----+---------+--------------+

		i := (b.index + bucketStep(m.globalDepth(), originalLocalDepth)) & (m.bucketCount() - 1)
		b = *m.dir.At(i)
		if b == startBucket {
			break
		}
	}
}

// dirEntries calls yield sequentially for every entry in the directory. If
// yield returns false, iteration stops.
func (m *Map[K, V]) dirEntries(yield func(b *bucket[K, V]) bool) {
	if m.globalShift == 0 {
		yield(&m.bucket0)
		return
	}

	for i, n := uintptr(0), m.bucketCount(); i < n; i++ {
		yield(*m.dir.At(i))
	}
}

// globalDepth returns the number of bits from the top of the hash to use for
// indexing in the buckets directory.
func (m *Map[K, V]) globalDepth() uint {
	if m.globalShift == 0 {
		return 0
	}
	return 64 - m.globalShift
}

// bucketCount returns the number of buckets in the buckets directory.
func (m *Map[K, V]) bucketCount() uintptr {
	return uintptr(1) << (m.globalDepth() & shiftMask)
}

// bucketStep is the number of buckets to step over in the buckets directory
// to reach the next different bucket. A bucket occupies 1 or more contiguous
// entries in the buckets directory specified by the range:
//
//	[b.index,b.index+bucketStep(m.globalDepth(), b.localDepth))
func bucketStep(globalDepth, localDepth uint) uintptr {
	return uintptr(1) << ((globalDepth - localDepth) & shiftMask)
}

// installBucket installs a bucket into the buckets directory, overwriting
// every index in the range of entries the bucket occupies.
func (m *Map[K, V]) installBucket(b *bucket[K, V]) *bucket[K, V] {
	if m.globalShift == 0 {
		m.bucket0 = *b
		return &m.bucket0
	}

	step := bucketStep(m.globalDepth(), b.localDepth)
	for i := uintptr(0); i < step; i++ {
		*m.dir.At(b.index + i) = b
	}
	return b
}

// growDirectory grows the directory slice to 1<<newGlobalDepth buckets.
func (m *Map[K, V]) growDirectory(newGlobalDepth uint) {
	if invariants && newGlobalDepth > 32 {
		panic(fmt.Sprintf("invariant failed: expectedly large newGlobalDepth %d->%d",
			m.globalDepth(), newGlobalDepth))
	}

	newDir := makeUnsafeSlice(make([]*bucket[K, V], 1<<newGlobalDepth))

	// NB: It would be more natural to use Map.buckets() here, but that
	// routine uses b.index during iteration which we're mutating in the loop
	// below.
	var last *bucket[K, V]
	i := uintptr(0)
	m.dirEntries(func(b *bucket[K, V]) bool {
		if b == last {
			return true
		}
		last = b
		b.index = i
		step := bucketStep(newGlobalDepth, b.localDepth)
		for j := uintptr(0); j < step; j++ {
			*newDir.At(i + j) = b
		}
		i += step
		return true
	})

	m.dir = newDir
	m.globalShift = 64 - newGlobalDepth

	m.checkInvariants()
}

// checkInvariants verifies the internal consistency of the map's structure,
// checking conditions that should always be true for a correctly functioning
// map. If any of these invariants are violated, it panics, indicating a bug
// in the map implementation.
func (m *Map[K, V]) checkInvariants() {
	if invariants {
		if m.globalShift == 0 {
			if m.dir.ptr != nil {
				panic("unexpectedly non-nil directory")
			}
			if m.bucket0.localDepth != 0 {
				panic(fmt.Sprintf("expected local-depth=0, but found %d", m.bucket0.localDepth))
			}
		} else {
			i := uintptr(0)
			m.dirEntries(func(b *bucket[K, V]) bool {
				if b == nil {
					panic(fmt.Sprintf("dir[%d]: nil bucket", i))
				}
				if b.localDepth > m.globalDepth() {
					panic(fmt.Sprintf("dir[%d]: local-depth=%d is greater than global-depth=%d",
						i, b.localDepth, m.globalDepth()))
				}
				n := uintptr(1) << (m.globalDepth() - b.localDepth)
				if i < b.index || i >= b.index+n {
					panic(fmt.Sprintf("dir[%d]: out of expected range [%d,%d)", i, b.index, b.index+n))
				}
				i++
				return true
			})
		}
	}
}

func (b *bucket[K, V]) close(allocator Allocator[K, V]) {
	if b.capacity > 0 {
		allocator.FreeSlots(b.slots.Slice(0, b.capacity))
		allocator.FreeControls(unsafeConvertSlice[uint8](b.ctrls.Slice(0, b.capacity+groupSize)))
		b.capacity = 0
		b.used = 0
	}
	b.ctrls = makeCtrlBytes(nil)
	b.slots = makeUnsafeSlice([]Slot[K, V](nil))
}

// setCtrl sets the control byte at index i, taking care to mirror the byte to
// the end of the control bytes slice if i<groupSize.
func (b *bucket[K, V]) setCtrl(i uintptr, v ctrl) {
	*b.ctrls.At(i) = v
	// Mirror the first groupSize control state to the end of the ctrls slice.
	// We do this unconditionally which is faster than performing a comparison
	// to do it only for the first groupSize slots. Note that the index will
	// be the identity for slots in the range [groupSize,capacity).
	*b.ctrls.At(((i - (groupSize - 1)) & b.capacity) + (groupSize - 1)) = v
}

// tombstones returns the number of deleted (tombstone) entries in the bucket.
// A tombstone is a slot that has been deleted but is still considered
// occupied so as not to violate the probing invariant.
func (b *bucket[K, V]) tombstones() uintptr {
	return (b.capacity*maxAvgGroupLoad)/groupSize - uintptr(b.used)
}

// wasNeverFull returns true if index i was never part a full group. This
// check allows an optimization during deletion whereby a deleted slot can be
// converted to empty rather than a tombstone. See the comment in Delete for
// further explanation.
func (b *bucket[K, V]) wasNeverFull(i uintptr) bool {
	if b.capacity < groupSize {
		// The map fits entirely in a single group so we will never probe
		// beyond this group.
		return true
	}

	indexBefore := (i - groupSize) & b.capacity
	emptyAfter := b.ctrls.GroupAt(i).matchEmpty()
	emptyBefore := b.ctrls.GroupAt(indexBefore).matchEmpty()

	// We're looking at the control bytes on either side of i trying to determine
	// if the control byte i ever overlapped with a group that was full:
	//
	//   xx xx xx xx xx xx xx xx  xx xx xx xx xx xx xx xx
	//   ^                        ^
	//   indexBefore              i
	//
	// We count how many consecutive non empties we have to the right of i
	// (including i) and to the left of i (not including i). If the sum is >=
	// groupSize then there is at least one probe window that might have seen a
	// full group.
	//
	// The empty{Before,After} != 0 checks are a quick test to see if the group
	// starting at indexBefore and i are completely full (TODO: are these quick
	// checks worthwhile, they aren't necessary for correctness).
	if emptyBefore != 0 && emptyAfter != 0 &&
		emptyBefore.absentAtEnd()+emptyAfter.absentAtStart() < groupSize {
		return true
	}
	return false
}

// uncheckedPut inserts an entry known not to be in the table. Used by Put
// after it has failed to find an existing entry to overwrite duration
// insertion.
func (b *bucket[K, V]) uncheckedPut(h uintptr, key K, value V) {
	if invariants && b.growthLeft == 0 {
		panic("invariant failed: growthLeft is unexpectedly 0")
	}

	// Given key and its hash hash(key), to insert it, we construct a
	// probeSeq, and use it to find the first group with an unoccupied (empty
	// or deleted) slot. We place the key/value into the first such slot in
	// the group and mark it as full with key's H2.
	seq := makeProbeSeq(h1(h), b.capacity)
	for ; ; seq = seq.next() {
		g := b.ctrls.GroupAt(seq.offset)
		match := g.matchEmptyOrDeleted()
		if match != 0 {
			i := seq.offsetAt(match.first())
			slot := b.slots.At(i)
			slot.key = key
			slot.value = value
			if b.ctrls.Get(i) == ctrlEmpty {
				b.growthLeft--
			}
			b.setCtrl(i, ctrl(h2(h)))
			return
		}
	}
}

func (b *bucket[K, V]) rehash(m *Map[K, V]) {
	// Rehash in place if we can recover >= 1/3 of the capacity. Note that
	// this heuristic differs from Abseil's and was experimentally determined
	// to balance performance on the PutDelete benchmark vs achieving a
	// reasonable load-factor.
	//
	// Abseil notes that in the worst case it takes ~4 Put/Delete pairs to
	// create a single tombstone. Rehashing in place is significantly faster
	// than resizing because the common case is that elements remain in their
	// current location. The performance of rehashInPlace is dominated by
	// recomputing the hash of every key. We know how much space we're going
	// to reclaim because every tombstone will be dropped and we're only
	// called if we've reached the thresold of capacity/8 empty slots. So the
	// number of tomstones is capacity*7/8 - used.
	if b.capacity > groupSize && b.tombstones() >= b.capacity/3 {
		b.rehashInPlace(m)
		return
	}

	// If the newCapacity is larger than the maxBucketCapacity split the
	// bucket instead of resizing. Each of the new buckets will be the same
	// size as the current bucket.
	newCapacity := 2*b.capacity + 1
	if newCapacity > m.maxBucketCapacity {
		b.split(m)
		return
	}

	b.resize(m, newCapacity)
}

func (b *bucket[K, V]) init(m *Map[K, V], newCapacity uintptr) {
	if (1 + newCapacity) < groupSize {
		newCapacity = groupSize - 1
	}

	b.slots = makeUnsafeSlice(m.allocator.AllocSlots(int(newCapacity)))
	b.ctrls = makeCtrlBytes(unsafeConvertSlice[ctrl](
		m.allocator.AllocControls(int(newCapacity + groupSize))))
	for i := uintptr(0); i < newCapacity+groupSize; i++ {
		*b.ctrls.At(i) = ctrlEmpty
	}
	*b.ctrls.At(newCapacity) = ctrlSentinel

	b.capacity = newCapacity

	b.resetGrowthLeft()
}

// resize the capacity of the table by allocating a bigger array and
// uncheckedPutting each element of the table into the new array (we know that
// no insertion here will Put an already-present value), and discard the old
// backing array.
func (b *bucket[K, V]) resize(m *Map[K, V], newCapacity uintptr) {
	oldCtrls, oldSlots := b.ctrls, b.slots
	oldCapacity := b.capacity
	b.init(m, newCapacity)

	for i := uintptr(0); i < oldCapacity; i++ {
		c := oldCtrls.Get(i)
		if c == ctrlEmpty || c == ctrlDeleted {
			continue
		}
		slot := oldSlots.At(i)
		h := m.hash(noescape(unsafe.Pointer(&slot.key)), m.seed)
		b.uncheckedPut(h, slot.key, slot.value)
	}

	if oldCapacity > 0 {
		m.allocator.FreeSlots(oldSlots.Slice(0, oldCapacity))
		m.allocator.FreeControls(unsafeConvertSlice[uint8](oldCtrls.Slice(0, oldCapacity+groupSize)))
	}

	b.checkInvariants(m)
}

// split divides the entries in a bucket between the receiver and a new bucket
// of the same size, and then installs the new bucket into the buckets
// directory, growing the buckets directory if necessary.
func (b *bucket[K, V]) split(m *Map[K, V]) {
	// Create the new bucket as a clone of the bucket being split.
	newb := &bucket[K, V]{
		localDepth: b.localDepth,
		index:      b.index,
	}
	newb.init(m, b.capacity)

	// Divide the records between the 2 buckets (b and newb). This is done by
	// examining the new bit in the hash that will be added to the bucket
	// index. If that bit is 0 the record stays in bucket b. If that bit is 1
	// the record is moved to bucket newb. We're relying on the bucket b
	// staying earlier in the directory than newb after the directory is
	// grown.
	mask := uintptr(1) << (64 - (b.localDepth + 1))
	for i := uintptr(0); i < b.capacity; i++ {
		c := b.ctrls.Get(i)
		if c == ctrlEmpty || c == ctrlDeleted {
			continue
		}

		slot := b.slots.At(i)
		h := m.hash(noescape(unsafe.Pointer(&slot.key)), m.seed)
		if (h & mask) == 0 {
			// Nothing to do, the record is staying in b.
			continue
		}

		// Insert the record into newb.
		newb.uncheckedPut(h, slot.key, slot.value)
		newb.used++

		// Delete the record from b.
		if b.wasNeverFull(i) {
			b.setCtrl(i, ctrlEmpty)
			b.growthLeft++
		} else {
			b.setCtrl(i, ctrlDeleted)
		}

		*slot = Slot[K, V]{}
		b.used--
	}

	if uintptr(b.used) >= (b.capacity*maxAvgGroupLoad)/groupSize {
		// We didn't move any records to the new bucket. Either
		// maxBucketCapacity is too small and we got unlucky, or we have a
		// degenerate hash function (e.g. one that returns a constant in the
		// high bits).
		m.maxBucketCapacity = 2*m.maxBucketCapacity + 1
		b.resize(m, 2*b.capacity+1)
		return
	}

	if uintptr(newb.used) >= (newb.capacity*maxAvgGroupLoad)/groupSize || newb.growthLeft == 0 {
		// We moved all of the records to the new bucket (note the two
		// conditions are equivalent and both are present merely for clarity).
		// Similar to the above, bump maxBucketCapacity and resize the bucket
		// rather than splitting. We'll replace the old bucket with the new
		// bucket in the directory.
		m.maxBucketCapacity = 2*m.maxBucketCapacity + 1
		newb = m.installBucket(newb)
		m.checkInvariants()
		newb.resize(m, 2*newb.capacity+1)
		return
	}

	// We need to ensure the old which we evacuated records from has empty
	// slots as we may be inserting into it.
	if b.growthLeft == 0 {
		b.rehashInPlace(m)
	}

	// Grow the directory if necessary.
	if b.localDepth >= m.globalDepth() {
		m.growDirectory(b.localDepth + 1)
	}

	// Complete the split by incrementing the local depth for the 2 buckets
	// and installing the new bucket in the directory.
	b.localDepth++
	newb.localDepth++
	newb.index = b.index + bucketStep(m.globalDepth(), b.localDepth)
	m.installBucket(newb)

	if invariants {
		m.checkInvariants()
		m.buckets(0, func(b *bucket[K, V]) bool {
			b.checkInvariants(m)
			return true
		})
	}
}

func (b *bucket[K, V]) rehashInPlace(m *Map[K, V]) {
	// We want to drop all of the deletes in place. We first walk over the
	// control bytes and mark every DELETED slot as EMPTY and every FULL slot
	// as DELETED. Marking the DELETED slots as EMPTY has effectively dropped
	// the tombstones, but we fouled up the probe invariant. Marking the FULL
	// slots as DELETED gives us a marker to locate the previously FULL slots.

	// Mark all DELETED slots as EMPTY and all FULL slots as DELETED.
	for i := uintptr(0); i < b.capacity; i += groupSize {
		b.ctrls.GroupAt(i).convertNonFullToEmptyAndFullToDeleted()
	}

	// Fixup the cloned control bytes and the sentinel.
	for i, n := uintptr(0), uintptr(groupSize-1); i < n; i++ {
		*b.ctrls.At(((i - (groupSize - 1)) & b.capacity) + (groupSize - 1)) = *b.ctrls.At(i)
	}
	*b.ctrls.At(b.capacity) = ctrlSentinel

	// Now we walk over all of the DELETED slots (a.k.a. the previously FULL
	// slots). For each slot we find the first probe group we can place the
	// element in which reestablishes the probe invariant. Note that as this
	// loop proceeds we have the invariant that there are no DELETED slots in
	// the range [0, i). We may move the element at i to the range [0, i) if
	// that is where the first group with an empty slot in its probe chain
	// resides, but we never set a slot in [0, i) to DELETED.
	for i := uintptr(0); i < b.capacity; i++ {
		if b.ctrls.Get(i) != ctrlDeleted {
			continue
		}

		s := b.slots.At(i)
		h := m.hash(noescape(unsafe.Pointer(&s.key)), m.seed)
		seq := makeProbeSeq(h1(h), b.capacity)
		desired := seq

		probeIndex := func(pos uintptr) uintptr {
			return ((pos - desired.offset) & b.capacity) / groupSize
		}

		var target uintptr
		for ; ; seq = seq.next() {
			g := b.ctrls.GroupAt(seq.offset)
			if match := g.matchEmptyOrDeleted(); match != 0 {
				target = seq.offsetAt(match.first())
				break
			}
		}

		if i == target || probeIndex(i) == probeIndex(target) {
			// If the target index falls within the first probe group
			// then we don't need to move the element as it already
			// falls in the best probe position.
			b.setCtrl(i, ctrl(h2(h)))
			continue
		}

		if b.ctrls.Get(target) == ctrlEmpty {
			// The target slot is empty. Transfer the element to the
			// empty slot and mark the slot at index i as empty.
			b.setCtrl(target, ctrl(h2(h)))
			*b.slots.At(target) = *b.slots.At(i)
			*b.slots.At(i) = Slot[K, V]{}
			b.setCtrl(i, ctrlEmpty)
			continue
		}

		if b.ctrls.Get(target) == ctrlDeleted {
			// The slot at target has an element (i.e. it was FULL).
			// We're going to swap our current element with that
			// element and then repeat processing of index i which now
			// holds the element which was at target.
			b.setCtrl(target, ctrl(h2(h)))
			t := b.slots.At(target)
			*s, *t = *t, *s
			// Repeat processing of the i'th slot which now holds a
			// new key/value.
			i--
			continue
		}

		panic(fmt.Sprintf("ctrl at position %d (%02x) should be empty or deleted",
			target, b.ctrls.Get(target)))
	}

	b.resetGrowthLeft()
	b.growthLeft -= b.used

	b.checkInvariants(m)
}

func (b *bucket[K, V]) resetGrowthLeft() {
	if b.capacity < groupSize {
		// If the map fits in a single group then we're able to fill all of
		// the slots except 1 (an empty slot is needed to terminate find
		// operations).
		b.growthLeft = int(b.capacity - 1)
	} else {
		b.growthLeft = int((b.capacity * maxAvgGroupLoad) / groupSize)
	}
}

func (b *bucket[K, V]) checkInvariants(m *Map[K, V]) {
	if invariants {
		if b.capacity > 0 {
			// Verify the cloned control bytes are good.
			for i, n := uintptr(0), uintptr(groupSize-1); i < n; i++ {
				j := ((i - (groupSize - 1)) & b.capacity) + (groupSize - 1)
				ci := b.ctrls.Get(i)
				cj := b.ctrls.Get(j)
				if ci != cj {
					panic(fmt.Sprintf("invariant failed: ctrl(%d)=%02x != ctrl(%d)=%02x\n%#v", i, ci, j, cj, b))
				}
			}
			// Verify the sentinel is good.
			if c := b.ctrls.Get(b.capacity); c != ctrlSentinel {
				panic(fmt.Sprintf("invariant failed: ctrl(%d): expected sentinel, but found %02x\n%#v", b.capacity, c, b))
			}
		}

		// For every non-empty slot, verify we can retrieve the key using Get.
		// Count the number of used and deleted slots.
		var used int
		var deleted int
		var empty int
		for i := uintptr(0); i < b.capacity; i++ {
			c := b.ctrls.Get(i)
			switch {
			case c == ctrlDeleted:
				deleted++
			case c == ctrlEmpty:
				empty++
			case c == ctrlSentinel:
				panic(fmt.Sprintf("invariant failed: ctrl(%d): unexpected sentinel", i))
			default:
				s := b.slots.At(i)
				if _, ok := m.Get(s.key); !ok {
					h := m.hash(noescape(unsafe.Pointer(&s.key)), m.seed)
					panic(fmt.Sprintf("invariant failed: slot(%d): %v not found [h2=%02x h1=%07x]\n%#v",
						i, s.key, h2(h), h1(h), b))
				}
				used++
			}
		}

		if used != b.used {
			panic(fmt.Sprintf("invariant failed: found %d used slots, but used count is %d\n%#v",
				used, b.used, b))
		}

		growthLeft := int((b.capacity*maxAvgGroupLoad)/groupSize-uintptr(b.used)) - deleted
		if growthLeft != b.growthLeft {
			panic(fmt.Sprintf("invariant failed: found %d growthLeft, but expected %d\n%#v",
				b.growthLeft, growthLeft, b))
		}
	}
}

// GoString implements the fmt.GoStringer interface which is used when
// formatting using the "%#v" format specifier.
func (b *bucket[K, V]) GoString() string {
	var buf strings.Builder
	b.goFormat(&buf)
	return buf.String()
}

func (b *bucket[K, V]) goFormat(w io.Writer) {
	fmt.Fprintf(w, "capacity=%d  used=%d  growth-left=%d\n", b.capacity, b.used, b.growthLeft)
	for i := uintptr(0); i < b.capacity+groupSize; i++ {
		switch c := b.ctrls.Get(i); c {
		case ctrlEmpty:
			fmt.Fprintf(w, "  %4d: %02x [empty]\n", i, c)
		case ctrlDeleted:
			fmt.Fprintf(w, "  %4d: %02x [deleted]\n", i, c)
		case ctrlSentinel:
			fmt.Fprintf(w, "  %4d: %02x [sentinel]\n", i, c)
		default:
			s := b.slots.At(i & b.capacity)
			fmt.Fprintf(w, "  %4d: %02x [%v:%v]\n", i, c, s.key, s.value)
		}
	}
}

// bitset represents a set of slots within a group.
//
// The underlying representation uses one byte per slot, where each byte is
// either 0x80 if the slot is part of the set or 0x00 otherwise. This makes it
// convenient to calculate for an entire group at once (e.g. see matchEmpty).
type bitset uint64

// first assumes that only the MSB of each control byte can be set (e.g. bitset
// is the result of matchEmpty or similar) and returns the relative index of the
// first control byte in the group that has the MSB set.
//
// Returns 8 if the bitset is 0.
// Returns groupSize if the bitset is empty.
func (b bitset) first() uintptr {
	return uintptr(bits.TrailingZeros64(uint64(b))) >> 3
}

// Returns the maximal number of contiguous slots at the beginning of the group
// that are NOT in the set.
func (b bitset) absentAtStart() uintptr {
	return b.first()
}

// Returns the maximal number of contiguous slots at the end of the group that
// are NOT in the set.
func (b bitset) absentAtEnd() uintptr {
	return uintptr(bits.LeadingZeros64(uint64(b))) >> 3
}

// remove removes the slot with the given relative index.
func (b bitset) remove(i uintptr) bitset {
	return b &^ (bitset(0x80) << (i << 3))
}

func (b bitset) String() string {
	var buf strings.Builder
	buf.Grow(groupSize)
	for i := 0; i < groupSize; i++ {
		if (b & (bitset(0x80) << (i << 3))) != 0 {
			buf.WriteString("1")
		} else {
			buf.WriteString("0")
		}
	}
	return buf.String()
}

// ctrlGroup contains a group of 8 control bytes (in little-endian). Note that a
// group can start at any control byte (not just those that are 8-byte aligned).
type ctrlGroup uint64

// matchH2 returns the set of slots which are full and for which the 7-bit hash
// matches the given value. May return false positives.
func (g *ctrlGroup) matchH2(h uintptr) bitset {
	// NB: This generic matching routine produces false positive matches when
	// h is 2^N and the control bytes have a seq of 2^N followed by 2^N+1. For
	// example: if ctrls==0x0302 and h=02, we'll compute v as 0x0100. When we
	// subtract off 0x0101 the first 2 bytes we'll become 0xffff and both be
	// considered matches of h. The false positive matches are not a problem,
	// just a rare inefficiency. Note that they only occur if there is a real
	// match and never occur on ctrlEmpty, ctrlDeleted, or ctrlSentinel. The
	// subsequent key comparisons ensure that there is no correctness issue.
	v := uint64(*g) ^ (bitsetLSB * uint64(h))
	return bitset(((v - bitsetLSB) &^ v) & bitsetMSB)
}

// matchEmpty returns the set of slots in the group that are empty.
func (g *ctrlGroup) matchEmpty() bitset {
	// An empty slot is              1000 0000
	// A deleted or sentinel slot is 1111 111?
	// A full slot is                0??? ????
	//
	// A slot is empty iff bit 7 is set and bit 1 is not.
	// We could select any of the other bits here (e.g. v << 1 would also
	// work).
	v := uint64(*g)
	return bitset((v &^ (v << 6)) & bitsetMSB)
}

// matchEmptyOrDeleted returns the set of slots in the group that are empty or
// deleted.
func (g *ctrlGroup) matchEmptyOrDeleted() bitset {
	// An empty slot is  1000 0000.
	// A deleted slot is 1111 1110.
	// The sentinel is   1111 1111.
	// A full slot is    0??? ????
	//
	// A slot is empty or deleted iff bit 7 is set and bit 0 is not.
	v := uint64(*g)
	return bitset((v &^ (v << 7)) & bitsetMSB)
}

// convertNonFullToEmptyAndFullToDeleted converts deleted or sentinel control
// bytes in a group to empty control bytes, and control bytes indicating full
// slots to deleted control bytes.
func (g *ctrlGroup) convertNonFullToEmptyAndFullToDeleted() {
	// An empty slot is     1000 0000
	// A deleted slot is    1111 1110
	// The sentinel slot is 1111 1111
	// A full slot is       0??? ????
	//
	// We select the MSB, invert, add 1 if the MSB was set and zero out the low
	// bit.
	//
	//  - if the MSB was set (i.e. slot was empty, deleted, or sentinel):
	//     v:             1000 0000
	//     ^v:            0111 1111
	//     ^v + (v >> 7): 1000 0000
	//     &^ bitsetLSB:  1000 0000 = empty slot.
	//
	// - if the MSB was not set (i.e. full slot):
	//     v:             0000 0000
	//     ^v:            1111 1111
	//     ^v + (v >> 7): 1111 1111
	//     &^ bitsetLSB:  1111 1110 = deleted slot.
	//
	v := uint64(*g) & bitsetMSB
	*g = ctrlGroup((^v + (v >> 7)) &^ bitsetLSB)
}

// Each slot in the hash table has a control byte which can have one of four
// states: empty, deleted, full and the sentinel. They have the following bit
// patterns:
//
//	   empty: 1 0 0 0 0 0 0 0
//	 deleted: 1 1 1 1 1 1 1 0
//	    full: 0 h h h h h h h  // h represents the H1 hash bits
//	sentinel: 1 1 1 1 1 1 1 1
type ctrl uint8

// ctrlBytes is the slice of control bytes.
type ctrlBytes struct {
	unsafeSlice[ctrl]
}

func makeCtrlBytes(s []ctrl) ctrlBytes {
	return ctrlBytes{unsafeSlice: makeUnsafeSlice(s)}
}

// Get returns the i-th control byte.
func (cb ctrlBytes) Get(i uintptr) ctrl {
	return *(*ctrl)(unsafe.Add(cb.ptr, i))
}

// GroupAt returns a pointer to the group that starts at i. The ctrlGroup
// contains the values of control bytes i through i+7. A group can start at any
// index (it does not have to be 8-byte aligned).
func (cb ctrlBytes) GroupAt(i uintptr) *ctrlGroup {
	return (*ctrlGroup)(unsafe.Add(cb.ptr, i))
}

var emptyCtrls = func() ctrlBytes {
	v := make([]ctrl, groupSize)
	for i := range v {
		v[i] = ctrlEmpty
	}
	return makeCtrlBytes(v)
}()

// probeSeq maintains the state for a probe sequence. The sequence is a
// triangular progression of the form
//
//	p(i) := groupSize * (i^2 + i)/2 + hash (mod mask+1)
//
// The use of groupSize ensures that each probe step does not overlap groups;
// the sequence effectively outputs the addresses of *groups* (although not
// necessarily aligned to any boundary). The group machinery allows us to
// check an entire group with minimal branching.
//
// Wrapping around at mask+1 is important, but not for the obvious reason. As
// described above, the first few entries of the control byte array are
// mirrored at the end of the array, which group will find and use for
// selecting candidates. However, when those candidates' slots are actually
// inspected, there are no corresponding slots for the cloned bytes, so we
// need to make sure we've treated those offsets as "wrapping around".
//
// It turns out that this probe sequence visits every group exactly once if
// the number of groups is a power of two, since (i^2+i)/2 is a bijection in
// Z/(2^m). See https://en.wikipedia.org/wiki/Quadratic_probing
type probeSeq struct {
	mask   uintptr
	offset uintptr
	index  uintptr
}

func makeProbeSeq(hash, mask uintptr) probeSeq {
	return probeSeq{
		mask:   mask,
		offset: hash & mask,
		index:  0,
	}
}

func (s probeSeq) next() probeSeq {
	s.index += groupSize
	s.offset = (s.offset + s.index) & s.mask
	return s
}

func (s probeSeq) offsetAt(i uintptr) uintptr {
	return (s.offset + i) & s.mask
}

func (s probeSeq) String() string {
	return fmt.Sprintf("mask=%d offset=%d index=%d", s.mask, s.offset, s.index)
}

// Extracts the H1 portion of a hash: the 57 upper bits.
func h1(h uintptr) uintptr {
	return h >> 7
}

// Extracts the H2 portion of a hash: the 7 bits not used for h1.
//
// These are used as an occupied control byte.
func h2(h uintptr) uintptr {
	return h & 0x7f
}

// noescape hides a pointer from escape analysis.  noescape is
// the identity function but escape analysis doesn't think the
// output depends on the input.  noescape is inlined and currently
// compiles down to zero instructions.
// USE CAREFULLY!
//
//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

// unsafeSlice provides semi-ergonomic limited slice-like functionality
// without bounds checking for fixed sized slices.
type unsafeSlice[T any] struct {
	ptr unsafe.Pointer
}

func makeUnsafeSlice[T any](s []T) unsafeSlice[T] {
	return unsafeSlice[T]{ptr: unsafe.Pointer(unsafe.SliceData(s))}
}

// At returns a pointer to the element at index i.
func (s unsafeSlice[T]) At(i uintptr) *T {
	var t T
	return (*T)(unsafe.Add(s.ptr, unsafe.Sizeof(t)*i))
}

// Slice returns a Go slice akin to slice[start:end] for a Go builtin slice.
func (s unsafeSlice[T]) Slice(start, end uintptr) []T {
	return unsafe.Slice((*T)(s.ptr), end)[start:end]
}

func unsafeConvertSlice[Dest any, Src any](s []Src) []Dest {
	return unsafe.Slice((*Dest)(unsafe.Pointer(unsafe.SliceData(s))), len(s))
}
