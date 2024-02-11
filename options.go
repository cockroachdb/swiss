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

import "unsafe"

// option provide an interface to do work on Map while it is being created.
type option[K comparable, V any] interface {
	apply(m *Map[K, V])
}

type hashOption[K comparable, V any] struct {
	hash func(key *K, seed uintptr) uintptr
}

func (op hashOption[K, V]) apply(m *Map[K, V]) {
	m.hash = *(*hashFn)(noescape(unsafe.Pointer(&op.hash)))
}

// WithHash is an option to specify the hash function to use for a Map[K,V].
func WithHash[K comparable, V any](hash func(key *K, seed uintptr) uintptr) option[K, V] {
	return hashOption[K, V]{hash}
}

// Allocator specifies an interface for allocating and releasing memory used
// by a Map. The default allocator utilizes Go's builtin make() and allows the
// GC to reclaim memory.
//
// If the allocator is manually managing memory and requires that slots and
// controls be freed then Map.Close must be called in order to ensure
// FreeSlots and FreeControls are called.
type Allocator[K comparable, V any] interface {
	// AllocSlots should return a slice equivalent to make([]Slot[K,V], n).
	AllocSlots(n int) []Slot[K, V]

	// AllocControls should return a slice equivalent to make([]uint8, n).
	AllocControls(n int) []uint8

	// FreeSlots can optional release the memory associated with the supplied
	// slice that is guaranteed to have been allocated by AllocSlots.
	FreeSlots(v []Slot[K, V])

	// FreeControls can optional release the memory associated with the
	// supplied slice that is guaranteed to have been allocated by
	// AllocControls.
	FreeControls(v []uint8)
}

type defaultAllocator[K comparable, V any] struct{}

func (defaultAllocator[K, V]) AllocSlots(n int) []Slot[K, V] {
	return make([]Slot[K, V], n)
}

func (defaultAllocator[K, V]) AllocControls(n int) []uint8 {
	return make([]uint8, n)
}

func (defaultAllocator[K, V]) FreeSlots(v []Slot[K, V]) {
}

func (defaultAllocator[K, V]) FreeControls(v []uint8) {
}

type allocatorOption[K comparable, V any] struct {
	allocator Allocator[K, V]
}

func (op allocatorOption[K, V]) apply(m *Map[K, V]) {
	m.allocator = op.allocator
}

// WithAllocator is an option for specify the Allocator to use for a Map[K,V].
func WithAllocator[K comparable, V any](allocator Allocator[K, V]) option[K, V] {
	return allocatorOption[K, V]{allocator}
}
