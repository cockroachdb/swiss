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
name                        old time/op  new time/op  delta
StringMaps/n=16/map-10      7.19ns ± 3%  7.28ns ± 0%     ~     (p=0.154 n=9+9)
StringMaps/n=128/map-10     7.66ns ± 5%  7.37ns ± 3%   -3.74%  (p=0.008 n=10+9)
StringMaps/n=1024/map-10    10.8ns ± 3%   7.6ns ± 3%  -29.76%  (p=0.000 n=10+10)
StringMaps/n=8192/map-10    20.3ns ± 2%   7.9ns ± 1%  -61.16%  (p=0.000 n=10+10)
StringMaps/n=131072/map-10  26.1ns ± 0%  14.0ns ± 1%  -46.56%  (p=0.000 n=10+10)
Int64Maps/n=16/map-10       4.96ns ± 1%  4.83ns ± 0%   -2.73%  (p=0.000 n=9+9)
Int64Maps/n=128/map-10      5.19ns ± 3%  4.89ns ± 5%   -5.80%  (p=0.000 n=10+10)
Int64Maps/n=1024/map-10     6.80ns ± 5%  5.01ns ± 2%  -26.32%  (p=0.000 n=10+10)
Int64Maps/n=8192/map-10     17.4ns ± 1%   5.3ns ± 0%  -69.59%  (p=0.000 n=10+7)
Int64Maps/n=131072/map-10   20.6ns ± 0%   6.7ns ± 0%  -67.67%  (p=0.000 n=10+9)
```

## Caveats

- Resizing a `swiss.Map` is done for the whole table rather than the
incremental resizing performed by Go's builtin map.

## TODO

- Add correctness tests.
- Add support for rehash in-place.
- Add support for SIMD searching on x86.
- Add support for 8-byte Neon SIMD searching:
  https://community.arm.com/arm-community-blogs/b/infrastructure-solutions-blog/posts/porting-x86-vector-bitmask-optimizations-to-arm-neon
  https://github.com/abseil/abseil-cpp/commit/6481443560a92d0a3a55a31807de0cd712cd4f88
- Abstract out the slice allocations so we can use manual memory allocation
  when used inside Pebble.
- Benchmark insertion and deletion.
- Add a note on thread safety (there isn't any).
- Add a note that a little endian system is required, and a test that asserts that.
- Explore extendible hashing to allow incremental resizing. See
  https://github.com/golang/go/issues/54766#issuecomment-1233125048
