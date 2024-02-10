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
StringMap/avgLoad,n=10/Map/Get-10            9.46ns ± 4%  8.43ns ± 1%  -10.89%  (p=0.000 n=10+9)
StringMap/avgLoad,n=83/Map/Get-10            10.9ns ± 7%   8.9ns ±12%  -18.45%  (p=0.000 n=10+10)
StringMap/avgLoad,n=671/Map/Get-10           15.4ns ± 3%   9.1ns ± 3%  -40.98%  (p=0.000 n=10+10)
StringMap/avgLoad,n=5375/Map/Get-10          25.8ns ± 1%   9.3ns ± 1%  -63.83%  (p=0.000 n=10+9)
StringMap/avgLoad,n=86015/Map/Get-10         30.4ns ± 1%  10.8ns ± 1%  -64.49%  (p=0.000 n=9+9)
Int64Map/avgLoad,n=10/Map/Get-10             5.05ns ± 2%  4.87ns ± 1%   -3.60%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=83/Map/Get-10             5.27ns ± 5%  5.29ns ±12%     ~     (p=0.912 n=10+10)
Int64Map/avgLoad,n=671/Map/Get-10            6.14ns ± 4%  5.35ns ± 3%  -12.85%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=5375/Map/Get-10           18.4ns ± 4%   5.7ns ± 2%  -68.94%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=86015/Map/Get-10          23.9ns ± 0%   6.9ns ± 0%  -71.35%  (p=0.000 n=10+8)

name                                         old time/op  new time/op  delta
StringMap/avgLoad,n=10/Map/PutDelete-10      25.4ns ± 6%  23.7ns ± 8%   -6.43%  (p=0.004 n=10+10)
StringMap/avgLoad,n=83/Map/PutDelete-10      31.4ns ± 7%  24.3ns ±12%  -22.66%  (p=0.000 n=10+10)
StringMap/avgLoad,n=671/Map/PutDelete-10     45.4ns ± 3%  24.9ns ± 4%  -45.21%  (p=0.000 n=10+10)
StringMap/avgLoad,n=5375/Map/PutDelete-10    56.7ns ± 1%  24.7ns ± 2%  -56.44%  (p=0.000 n=10+10)
StringMap/avgLoad,n=86015/Map/PutDelete-10   60.8ns ± 1%  31.6ns ± 2%  -48.03%  (p=0.000 n=9+9)
Int64Map/avgLoad,n=10/Map/PutDelete-10       18.0ns ± 3%  17.1ns ±34%     ~     (p=0.095 n=9+10)
Int64Map/avgLoad,n=83/Map/PutDelete-10       19.8ns ± 3%  14.6ns ±12%  -26.11%  (p=0.000 n=9+9)
Int64Map/avgLoad,n=671/Map/PutDelete-10      27.2ns ± 3%  15.2ns ± 6%  -44.02%  (p=0.000 n=10+10)
Int64Map/avgLoad,n=5375/Map/PutDelete-10     44.5ns ± 0%  16.9ns ± 3%  -62.10%  (p=0.000 n=7+10)
Int64Map/avgLoad,n=86015/Map/PutDelete-10    50.8ns ± 0%  21.0ns ± 1%  -58.65%  (p=0.000 n=10+10)
```

## Caveats

- Resizing a `swiss.Map` is done for the whole table rather than the
  incremental resizing performed by Go's builtin map.
- The implementation currently requires a little endian CPU architecture. This
  is not a fundamental limitation of the implementation, merely a choice of
  expediency.
- Go's builtin map has a fast-path for comparing strings that [share their
  underlying
  storage](https://github.com/golang/go/blob/4a7f3ac8eb4381ea62caa1741eeeec28363245b4/src/runtime/map_faststr.go#L100).
  This fast-path is feasible because `map[string]T` is specialized which isn't
  currently possible with Go's generics.

## TODO

- Add support for SIMD searching on x86 and [8-byte Neon SIMD searching on
  arm64](https://github.com/abseil/abseil-cpp/commit/6481443560a92d0a3a55a31807de0cd712cd4f88)
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
