# Swiss Map [WORK IN PROGRESS; DO NOT USE]

`swiss.Map` is a Go implementation of [Google's Swiss Tables hash table
design](https://abseil.io/about/design/swisstables). The [Rust version of
Swiss Tables](https://github.com/rust-lang/hashbrown) is now the `HashMap`
implementation in the Rust standard library.

A `swiss.Map[K,V]` maps keys of type `K` to values of type `V`, similar to
Go's builtin `map[K]V` type. The primary advantage of `swiss.Map` over Go's
builtin map is performance. `swiss.Map` has similar or slightly better
performance Go's builtin map for small map sizes, and significantly better
performance at large map sizes.

```
name                                         old time/op  new time/op  delta
StringMap/avgLoad,n=10/Map/Get-10            9.53ns ± 6%  8.43ns ± 1%  -11.50%  (p=0.000 n=10+9)
StringMap/avgLoad,n=83/Map/Get-10            11.0ns ± 9%   9.2ns ±11%  -16.57%  (p=0.000 n=10+10)
StringMap/avgLoad,n=671/Map/Get-10           15.7ns ± 3%   9.0ns ± 3%  -42.31%  (p=0.000 n=10+10)
StringMap/avgLoad,n=5375/Map/Get-10          25.8ns ± 1%   9.3ns ± 1%  -63.88%  (p=0.000 n=10+10)
StringMap/avgLoad,n=86015/Map/Get-10         30.5ns ± 1%  10.9ns ± 2%  -64.34%  (p=0.000 n=9+10)
Int64Map/avgLoad,n=10/Map/Get-10             5.11ns ± 3%  4.85ns ± 1%   -5.13%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=83/Map/Get-10             5.23ns ± 3%  5.18ns ± 7%     ~     (p=0.529 n=10+10)
Int64Map/avgLoad,n=671/Map/Get-10            6.03ns ± 7%  5.36ns ± 5%  -11.08%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=5375/Map/Get-10           18.3ns ± 2%   5.7ns ± 2%  -68.76%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=86015/Map/Get-10          23.9ns ± 1%   6.9ns ± 0%  -71.24%  (p=0.000 n=10+9)

name                                         old time/op  new time/op  delta
StringMap/avgLoad,n=10/Map/PutDelete-10      26.3ns ±11%  23.3ns ± 2%  -11.41%  (p=0.000 n=10+8)
StringMap/avgLoad,n=83/Map/PutDelete-10      31.6ns ± 7%  23.4ns ± 4%  -25.94%  (p=0.000 n=10+10)
StringMap/avgLoad,n=671/Map/PutDelete-10     45.2ns ± 1%  23.5ns ± 1%  -47.96%  (p=0.000 n=10+9)
StringMap/avgLoad,n=5375/Map/PutDelete-10    56.7ns ± 1%  24.3ns ± 3%  -57.25%  (p=0.000 n=10+10)
StringMap/avgLoad,n=86015/Map/PutDelete-10   60.9ns ± 0%  38.9ns ± 3%  -36.17%  (p=0.000 n=9+10)
Int64Map/avgLoad,n=10/Map/PutDelete-10       18.4ns ± 9%  15.8ns ±12%  -13.99%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=83/Map/PutDelete-10       19.6ns ± 4%  14.7ns ± 1%  -25.14%  (p=0.000 n=9+8)
Int64Map/avgLoad,n=671/Map/PutDelete-10      27.1ns ± 2%  14.2ns ± 3%  -47.52%  (p=0.000 n=10+9)
Int64Map/avgLoad,n=5375/Map/PutDelete-10     44.4ns ± 1%  16.0ns ± 2%  -63.93%  (p=0.000 n=10+8)
Int64Map/avgLoad,n=86015/Map/PutDelete-10    50.6ns ± 0%  21.6ns ± 3%  -57.41%  (p=0.000 n=9+10)
```

## Caveats

- Resizing a `swiss.Map` is done for the whole table rather than the
  incremental resizing performed by Go's builtin map.
- The implementation currently requires a little endian CPU architecture. This
  is not a fundamental limitation of the implementation, merely a choice of
  expediency.

## TODO

- Add support for rehash in-place.
- Add support for SIMD searching on x86 and (8-byte Neon SIMD searching on
  arm64)[https://github.com/abseil/abseil-cpp/commit/6481443560a92d0a3a55a31807de0cd712cd4f88]
  - This appears to be somewhat difficult. Naively implementing the match
    routines in assembly isn't workable as the function call overhead
    dominates the performance improvement form the SIMD comparisons. The
    Abseil implementation is leveraring gcc/llvm assembly intrinsics which are
    not currently available in Go. In order to take advantage of SIMD we'll
    have to write most/all of the probing loop in assembly.
- Abstract out the slice allocations so we can use manual memory allocation
  when used inside Pebble.
- Explore extendible hashing to allow incremental resizing. See
  https://github.com/golang/go/issues/54766#issuecomment-1233125048
