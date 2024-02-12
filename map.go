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
// map for small map sizes, and is much faster at large map sizes (old=go-map,
// new=swissmap):
//
//	name                                         old time/op  new time/op  delta
//	StringMap/avgLoad,n=10/Map/Get-10            9.46ns ± 4%  8.43ns ± 1%  -10.89%  (p=0.000 n=10+9)
//	StringMap/avgLoad,n=83/Map/Get-10            10.9ns ± 7%   8.9ns ±12%  -18.45%  (p=0.000 n=10+10)
//	StringMap/avgLoad,n=671/Map/Get-10           15.4ns ± 3%   9.1ns ± 3%  -40.98%  (p=0.000 n=10+10)
//	StringMap/avgLoad,n=5375/Map/Get-10          25.8ns ± 1%   9.3ns ± 1%  -63.83%  (p=0.000 n=10+9)
//	StringMap/avgLoad,n=86015/Map/Get-10         30.4ns ± 1%  10.8ns ± 1%  -64.49%  (p=0.000 n=9+9)
//	Int64Map/avgLoad,n=10/Map/Get-10             5.05ns ± 2%  4.87ns ± 1%   -3.60%  (p=0.000 n=10+10)
//	Int64Map/avgLoad,n=83/Map/Get-10             5.27ns ± 5%  5.29ns ±12%     ~     (p=0.912 n=10+10)
//	Int64Map/avgLoad,n=671/Map/Get-10            6.14ns ± 4%  5.35ns ± 3%  -12.85%  (p=0.000 n=10+10)
//	Int64Map/avgLoad,n=5375/Map/Get-10           18.4ns ± 4%   5.7ns ± 2%  -68.94%  (p=0.000 n=10+10)
//	Int64Map/avgLoad,n=86015/Map/Get-10          23.9ns ± 0%   6.9ns ± 0%  -71.35%  (p=0.000 n=10+8)
//
//	name                                         old time/op  new time/op  delta
//	StringMap/avgLoad,n=10/Map/PutDelete-10      25.4ns ± 6%  23.7ns ± 8%   -6.43%  (p=0.004 n=10+10)
//	StringMap/avgLoad,n=83/Map/PutDelete-10      31.4ns ± 7%  24.3ns ±12%  -22.66%  (p=0.000 n=10+10)
//	StringMap/avgLoad,n=671/Map/PutDelete-10     45.4ns ± 3%  24.9ns ± 4%  -45.21%  (p=0.000 n=10+10)
//	StringMap/avgLoad,n=5375/Map/PutDelete-10    56.7ns ± 1%  24.7ns ± 2%  -56.44%  (p=0.000 n=10+10)
//	StringMap/avgLoad,n=86015/Map/PutDelete-10   60.8ns ± 1%  31.6ns ± 2%  -48.03%  (p=0.000 n=9+9)
//	Int64Map/avgLoad,n=10/Map/PutDelete-10       18.0ns ± 3%  17.1ns ±34%     ~     (p=0.095 n=9+10)
//	Int64Map/avgLoad,n=83/Map/PutDelete-10       19.8ns ± 3%  14.6ns ±12%  -26.11%  (p=0.000 n=9+9)
//	Int64Map/avgLoad,n=671/Map/PutDelete-10      27.2ns ± 3%  15.2ns ± 6%  -44.02%  (p=0.000 n=10+10)
//	Int64Map/avgLoad,n=5375/Map/PutDelete-10     44.5ns ± 0%  16.9ns ± 3%  -62.10%  (p=0.000 n=7+10)
//	Int64Map/avgLoad,n=86015/Map/PutDelete-10    50.8ns ± 0%  21.0ns ± 1%  -58.65%  (p=0.000 n=10+10)
package swiss

import (
	"fmt"
	"math/bits"
	"strings"
	"unsafe"
)

const (
	debug = false

	groupSize       = 8
	maxAvgGroupLoad = 7

	ctrlEmpty    ctrl = 0b10000000
	ctrlDeleted  ctrl = 0b11111110
	ctrlSentinel ctrl = 0b11111111

	bitsetLSB     = 0x0101010101010101
	bitsetMSB     = 0x8080808080808080
	bitsetEmpty   = bitsetLSB * uint64(ctrlEmpty)
	bitsetDeleted = bitsetLSB * uint64(ctrlDeleted)
)

// Slot holds a key and value.
type Slot[K comparable, V any] struct {
	key   K
	value V
}

// Map is an unordered map from keys to values with Put, Get, Delete, and All
// operations. It is inspired by Google's Swiss Tables design as implemented
// in Abseil's flat_hash_map. By default, a Map[K,V] uses the same hash
// function as Go's builtin map[K]V, though a different hash function can be
// specified using the WithHash option.
//
// A Map is NOT goroutine-safe.
type Map[K comparable, V any] struct {
	// ctrls is capacity+groupSize in length. Ctrls[capacity] is always
	// ctrlSentinel which is used to stop probe iteration. A copy of the first
	// groupSize-1 elements of ctrls is mirrored into the remaining slots
	// which is done so that a probe sequence which picks a value near the end
	// of ctrls will have valid control bytes to look at.
	//
	// When the map is empty, ctrls points to emptyCtrls which will never be
	// modified and is used to simplify the Put, Get, and Delete code which
	// doesn't have to check for a nil ctrls.
	ctrls ctrlBytes
	// slots is capacity in length.
	slots unsafeSlice[Slot[K, V]]
	// The hash function to each keys of type K. The hash function is
	// extracted from the Go runtime's implementation of map[K]struct{}.
	hash hashFn
	seed uintptr
	// The allocator to use for the ctrls and slots slices.
	allocator Allocator[K, V]
	// The total number slots (always 2^N-1). The capacity is used as a mask
	// to quickly compute i%N using a bitwise & operation.
	capacity uintptr
	// The number of filled slots (i.e. the number of elements in the map).
	used int
	// The number of slots we can still fill without needing to rehash.
	//
	// This is stored separately due to tombstones: we do not include
	// tombstones in the growth capacity because we'd like to rehash when the
	// table is filled with tombstones as otherwise probe sequences might get
	// unacceptably long without triggering a rehash.
	growthLeft int
}

// New constructs a new M with the specified initial capacity. If
// initialCapacity is 0 the map will start out with zero capacity and will
// grow on the first insert. The zero value for an M is not usable.
func New[K comparable, V any](initialCapacity int, options ...option[K, V]) *Map[K, V] {
	// The ctrls for an empty map points to emptyCtrls which simplifies
	// probing in Get, Put, and Delete. The emptyCtrls never match a probe
	// operation, but because growthLeft == 0 if we try to insert we'll
	// immediately rehash and grow.
	m := &Map[K, V]{
		ctrls:     emptyCtrls,
		hash:      getRuntimeHasher[K](),
		seed:      uintptr(fastrand64()),
		allocator: defaultAllocator[K, V]{},
	}

	for _, op := range options {
		op.apply(m)
	}

	if initialCapacity > 0 {
		// targetCapacity is the smallest value of the form 2^k-1 that is >=
		// initialCapacity.
		targetCapacity := (uintptr(1) << bits.Len(uint(initialCapacity))) - 1
		m.resize(targetCapacity)
	}
	m.checkInvariants()
	return m
}

// Close closes the map, releasing any memory back to its configured
// allocator. It is unnecessary to close a map using the default allocator. It
// is invalid to use a Map after it has been closed, though Close itself is
// idempotent.
func (m *Map[K, V]) Close() {
	if m.capacity > 0 {
		m.allocator.FreeSlots(m.slots.Slice(0, m.capacity))
		m.allocator.FreeControls(unsafeConvertSlice[uint8](m.ctrls.Slice(0, m.capacity+groupSize)))
		m.capacity = 0
		m.used = 0
	}
	m.ctrls = makeCtrlBytes(nil)
	m.slots = makeUnsafeSlice([]Slot[K, V](nil))
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

	// NB: Unlike the abseil swiss table implementation which uses a common
	// find routine for Get, Put, and Delete, we have to manually inline the
	// find routine for performance.
	seq := makeProbeSeq(h1(h), m.capacity)
	if debug {
		fmt.Printf("put(%v): %s\n", key, seq)
	}

	for ; ; seq = seq.next() {
		g := m.ctrls.GroupAt(seq.offset)
		match := g.matchH2(h2(h))
		if debug {
			fmt.Printf("put(probing): offset=%d h2=%02x match=%s [% 02x]\n",
				seq.offset, h2(h), match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
		}

		for match != 0 {
			slotIdx := match.first()
			i := seq.offsetAt(slotIdx)
			if debug {
				fmt.Printf("put(checking): index=%d  key=%v\n", i, m.slots.At(i).key)
			}
			slot := m.slots.At(i)
			if key == slot.key {
				if debug {
					fmt.Printf("put(updating): index=%d  key=%v\n", i, key)
				}
				slot.value = value
				m.checkInvariants()
				return
			}
			match = match.remove(slotIdx)
		}

		match = g.matchEmpty()
		if match != 0 {
			if debug {
				fmt.Printf("put(not-found): offset=%d match-empty=%s [% 02x]\n",
					seq.offset, match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
			}
			m.uncheckedPut(h, key, value)
			m.used++
			m.checkInvariants()
			return
		}

		if debug {
			fmt.Printf("put(skipping): offset=%d match-empty=%s [% 02x]\n",
				seq.offset, match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
		}
	}
}

// Get retrieves the value from the map for the specified key, return ok=false
// if the key is not present.
func (m *Map[K, V]) Get(key K) (value V, ok bool) {
	h := m.hash(noescape(unsafe.Pointer(&key)), m.seed)

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
	seq := makeProbeSeq(h1(h), m.capacity)
	if debug {
		fmt.Printf("get(%v): %s\n", key, seq)
	}

	for ; ; seq = seq.next() {
		g := m.ctrls.GroupAt(seq.offset)
		match := g.matchH2(h2(h))
		if debug {
			fmt.Printf("get(probing): offset=%d h2=%02x match=%s [% 02x]\n",
				seq.offset, h2(h), match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
		}

		for match != 0 {
			slotIdx := match.first()
			i := seq.offsetAt(slotIdx)
			if debug {
				fmt.Printf("get(checking): index=%d  key=%v\n", i, m.slots.At(i).key)
			}
			slot := m.slots.At(i)
			if key == slot.key {
				return slot.value, true
			}
			match = match.remove(slotIdx)
		}

		match = g.matchEmpty()
		if match != 0 {
			if debug {
				fmt.Printf("get(not-found): offset=%d match-empty=%s [% 02x]\n",
					seq.offset, match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
			}
			return value, false
		}

		if debug {
			fmt.Printf("get(skipping): offset=%d match-empty=%s [% 02x]\n",
				seq.offset, match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
		}
	}
}

// Delete deletes the entry corresponding to the specified key from the map.
// It is a noop to delete a non-existent key.
func (m *Map[K, V]) Delete(key K) {
	// Delete is find composed with "deleted at": we perform find(key), and
	// then delete at the resulting slot if found.
	h := m.hash(noescape(unsafe.Pointer(&key)), m.seed)

	// NB: Unlike the abseil swiss table implementation which uses a common
	// find routine for Get, Put, and Delete, we have to manually inline the
	// find routine for performance.
	seq := makeProbeSeq(h1(h), m.capacity)
	if debug {
		fmt.Printf("delete(%v): %s\n", key, seq)
	}

	for ; ; seq = seq.next() {
		g := m.ctrls.GroupAt(seq.offset)
		match := g.matchH2(h2(h))
		if debug {
			fmt.Printf("delete(probing): offset=%d h2=%02x match=%s [% 02x]\n",
				seq.offset, h2(h), match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
		}

		for match != 0 {
			slotIdx := match.first()
			i := seq.offsetAt(slotIdx)
			if debug {
				fmt.Printf("delete(checking): index=%d  key=%v\n", i, m.slots.At(i).key)
			}
			s := m.slots.At(i)
			if key == s.key {
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
				if m.wasNeverFull(i) {
					m.setCtrl(i, ctrlEmpty)
					m.growthLeft++

					if debug {
						fmt.Printf("delete(%v): index=%d used=%d growth-left=%d\n",
							key, i, m.used, m.growthLeft)
					}
				} else {
					m.setCtrl(i, ctrlDeleted)

					if debug {
						fmt.Printf("delete(%v): index=%d used=%d\n", key, i, m.used)
					}
				}
				m.checkInvariants()
				return
			}
			match = match.remove(slotIdx)
		}

		match = g.matchEmpty()
		if match != 0 {
			if debug {
				fmt.Printf("delete(not-found): offset=%d match-empty=%s [% 02x]\n",
					seq.offset, match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
			}
			m.checkInvariants()
			return
		}

		if debug {
			fmt.Printf("delete(skipping): offset=%d match-empty=%s [% 02x]\n",
				seq.offset, match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
		}
	}
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
	// Snapshot the capacity, controls, and slots so that iteration remains
	// valid if the map is resized during iteration.
	capacity := m.capacity
	ctrls := m.ctrls
	slots := m.slots

	for i := uintptr(0); i < capacity; i++ {
		// Match full entries which have a high-bit of zero.
		if (ctrls.Get(i) & ctrlEmpty) != ctrlEmpty {
			s := slots.At(i)
			if !yield(s.key, s.value) {
				return
			}
		}
	}
}

// Len returns the number of entries in the map.
func (m *Map[K, V]) Len() int {
	return m.used
}

// setCtrl sets the control byte at index i, taking care to mirror the byte to
// the end of the control bytes slice if i<groupSize.
func (m *Map[K, V]) setCtrl(i uintptr, v ctrl) {
	*m.ctrls.At(i) = v
	// Mirror the first groupSize control state to the end of the ctrls slice.
	// We do this unconditionally which is faster than performing a comparison
	// to do it only for the first groupSize slots. Note that the index will
	// be the identity for slots in the range [groupSize,capacity).
	*m.ctrls.At(((i - (groupSize - 1)) & m.capacity) + (groupSize - 1)) = v
}

// wasNeverFull returns true if index i was never part a full group. This
// check allows an optimization during deletion whereby a deleted slot can be
// converted to empty rather than a tombstone. See the comment in Delete for
// further explanation.
func (m *Map[K, V]) wasNeverFull(i uintptr) bool {
	if m.capacity < groupSize {
		// The map fits entirely in a single group so we will never probe
		// beyond this group.
		return true
	}

	indexBefore := (i - groupSize) & m.capacity
	emptyAfter := m.ctrls.GroupAt(i).matchEmpty()
	emptyBefore := m.ctrls.GroupAt(indexBefore).matchEmpty()
	if debug {
		fmt.Printf("wasNeverFull: before=%d/%s/%d after=%d/%s/%d\n",
			indexBefore, emptyBefore, emptyBefore.absentAtEnd(),
			i, emptyAfter, emptyAfter.absentAtStart())
	}

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
func (m *Map[K, V]) uncheckedPut(h uintptr, key K, value V) {
	// Before performing the insertion we may decide the table is getting
	// overcrowded (i.e. the load factor is greater than 7/8 for big tables;
	// small tables use a max load factor of 1).
	if m.growthLeft == 0 {
		m.rehash()
	}

	// Given key and its hash hash(key), to insert it, we construct a
	// probeSeq, and use it to find the first group with an unoccupied (empty
	// or deleted) slot. We place the key/value into the first such slot in
	// the group and mark it as full with key's H2.
	seq := makeProbeSeq(h1(h), m.capacity)
	if debug {
		fmt.Printf("put(%v,%v): %s\n", key, value, seq)
	}

	for ; ; seq = seq.next() {
		g := m.ctrls.GroupAt(seq.offset)
		match := g.matchEmptyOrDeleted()
		if debug {
			fmt.Printf("put(probing): offset=%d match-empty=%s [% 02x]\n",
				seq.offset, match, m.ctrls.Slice(seq.offset, seq.offset+groupSize))
		}

		if match != 0 {
			i := seq.offsetAt(match.first())
			slot := m.slots.At(i)
			slot.key = key
			slot.value = value
			if m.ctrls.Get(i) == ctrlEmpty {
				m.growthLeft--
			}
			m.setCtrl(i, ctrl(h2(h)))
			if debug {
				fmt.Printf("put(inserting): index=%d used=%d growth-left=%d\n", i, m.used+1, m.growthLeft)
			}
			return
		}
	}
}

func (m *Map[K, V]) rehash() {
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

	recoverable := (m.capacity*maxAvgGroupLoad)/groupSize - uintptr(m.used)
	if m.capacity > groupSize && recoverable >= m.capacity/3 {
		m.rehashInPlace()
	} else {
		m.resize(2*m.capacity + 1)
	}
}

// resize resize the capacity of the table by allocating a bigger array and
// uncheckedPutting each element of the table into the new array (we know that
// no insertion here will Put an already-present value), and discard the old
// backing array.
func (m *Map[K, V]) resize(newCapacity uintptr) {
	if (1 + newCapacity) < groupSize {
		newCapacity = groupSize - 1
	}

	oldCtrls, oldSlots := m.ctrls, m.slots
	m.slots = makeUnsafeSlice(m.allocator.AllocSlots(int(newCapacity)))
	m.ctrls = makeCtrlBytes(unsafeConvertSlice[ctrl](
		m.allocator.AllocControls(int(newCapacity + groupSize))))
	for i := uintptr(0); i < newCapacity+groupSize; i++ {
		*m.ctrls.At(i) = ctrlEmpty
	}
	*m.ctrls.At(newCapacity) = ctrlSentinel

	if newCapacity < groupSize {
		// If the map fits in a single group then we're able to fill all of
		// the slots except 1 (an empty slot is needed to terminate find
		// operations).
		m.growthLeft = int(newCapacity - 1)
	} else {
		m.growthLeft = int((newCapacity * maxAvgGroupLoad) / groupSize)
	}

	oldCapacity := m.capacity
	m.capacity = newCapacity

	if debug {
		fmt.Printf("resize: capacity=%d->%d  growth-left=%d\n",
			oldCapacity, newCapacity, m.growthLeft)
	}

	for i := uintptr(0); i < oldCapacity; i++ {
		c := *oldCtrls.At(i)
		if c == ctrlEmpty || c == ctrlDeleted {
			continue
		}
		slot := oldSlots.At(i)
		h := m.hash(noescape(unsafe.Pointer(&slot.key)), m.seed)
		m.uncheckedPut(h, slot.key, slot.value)
	}

	if oldCapacity > 0 {
		m.allocator.FreeSlots(oldSlots.Slice(0, oldCapacity))
		m.allocator.FreeControls(unsafeConvertSlice[uint8](oldCtrls.Slice(0, oldCapacity+groupSize)))
	}

	m.checkInvariants()
}

func (m *Map[K, V]) rehashInPlace() {
	if debug {
		fmt.Printf("rehash: %d/%d\n", m.used, m.capacity)
		for i := uintptr(0); i < m.capacity; i++ {
			switch m.ctrls.Get(i) {
			case ctrlEmpty:
				fmt.Printf("  %d: empty\n", i)
			case ctrlDeleted:
				fmt.Printf("  %d: deleted\n", i)
			case ctrlSentinel:
				fmt.Printf("  %d: sentinel\n", i)
			default:
				fmt.Printf("  %d: %v\n", i, m.slots.At(i).key)
			}
		}
	}

	// We want to drop all of the deletes in place. We first walk over the
	// control bytes and mark every DELETED slot as EMPTY and every FULL slot
	// as DELETED. Marking the DELETED slots as EMPTY has effectively dropped
	// the tombstones, but we fouled up the probe invariant. Marking the FULL
	// slots as DELETED gives us a marker to locate the previously FULL slots.

	// Mark all DELETED slots as EMPTY and all FULL slots as DELETED.
	for i := uintptr(0); i < m.capacity; i += groupSize {
		m.ctrls.GroupAt(i).convertNonFullToEmptyAndFullToDeleted()
	}

	// Fixup the cloned control bytes and the sentinel.
	for i, n := uintptr(0), uintptr(groupSize-1); i < n; i++ {
		*m.ctrls.At(((i - (groupSize - 1)) & m.capacity) + (groupSize - 1)) = *m.ctrls.At(i)
	}
	*m.ctrls.At(m.capacity) = ctrlSentinel

	// Now we walk over all of the DELETED slots (a.k.a. the previously FULL
	// slots). For each slot we find the first probe group we can place the
	// element in which reestablishes the probe invariant. Note that as this
	// loop proceeds we have the invariant that there are no DELETED slots in
	// the range [0, i). We may move the element at i to the range [0, i) if
	// that is where the first group with an empty slot in its probe chain
	// resides, but we never set a slot in [0, i) to DELETED.
	for i := uintptr(0); i < m.capacity; i++ {
		if m.ctrls.Get(i) != ctrlDeleted {
			continue
		}

		s := m.slots.At(i)
		h := m.hash(noescape(unsafe.Pointer(&s.key)), m.seed)
		seq := makeProbeSeq(h1(h), m.capacity)
		desired := seq

		probeIndex := func(pos uintptr) uintptr {
			return ((pos - desired.offset) & m.capacity) / groupSize
		}

		var target uintptr
		for ; ; seq = seq.next() {
			g := m.ctrls.GroupAt(seq.offset)
			if match := g.matchEmptyOrDeleted(); match != 0 {
				target = seq.offsetAt(match.first())
				break
			}
		}

		if i == target || probeIndex(i) == probeIndex(target) {
			if debug {
				fmt.Printf("rehash: %d not moving\n", i)
			}
			// If the target index falls within the first probe group
			// then we don't need to move the element as it already
			// falls in the best probe position.
			m.setCtrl(i, ctrl(h2(h)))
			continue
		}

		if m.ctrls.Get(target) == ctrlEmpty {
			if debug {
				fmt.Printf("rehash: %d -> %d replacing empty\n", i, target)
			}
			// The target slot is empty. Transfer the element to the
			// empty slot and mark the slot at index i as empty.
			m.setCtrl(target, ctrl(h2(h)))
			*m.slots.At(target) = *m.slots.At(i)
			*m.slots.At(i) = Slot[K, V]{}
			m.setCtrl(i, ctrlEmpty)
			continue
		}

		if m.ctrls.Get(target) == ctrlDeleted {
			if debug {
				fmt.Printf("rehash: %d -> %d swapping\n", i, target)
			}
			// The slot at target has an element (i.e. it was FULL).
			// We're going to swap our current element with that
			// element and then repeat processing of index i which now
			// holds the element which was at target.
			m.setCtrl(target, ctrl(h2(h)))
			t := m.slots.At(target)
			*s, *t = *t, *s
			// Repeat processing of the i'th slot which now holds a
			// new key/value.
			i--
			continue
		}

		panic(fmt.Sprintf("ctrl at position %d (%02x) should be empty or deleted",
			target, m.ctrls.Get(target)))
	}

	m.growthLeft = int((m.capacity*maxAvgGroupLoad)/groupSize) - m.used

	if debug {
		fmt.Printf("rehash: done: used=%d growth-left=%d\n", m.used, m.growthLeft)
		for i := uintptr(0); i < m.capacity; i++ {
			switch m.ctrls.Get(i) {
			case ctrlEmpty:
				fmt.Printf("  %d: empty\n", i)
			case ctrlDeleted:
				fmt.Printf("  %d: deleted\n", i)
			case ctrlSentinel:
				fmt.Printf("  %d: sentinel\n", i)
			default:
				s := m.slots.At(i)
				h := m.hash(noescape(unsafe.Pointer(&s.key)), m.seed)
				fmt.Printf("  %d: %02x/%02x %v\n", i, m.ctrls.Get(i), h2(h), s.key)
			}
		}
	}

	m.checkInvariants()
}

func (m *Map[K, V]) checkInvariants() {
	if invariants {
		if m.capacity > 0 {
			// Verify the cloned control bytes are good.
			for i, n := uintptr(0), uintptr(groupSize-1); i < n; i++ {
				j := ((i - (groupSize - 1)) & m.capacity) + (groupSize - 1)
				ci := m.ctrls.Get(i)
				cj := m.ctrls.Get(j)
				if ci != cj {
					panic(fmt.Sprintf("invariant failed: ctrl(%d)=%02x != ctrl(%d)=%02x\n%s", i, ci, j, cj, m.DebugString()))
				}
			}
			// Verify the sentinel is good.
			if c := m.ctrls.Get(m.capacity); c != ctrlSentinel {
				panic(fmt.Sprintf("invariant failed: ctrl(%d): expected sentinel, but found %02x\n%s", m.capacity, c, m.DebugString()))
			}
		}

		// For every non-empty slot, verify we can retrieve the key using Get.
		// Count the number of used and deleted slots.
		var used int
		var deleted int
		var empty int
		for i := uintptr(0); i < m.capacity; i++ {
			c := m.ctrls.Get(i)
			switch {
			case c == ctrlDeleted:
				deleted++
			case c == ctrlEmpty:
				empty++
			case c == ctrlSentinel:
				panic(fmt.Sprintf("invariant failed: ctrl(%d): unexpected sentinel", i))
			default:
				s := m.slots.At(i)
				if _, ok := m.Get(s.key); !ok {
					h := m.hash(noescape(unsafe.Pointer(&s.key)), m.seed)
					panic(fmt.Sprintf("invariant failed: slot(%d): %v not found [h2=%02x h1=%07x]\n%s",
						i, s.key, h2(h), h1(h), m.DebugString()))
				}
				used++
			}
		}

		if used != m.used {
			panic(fmt.Sprintf("invariant failed: found %d used slots, but used count is %d\n%s",
				used, m.used, m.DebugString()))
		}

		growthLeft := int((m.capacity*maxAvgGroupLoad)/groupSize-uintptr(m.used)) - deleted
		if growthLeft != m.growthLeft {
			panic(fmt.Sprintf("invariant failed: found %d growthLeft, but expected %d\n%s",
				m.growthLeft, growthLeft, m.DebugString()))
		}
	}
}

func (m *Map[K, V]) DebugString() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "capacity=%d  used=%d  growth-left=%d\n", m.capacity, m.used, m.growthLeft)
	for i := uintptr(0); i < m.capacity+groupSize; i++ {
		switch c := m.ctrls.Get(i); c {
		case ctrlEmpty:
			fmt.Fprintf(&buf, "  %4d: empty\n", i)
		case ctrlDeleted:
			fmt.Fprintf(&buf, "  %4d: deleted\n", i)
		case ctrlSentinel:
			fmt.Fprintf(&buf, "  %4d: sentinel\n", i)
		default:
			if i < m.capacity {
				s := m.slots.At(i)
				h := m.hash(noescape(unsafe.Pointer(&s.key)), m.seed)
				fmt.Fprintf(&buf, "  %4d: %v [ctrl=%02x h2=%02x] \n", i, s.key, c, h2(h))
			} else {
				fmt.Fprintf(&buf, "  %4d: [ctrl=%02x]\n", i, c)
			}
		}
	}
	return buf.String()
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

// Group returns the value of group that starts at i. The ctrlGroup contains the
// values of control bytes i through i+7. A group can start at any index (it
// does not have to be 8-byte aligned).
func (cb ctrlBytes) Group(i uintptr) ctrlGroup {
	return *(*ctrlGroup)(unsafe.Add(cb.ptr, i))
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
