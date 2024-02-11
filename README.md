# Swiss Map [![Build Status](https://github.com/cockroachdb/swiss/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/cockroachdb/swiss/actions/workflows/ci.yaml) [![GoDoc](https://godoc.org/github.com/cockroachdb/swiss?status.svg)](https://godoc.org/github.com/cockroachdb/swiss)

`swiss.Map` is a Go implementation of [Google's Swiss Tables hash table
design](https://abseil.io/about/design/swisstables). The [Rust version of
Swiss Tables](https://github.com/rust-lang/hashbrown) is now the `HashMap`
implementation in the Rust standard library.

A `swiss.Map[K,V]` maps keys of type `K` to values of type `V`, similar to
Go's builtin `map[K]V` type. The primary advantage of `swiss.Map` over Go's
builtin map is performance. `swiss.Map` has similar or slightly better
performance than Go's builtin map for small map sizes, and significantly
better performance at large map sizes.

```
name                                         old time/op  new time/op  delta
StringMap/avgLoad,n=10/Map/Get-10            10.2ns ±17%   8.3ns ± 0%  -18.62%  (p=0.000 n=10+9)
StringMap/avgLoad,n=83/Map/Get-10            10.9ns ± 9%   8.9ns ±10%  -18.97%  (p=0.000 n=10+10)
StringMap/avgLoad,n=671/Map/Get-10           15.4ns ± 5%   8.9ns ± 3%  -42.07%  (p=0.000 n=10+10)
StringMap/avgLoad,n=5375/Map/Get-10          25.8ns ± 1%  11.4ns ± 1%  -56.01%  (p=0.000 n=10+9)
StringMap/avgLoad,n=86015/Map/Get-10         30.2ns ± 1%  12.5ns ± 1%  -58.68%  (p=0.000 n=9+9)
Int64Map/avgLoad,n=10/Map/Get-10             5.00ns ± 0%  4.78ns ± 1%   -4.33%  (p=0.000 n=9+9)
Int64Map/avgLoad,n=83/Map/Get-10             5.19ns ± 2%  5.26ns ±17%     ~     (p=0.353 n=10+10)
Int64Map/avgLoad,n=671/Map/Get-10            6.37ns ±10%  5.39ns ± 7%  -15.39%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=5375/Map/Get-10           17.9ns ± 2%   6.7ns ± 2%  -62.80%  (p=0.000 n=9+9)
Int64Map/avgLoad,n=86015/Map/Get-10          23.8ns ± 0%   8.0ns ± 0%  -66.12%  (p=0.000 n=10+8)

name                                         old time/op  new time/op  delta
StringMap/avgLoad,n=10/Map/PutDelete-10      26.3ns ±13%  28.0ns ±21%     ~     (p=0.133 n=10+9)
StringMap/avgLoad,n=83/Map/PutDelete-10      30.8ns ± 8%  33.9ns ± 8%  +10.14%  (p=0.000 n=10+10)
StringMap/avgLoad,n=671/Map/PutDelete-10     45.3ns ± 1%  35.2ns ± 5%  -22.30%  (p=0.000 n=10+10)
StringMap/avgLoad,n=5375/Map/PutDelete-10    56.5ns ± 1%  41.2ns ± 3%  -27.03%  (p=0.000 n=10+10)
StringMap/avgLoad,n=86015/Map/PutDelete-10   60.4ns ± 0%  45.7ns ± 1%  -24.24%  (p=0.000 n=10+9)
Int64Map/avgLoad,n=10/Map/PutDelete-10       18.1ns ± 6%  16.5ns ±13%   -8.84%  (p=0.002 n=9+9)
Int64Map/avgLoad,n=83/Map/PutDelete-10       19.7ns ±10%  20.4ns ± 9%     ~     (p=0.110 n=10+10)
Int64Map/avgLoad,n=671/Map/PutDelete-10      27.0ns ± 2%  24.8ns ± 7%   -7.82%  (p=0.000 n=9+10)
Int64Map/avgLoad,n=5375/Map/PutDelete-10     43.9ns ± 1%  32.2ns ± 3%  -26.63%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=86015/Map/PutDelete-10    50.4ns ± 0%  35.6ns ± 1%  -29.25%  (p=0.000 n=10+8)
```

On top of the base Swiss Tables design, `swiss.Map` adds an [extendible
hashing](https://en.wikipedia.org/wiki/Extendible_hashing) layer in order to
enable incremental resizing of large maps.

## Caveats

- The implementation currently requires a little endian CPU architecture. This
  is not a fundamental limitation of the implementation, merely a choice of
  expediency.
- Go's builtin map has a fast-path for comparing strings that [share their
  underlying
  storage](https://github.com/golang/go/blob/4a7f3ac8eb4381ea62caa1741eeeec28363245b4/src/runtime/map_faststr.go#L100).
  This fast-path is feasible because `map[string]T` is specialized for string
  keys which isn't currently possible with Go's generics.

## TODO

- Add support for SIMD searching on x86 and [8-byte Neon SIMD searching on
  arm64](https://github.com/abseil/abseil-cpp/commit/6481443560a92d0a3a55a31807de0cd712cd4f88)
  - This appears to be somewhat difficult. Naively implementing the match
    routines in assembly isn't workable as the function call overhead
    dominates the performance improvement from the SIMD comparisons. The
    Abseil implementation is leveraring gcc/llvm assembly intrinsics which are
    not currently available in Go. In order to take advantage of SIMD we'll
    have to write most/all of the probing loop in assembly.
