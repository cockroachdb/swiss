# Swiss Map [![Build Status](https://github.com/cockroachdb/swiss/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/cockroachdb/swiss/actions/workflows/ci.yaml) [![GoDoc](https://godoc.org/github.com/cockroachdb/swiss?status.svg)](https://godoc.org/github.com/cockroachdb/swiss)

`swiss.Map` is a Go implementation of [Google's Swiss Tables hash table
design](https://abseil.io/about/design/swisstables). The [Rust version of
Swiss Tables](https://github.com/rust-lang/hashbrown) is now the `HashMap`
implementation in the Rust standard library.

A `swiss.Map[K,V]` maps keys of type `K` to values of type `V`, similar to
Go's builtin `map[K]V` type. The primary advantage of `swiss.Map` over Go's
builtin map is performance. `swiss.Map` has similar or slightly better
performance than Go's builtin map for small map sizes, and significantly
better performance at large map sizes. The benchmarks where `swiss.Map`
performs worse are due to a fast-path optimization in Go's builtin maps for
tiny (6 or fewer element) maps with int32, int64, or string keys, and when
growing a map. In the latter case, Go's builtin map is somewhat more
parsimonious with allocations at small map sizes and the performance hit of
the extra allocation dominates in the operations.

<details>
<summary>Performance: Go's builtin map (old) vs swiss.Map (new)</summary>

Tested on a GCE `n1-standard-16` instance.

```
goos: linux
goarch: amd64
pkg: github.com/cockroachdb/swiss
cpu: Intel(R) Xeon(R) CPU @ 2.60GHz

name                               old time/op    new time/op    delta
MapIter/Int/6-16                     70.3ns ± 0%    39.2ns ± 3%    -44.25%  (p=0.000 n=9+9)
MapIter/Int/12-16                     120ns ± 1%      58ns ± 2%    -51.72%  (p=0.000 n=8+9)
MapIter/Int/18-16                     187ns ± 3%      90ns ± 2%    -51.99%  (p=0.000 n=10+10)
MapIter/Int/24-16                     223ns ± 3%      96ns ± 1%    -57.17%  (p=0.000 n=10+10)
MapIter/Int/30-16                     301ns ± 3%     134ns ± 1%    -55.38%  (p=0.000 n=9+10)
MapIter/Int/64-16                     584ns ± 3%     237ns ± 0%    -59.51%  (p=0.000 n=10+8)
MapIter/Int/128-16                   1.15µs ± 2%    0.42µs ± 1%    -63.03%  (p=0.000 n=9+10)
MapIter/Int/256-16                   2.40µs ± 4%    0.80µs ± 1%    -66.55%  (p=0.000 n=10+10)
MapIter/Int/512-16                   5.12µs ± 2%    1.55µs ± 1%    -69.68%  (p=0.000 n=10+10)
MapIter/Int/1024-16                  10.9µs ± 2%     3.0µs ± 1%    -72.13%  (p=0.000 n=10+10)
MapIter/Int/2048-16                  22.4µs ± 1%     6.1µs ± 0%    -72.82%  (p=0.000 n=9+10)
MapIter/Int/4096-16                  45.3µs ± 2%    23.4µs ± 4%    -48.38%  (p=0.000 n=10+10)
MapIter/Int/8192-16                  90.6µs ± 1%    71.0µs ± 1%    -21.65%  (p=0.000 n=9+10)
MapIter/Int/65536-16                  724µs ± 1%     663µs ± 1%     -8.43%  (p=0.000 n=9+9)
MapGetHit/Int64/6-16                 4.93ns ± 0%    6.54ns ± 0%    +32.72%  (p=0.000 n=10+8)
MapGetHit/Int64/12-16                7.33ns ± 3%    6.57ns ± 1%    -10.34%  (p=0.000 n=9+8)
MapGetHit/Int64/18-16                8.16ns ± 7%    6.56ns ± 1%    -19.57%  (p=0.000 n=9+9)
MapGetHit/Int64/24-16                7.36ns ± 2%    6.81ns ± 8%     -7.38%  (p=0.000 n=10+10)
MapGetHit/Int64/30-16                6.93ns ± 1%    6.55ns ± 0%     -5.54%  (p=0.000 n=10+8)
MapGetHit/Int64/64-16                7.00ns ± 1%    6.69ns ± 5%     -4.43%  (p=0.000 n=9+10)
MapGetHit/Int64/128-16               7.06ns ± 2%    6.83ns ± 4%     -3.25%  (p=0.001 n=10+10)
MapGetHit/Int64/256-16               7.12ns ± 2%    6.73ns ± 3%     -5.47%  (p=0.000 n=10+10)
MapGetHit/Int64/512-16               7.11ns ± 2%    6.70ns ± 3%     -5.70%  (p=0.000 n=10+10)
MapGetHit/Int64/1024-16              7.22ns ± 1%    6.82ns ± 2%     -5.45%  (p=0.000 n=9+10)
MapGetHit/Int64/2048-16              8.93ns ± 5%    7.06ns ± 1%    -20.94%  (p=0.000 n=9+10)
MapGetHit/Int64/4096-16              16.3ns ± 2%     8.7ns ± 0%    -46.43%  (p=0.000 n=9+9)
MapGetHit/Int64/8192-16              19.5ns ± 2%     9.2ns ± 0%    -52.86%  (p=0.000 n=10+10)
MapGetHit/Int64/65536-16             25.7ns ± 1%    14.0ns ± 1%    -45.57%  (p=0.000 n=10+7)
MapGetHit/Int32/6-16                 5.14ns ± 0%    6.51ns ± 0%    +26.67%  (p=0.000 n=10+8)
MapGetHit/Int32/12-16                7.53ns ± 5%    6.84ns ±15%     -9.14%  (p=0.001 n=10+10)
MapGetHit/Int32/18-16                7.92ns ±17%    6.52ns ± 0%    -17.68%  (p=0.000 n=10+10)
MapGetHit/Int32/24-16                7.51ns ± 7%    6.69ns ± 8%    -10.96%  (p=0.000 n=10+10)
MapGetHit/Int32/30-16                6.97ns ± 4%    6.53ns ± 0%     -6.27%  (p=0.000 n=10+10)
MapGetHit/Int32/64-16                7.13ns ± 5%    6.73ns ± 6%     -5.60%  (p=0.011 n=10+10)
MapGetHit/Int32/128-16               7.18ns ± 2%    6.77ns ± 5%     -5.77%  (p=0.000 n=10+10)
MapGetHit/Int32/256-16               7.21ns ± 3%    6.73ns ± 5%     -6.66%  (p=0.000 n=10+10)
MapGetHit/Int32/512-16               7.19ns ± 1%    6.72ns ± 1%     -6.53%  (p=0.000 n=10+8)
MapGetHit/Int32/1024-16              7.40ns ± 1%    6.68ns ± 1%     -9.68%  (p=0.000 n=8+9)
MapGetHit/Int32/2048-16              9.19ns ± 3%    6.71ns ± 2%    -26.99%  (p=0.000 n=9+10)
MapGetHit/Int32/4096-16              16.1ns ± 2%     8.2ns ± 0%    -49.33%  (p=0.000 n=10+9)
MapGetHit/Int32/8192-16              19.0ns ± 1%     8.6ns ± 1%    -54.73%  (p=0.000 n=10+10)
MapGetHit/Int32/65536-16             23.7ns ± 1%    11.1ns ± 2%    -53.21%  (p=0.000 n=10+10)
MapGetHit/String/6-16                13.0ns ± 0%     9.3ns ± 0%    -28.55%  (p=0.000 n=8+10)
MapGetHit/String/12-16               9.55ns ±10%    9.29ns ± 0%       ~     (p=0.173 n=10+8)
MapGetHit/String/18-16               10.7ns ± 7%     9.9ns ±13%     -8.01%  (p=0.050 n=10+10)
MapGetHit/String/24-16               9.82ns ±11%   10.86ns ± 2%    +10.60%  (p=0.002 n=10+8)
MapGetHit/String/30-16               8.96ns ± 4%    9.35ns ± 1%     +4.34%  (p=0.000 n=10+9)
MapGetHit/String/64-16               9.20ns ± 4%    9.53ns ± 7%     +3.58%  (p=0.027 n=10+10)
MapGetHit/String/128-16              9.72ns ± 2%    9.46ns ± 3%     -2.62%  (p=0.001 n=10+9)
MapGetHit/String/256-16              10.6ns ± 2%     9.5ns ± 2%     -9.98%  (p=0.000 n=10+10)
MapGetHit/String/512-16              11.2ns ± 1%     9.6ns ± 2%    -14.28%  (p=0.000 n=10+10)
MapGetHit/String/1024-16             11.5ns ± 1%    10.1ns ± 1%    -12.46%  (p=0.000 n=9+10)
MapGetHit/String/2048-16             15.1ns ± 4%    10.2ns ± 1%    -32.59%  (p=0.000 n=10+10)
MapGetHit/String/4096-16             27.5ns ± 1%    12.3ns ± 1%    -55.09%  (p=0.000 n=10+10)
MapGetHit/String/8192-16             30.6ns ± 1%    12.9ns ± 1%    -57.85%  (p=0.000 n=10+10)
MapGetHit/String/65536-16            39.4ns ± 1%    19.2ns ± 3%    -51.39%  (p=0.000 n=10+10)
MapGetMiss/Int64/6-16                6.58ns ± 1%    5.98ns ± 0%     -9.10%  (p=0.000 n=9+8)
MapGetMiss/Int64/12-16               9.73ns ±23%    6.57ns ±12%    -32.50%  (p=0.000 n=10+10)
MapGetMiss/Int64/18-16               8.85ns ± 1%    6.38ns ±11%    -27.89%  (p=0.000 n=8+10)
MapGetMiss/Int64/24-16               12.2ns ±11%     7.0ns ± 3%    -42.27%  (p=0.000 n=9+9)
MapGetMiss/Int64/30-16               8.86ns ± 1%    6.23ns ±10%    -29.69%  (p=0.000 n=10+10)
MapGetMiss/Int64/64-16               8.87ns ± 2%    6.84ns ± 4%    -22.86%  (p=0.000 n=8+10)
MapGetMiss/Int64/128-16              9.59ns ±15%    6.80ns ± 5%    -29.11%  (p=0.000 n=10+10)
MapGetMiss/Int64/256-16              9.41ns ± 6%    7.05ns ± 5%    -25.07%  (p=0.000 n=10+10)
MapGetMiss/Int64/512-16              9.44ns ± 6%    7.08ns ± 4%    -24.99%  (p=0.000 n=10+10)
MapGetMiss/Int64/1024-16             9.24ns ± 3%    7.26ns ± 3%    -21.49%  (p=0.000 n=9+10)
MapGetMiss/Int64/2048-16             10.0ns ± 2%     7.2ns ± 3%    -27.64%  (p=0.000 n=10+10)
MapGetMiss/Int64/4096-16             10.5ns ± 1%    13.9ns ± 5%    +32.92%  (p=0.000 n=10+10)
MapGetMiss/Int64/8192-16             11.1ns ± 1%    15.0ns ±24%    +35.36%  (p=0.000 n=10+10)
MapGetMiss/Int64/65536-16            15.6ns ± 3%    17.4ns ±10%    +11.43%  (p=0.000 n=10+10)
MapGetMiss/Int32/6-16                8.62ns ± 0%    6.13ns ± 6%    -28.86%  (p=0.000 n=9+10)
MapGetMiss/Int32/12-16               10.7ns ± 1%     6.6ns ±11%    -37.98%  (p=0.000 n=8+9)
MapGetMiss/Int32/18-16               10.7ns ± 0%     6.3ns ± 6%    -40.84%  (p=0.000 n=10+10)
MapGetMiss/Int32/24-16               12.6ns ±20%     6.9ns ±10%    -44.81%  (p=0.000 n=10+10)
MapGetMiss/Int32/30-16               10.7ns ± 0%     6.0ns ± 1%    -43.95%  (p=0.000 n=10+8)
MapGetMiss/Int32/64-16               10.7ns ± 0%     6.8ns ± 8%    -36.81%  (p=0.000 n=9+10)
MapGetMiss/Int32/128-16              11.1ns ± 8%     6.9ns ± 4%    -37.67%  (p=0.000 n=10+10)
MapGetMiss/Int32/256-16              11.1ns ± 7%     6.9ns ± 4%    -38.39%  (p=0.000 n=10+10)
MapGetMiss/Int32/512-16              11.0ns ± 4%     7.0ns ± 4%    -36.24%  (p=0.000 n=10+10)
MapGetMiss/Int32/1024-16             11.1ns ± 3%     7.1ns ± 2%    -35.92%  (p=0.000 n=10+10)
MapGetMiss/Int32/2048-16             11.1ns ± 2%     7.1ns ± 2%    -35.74%  (p=0.000 n=10+10)
MapGetMiss/Int32/4096-16             11.2ns ± 3%    14.2ns ± 8%    +26.59%  (p=0.000 n=10+10)
MapGetMiss/Int32/8192-16             11.2ns ± 1%    14.9ns ±25%    +32.64%  (p=0.001 n=10+10)
MapGetMiss/Int32/65536-16            13.7ns ± 1%    17.6ns ± 3%    +28.21%  (p=0.000 n=9+9)
MapGetMiss/String/6-16               7.45ns ± 1%    7.61ns ± 1%     +2.17%  (p=0.000 n=10+8)
MapGetMiss/String/12-16              12.1ns ±15%     8.1ns ±15%    -32.71%  (p=0.000 n=10+10)
MapGetMiss/String/18-16              12.8ns ± 5%     9.2ns ±18%    -28.45%  (p=0.000 n=10+10)
MapGetMiss/String/24-16              14.2ns ±17%     9.5ns ±10%    -33.00%  (p=0.000 n=10+10)
MapGetMiss/String/30-16              11.8ns ± 1%     8.1ns ± 6%    -31.09%  (p=0.000 n=8+10)
MapGetMiss/String/64-16              11.8ns ± 9%     8.5ns ± 6%    -27.58%  (p=0.000 n=10+9)
MapGetMiss/String/128-16             11.8ns ± 5%     8.8ns ± 9%    -25.76%  (p=0.000 n=10+10)
MapGetMiss/String/256-16             11.8ns ± 7%     8.9ns ± 8%    -24.26%  (p=0.000 n=10+10)
MapGetMiss/String/512-16             12.0ns ± 2%     9.0ns ± 6%    -24.90%  (p=0.000 n=10+10)
MapGetMiss/String/1024-16            13.4ns ± 2%     9.0ns ± 3%    -32.64%  (p=0.000 n=10+10)
MapGetMiss/String/2048-16            13.8ns ± 5%     9.1ns ± 1%    -34.32%  (p=0.000 n=10+10)
MapGetMiss/String/4096-16            14.1ns ± 3%    17.9ns ± 3%    +26.71%  (p=0.000 n=10+10)
MapGetMiss/String/8192-16            14.9ns ± 2%    17.6ns ±21%       ~     (p=1.000 n=10+10)
MapGetMiss/String/65536-16           23.8ns ± 2%    21.4ns ±10%    -10.00%  (p=0.000 n=10+10)
MapPutGrow/Int64/6-16                83.5ns ± 0%   210.4ns ± 1%   +151.86%  (p=0.000 n=10+10)
MapPutGrow/Int64/12-16                422ns ± 1%     602ns ± 1%    +42.46%  (p=0.000 n=10+10)
MapPutGrow/Int64/18-16                913ns ± 1%    1277ns ± 0%    +39.92%  (p=0.000 n=10+10)
MapPutGrow/Int64/24-16               1.14µs ± 1%    1.35µs ± 0%    +18.48%  (p=0.000 n=9+10)
MapPutGrow/Int64/30-16               2.02µs ± 1%    2.46µs ± 0%    +21.91%  (p=0.000 n=10+10)
MapPutGrow/Int64/64-16               4.64µs ± 1%    4.97µs ± 1%     +7.16%  (p=0.000 n=10+10)
MapPutGrow/Int64/128-16              9.42µs ± 1%    9.81µs ± 1%     +4.06%  (p=0.000 n=10+10)
MapPutGrow/Int64/256-16              18.4µs ± 1%    19.2µs ± 1%     +4.31%  (p=0.000 n=10+8)
MapPutGrow/Int64/512-16              35.8µs ± 1%    38.0µs ± 1%     +6.17%  (p=0.000 n=10+10)
MapPutGrow/Int64/1024-16             71.0µs ± 1%    74.4µs ± 0%     +4.66%  (p=0.000 n=10+10)
MapPutGrow/Int64/2048-16              141µs ± 1%     148µs ± 1%     +5.03%  (p=0.000 n=10+10)
MapPutGrow/Int64/4096-16              282µs ± 1%     262µs ± 1%     -7.32%  (p=0.000 n=10+10)
MapPutGrow/Int64/8192-16              571µs ± 0%     536µs ± 0%     -6.09%  (p=0.000 n=10+10)
MapPutGrow/Int64/65536-16            5.62ms ± 2%    4.51ms ± 1%    -19.72%  (p=0.000 n=10+10)
MapPutGrow/Int32/6-16                83.1ns ± 0%   200.0ns ± 0%   +140.59%  (p=0.000 n=10+9)
MapPutGrow/Int32/12-16                390ns ± 0%     578ns ± 0%    +48.22%  (p=0.000 n=10+10)
MapPutGrow/Int32/18-16                836ns ± 0%    1221ns ± 1%    +46.08%  (p=0.000 n=9+10)
MapPutGrow/Int32/24-16               1.06µs ± 1%    1.31µs ± 1%    +23.64%  (p=0.000 n=10+10)
MapPutGrow/Int32/30-16               1.83µs ± 0%    2.40µs ± 0%    +31.05%  (p=0.000 n=10+10)
MapPutGrow/Int32/64-16               4.16µs ± 1%    4.72µs ± 0%    +13.46%  (p=0.000 n=10+10)
MapPutGrow/Int32/128-16              8.46µs ± 1%    9.19µs ± 1%     +8.57%  (p=0.000 n=10+10)
MapPutGrow/Int32/256-16              16.8µs ± 0%    18.0µs ± 1%     +7.30%  (p=0.000 n=9+10)
MapPutGrow/Int32/512-16              32.8µs ± 0%    35.6µs ± 1%     +8.63%  (p=0.000 n=10+9)
MapPutGrow/Int32/1024-16             64.3µs ± 0%    69.7µs ± 1%     +8.40%  (p=0.000 n=9+10)
MapPutGrow/Int32/2048-16              128µs ± 0%     138µs ± 1%     +7.80%  (p=0.000 n=10+9)
MapPutGrow/Int32/4096-16              252µs ± 0%     243µs ± 0%     -3.85%  (p=0.000 n=10+10)
MapPutGrow/Int32/8192-16              506µs ± 1%     498µs ± 1%     -1.62%  (p=0.000 n=10+8)
MapPutGrow/Int32/65536-16            4.57ms ± 3%    4.18ms ± 0%     -8.40%  (p=0.000 n=10+10)
MapPutGrow/String/6-16               90.0ns ± 0%   271.6ns ± 1%   +201.82%  (p=0.000 n=10+10)
MapPutGrow/String/12-16               518ns ± 2%     806ns ± 1%    +55.73%  (p=0.000 n=10+10)
MapPutGrow/String/18-16              1.18µs ± 0%    1.71µs ± 1%    +44.34%  (p=0.000 n=6+10)
MapPutGrow/String/24-16              1.41µs ± 1%    1.81µs ± 1%    +28.31%  (p=0.000 n=10+9)
MapPutGrow/String/30-16              2.57µs ± 1%    3.45µs ± 1%    +34.37%  (p=0.000 n=10+10)
MapPutGrow/String/64-16              5.80µs ± 1%    7.02µs ± 1%    +21.00%  (p=0.000 n=10+10)
MapPutGrow/String/128-16             11.5µs ± 1%    13.6µs ± 1%    +17.73%  (p=0.000 n=10+10)
MapPutGrow/String/256-16             22.9µs ± 1%    25.7µs ± 0%    +12.21%  (p=0.000 n=10+10)
MapPutGrow/String/512-16             46.1µs ± 1%    49.3µs ± 1%     +7.02%  (p=0.000 n=10+10)
MapPutGrow/String/1024-16            92.1µs ± 1%    96.9µs ± 1%     +5.16%  (p=0.000 n=10+10)
MapPutGrow/String/2048-16             185µs ± 1%     191µs ± 1%     +3.02%  (p=0.000 n=10+10)
MapPutGrow/String/4096-16             392µs ± 2%     345µs ± 1%    -12.09%  (p=0.000 n=10+10)
MapPutGrow/String/8192-16             923µs ± 1%     699µs ± 1%    -24.23%  (p=0.000 n=10+10)
MapPutGrow/String/65536-16           9.55ms ± 3%    7.26ms ± 3%    -23.97%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/6-16         85.4ns ± 0%   211.2ns ± 0%   +147.30%  (p=0.000 n=10+9)
MapPutPreAllocate/Int64/12-16         344ns ± 1%     312ns ± 1%     -9.07%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/18-16         553ns ± 0%     445ns ± 1%    -19.51%  (p=0.000 n=10+9)
MapPutPreAllocate/Int64/24-16         785ns ± 1%     534ns ± 0%    -31.92%  (p=0.000 n=10+8)
MapPutPreAllocate/Int64/30-16         950ns ± 1%     687ns ± 1%    -27.62%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/64-16        2.19µs ± 0%    1.34µs ± 0%    -39.02%  (p=0.000 n=10+9)
MapPutPreAllocate/Int64/128-16       4.21µs ± 0%    2.54µs ± 1%    -39.62%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/256-16       8.02µs ± 1%    4.79µs ± 1%    -40.26%  (p=0.000 n=9+10)
MapPutPreAllocate/Int64/512-16       16.0µs ± 1%     9.0µs ± 0%    -43.64%  (p=0.000 n=10+7)
MapPutPreAllocate/Int64/1024-16      31.9µs ± 1%    17.3µs ± 1%    -45.78%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/2048-16      64.0µs ± 1%    34.4µs ± 1%    -46.32%  (p=0.000 n=9+10)
MapPutPreAllocate/Int64/4096-16       131µs ± 0%      83µs ± 1%    -36.42%  (p=0.000 n=8+10)
MapPutPreAllocate/Int64/8192-16       272µs ± 0%     171µs ± 1%    -37.08%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/65536-16     2.57ms ± 1%    1.91ms ± 2%    -25.73%  (p=0.000 n=8+9)
MapPutPreAllocate/Int32/6-16         86.8ns ± 0%   207.1ns ± 1%   +138.61%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/12-16         313ns ± 0%     292ns ± 1%     -6.86%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/18-16         501ns ± 0%     392ns ± 1%    -21.84%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/24-16         720ns ± 0%     476ns ± 1%    -33.94%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/30-16         850ns ± 1%     588ns ± 1%    -30.83%  (p=0.000 n=9+10)
MapPutPreAllocate/Int32/64-16        1.94µs ± 1%    1.09µs ± 1%    -43.67%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/128-16       3.80µs ± 1%    2.14µs ± 1%    -43.62%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/256-16       7.54µs ± 0%    4.16µs ± 1%    -44.83%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/512-16       14.5µs ± 1%     8.0µs ± 1%    -44.68%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/1024-16      28.6µs ± 0%    15.2µs ± 1%    -46.69%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/2048-16      58.0µs ± 1%    30.0µs ± 1%    -48.34%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/4096-16       118µs ± 0%      74µs ± 2%    -37.34%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/8192-16       242µs ± 1%     149µs ± 1%    -38.30%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/65536-16     2.48ms ± 2%    1.52ms ± 1%    -38.79%  (p=0.000 n=10+9)
MapPutPreAllocate/String/6-16        92.4ns ± 0%   280.2ns ± 1%   +203.18%  (p=0.000 n=9+10)
MapPutPreAllocate/String/12-16        412ns ± 0%     434ns ± 1%     +5.40%  (p=0.000 n=10+10)
MapPutPreAllocate/String/18-16        697ns ± 0%     601ns ± 1%    -13.69%  (p=0.000 n=9+10)
MapPutPreAllocate/String/24-16        943ns ± 1%     715ns ± 1%    -24.12%  (p=0.000 n=10+10)
MapPutPreAllocate/String/30-16       1.23µs ± 1%    1.02µs ± 1%    -17.16%  (p=0.000 n=9+9)
MapPutPreAllocate/String/64-16       2.68µs ± 1%    1.99µs ± 2%    -25.91%  (p=0.000 n=10+10)
MapPutPreAllocate/String/128-16      5.14µs ± 1%    3.75µs ± 1%    -27.07%  (p=0.000 n=10+10)
MapPutPreAllocate/String/256-16      10.3µs ± 1%     7.0µs ± 2%    -32.16%  (p=0.000 n=10+10)
MapPutPreAllocate/String/512-16      20.7µs ± 1%    14.4µs ± 3%    -30.73%  (p=0.000 n=10+9)
MapPutPreAllocate/String/1024-16     42.1µs ± 1%    28.1µs ± 2%    -33.21%  (p=0.000 n=10+10)
MapPutPreAllocate/String/2048-16     84.3µs ± 1%    57.0µs ± 1%    -32.36%  (p=0.000 n=10+10)
MapPutPreAllocate/String/4096-16      177µs ± 1%     137µs ± 2%    -22.88%  (p=0.000 n=10+10)
MapPutPreAllocate/String/8192-16      378µs ± 1%     288µs ± 2%    -23.77%  (p=0.000 n=10+10)
MapPutPreAllocate/String/65536-16    3.99ms ± 3%    5.16ms ± 1%    +29.26%  (p=0.000 n=10+9)
MapPutReuse/Int64/6-16               96.4ns ± 1%   189.4ns ± 0%    +96.35%  (p=0.000 n=10+9)
MapPutReuse/Int64/12-16               283ns ± 0%     271ns ±74%       ~     (p=0.138 n=10+10)
MapPutReuse/Int64/18-16               449ns ± 1%     380ns ±50%       ~     (p=0.481 n=10+10)
MapPutReuse/Int64/24-16               678ns ± 1%     492ns ±47%       ~     (p=0.482 n=9+10)
MapPutReuse/Int64/30-16               775ns ± 0%     420ns ± 0%    -45.79%  (p=0.000 n=10+10)
MapPutReuse/Int64/64-16              1.71µs ± 1%    0.93µs ±16%    -45.29%  (p=0.000 n=10+10)
MapPutReuse/Int64/128-16             3.44µs ± 1%    1.90µs ± 8%    -44.84%  (p=0.000 n=10+9)
MapPutReuse/Int64/256-16             6.90µs ± 1%    3.52µs ± 7%    -48.96%  (p=0.000 n=10+10)
MapPutReuse/Int64/512-16             13.7µs ± 0%     7.1µs ± 5%    -48.08%  (p=0.000 n=10+10)
MapPutReuse/Int64/1024-16            27.4µs ± 0%    14.4µs ± 3%    -47.65%  (p=0.000 n=10+10)
MapPutReuse/Int64/2048-16            56.9µs ± 0%    29.8µs ± 6%    -47.54%  (p=0.000 n=10+10)
MapPutReuse/Int64/4096-16             117µs ± 1%      73µs ± 1%    -37.43%  (p=0.000 n=10+8)
MapPutReuse/Int64/8192-16             240µs ± 1%     165µs ±13%    -31.39%  (p=0.000 n=10+10)
MapPutReuse/Int64/65536-16           2.39ms ± 1%    1.34ms ± 2%    -44.05%  (p=0.000 n=10+10)
MapPutReuse/Int32/6-16               98.3ns ± 0%   186.6ns ± 0%    +89.77%  (p=0.000 n=10+10)
MapPutReuse/Int32/12-16               281ns ± 0%     187ns ± 1%    -33.44%  (p=0.000 n=10+10)
MapPutReuse/Int32/18-16               456ns ± 1%     257ns ± 0%    -43.55%  (p=0.000 n=10+8)
MapPutReuse/Int32/24-16               680ns ± 1%     340ns ± 1%    -50.07%  (p=0.000 n=10+10)
MapPutReuse/Int32/30-16               791ns ± 1%     423ns ± 1%    -46.52%  (p=0.000 n=10+10)
MapPutReuse/Int32/64-16              1.72µs ± 1%    0.88µs ± 0%    -49.09%  (p=0.000 n=10+10)
MapPutReuse/Int32/128-16             3.52µs ± 0%    1.77µs ± 1%    -49.64%  (p=0.000 n=10+9)
MapPutReuse/Int32/256-16             7.06µs ± 1%    3.45µs ± 0%    -51.13%  (p=0.000 n=9+9)
MapPutReuse/Int32/512-16             14.1µs ± 0%     7.0µs ± 4%    -50.52%  (p=0.000 n=10+10)
MapPutReuse/Int32/1024-16            28.0µs ± 0%    14.0µs ± 1%    -49.91%  (p=0.000 n=10+8)
MapPutReuse/Int32/2048-16            55.2µs ± 0%    28.8µs ± 5%    -47.78%  (p=0.000 n=10+10)
MapPutReuse/Int32/4096-16             115µs ± 0%      72µs ± 1%    -37.45%  (p=0.000 n=10+7)
MapPutReuse/Int32/8192-16             237µs ± 1%     143µs ± 3%    -39.51%  (p=0.000 n=10+9)
MapPutReuse/Int32/65536-16           2.14ms ± 1%    1.26ms ± 1%    -41.24%  (p=0.000 n=9+10)
MapPutReuse/String/6-16               108ns ± 0%     226ns ± 0%   +110.41%  (p=0.000 n=9+9)
MapPutReuse/String/12-16              315ns ± 0%     220ns ± 1%    -30.07%  (p=0.000 n=10+9)
MapPutReuse/String/18-16              519ns ± 1%     328ns ±11%    -36.71%  (p=0.000 n=10+10)
MapPutReuse/String/24-16              771ns ± 0%     416ns ± 1%    -46.01%  (p=0.000 n=9+8)
MapPutReuse/String/30-16              898ns ± 1%     504ns ± 2%    -43.87%  (p=0.000 n=10+10)
MapPutReuse/String/64-16             1.93µs ± 1%    1.02µs ± 0%    -46.93%  (p=0.000 n=10+10)
MapPutReuse/String/128-16            3.90µs ± 0%    2.07µs ± 0%    -46.89%  (p=0.000 n=10+8)
MapPutReuse/String/256-16            7.80µs ± 1%    4.13µs ± 1%    -47.02%  (p=0.000 n=10+7)
MapPutReuse/String/512-16            15.5µs ± 1%     8.6µs ± 4%    -44.88%  (p=0.000 n=10+10)
MapPutReuse/String/1024-16           32.6µs ± 1%    18.7µs ± 1%    -42.72%  (p=0.000 n=10+10)
MapPutReuse/String/2048-16           66.9µs ± 1%    37.7µs ± 1%    -43.75%  (p=0.000 n=10+10)
MapPutReuse/String/4096-16            136µs ± 2%      90µs ± 3%    -34.14%  (p=0.000 n=10+10)
MapPutReuse/String/8192-16            282µs ± 1%     182µs ± 1%    -35.37%  (p=0.000 n=10+9)
MapPutReuse/String/65536-16          3.12ms ± 2%    1.91ms ± 3%    -38.89%  (p=0.000 n=9+9)
MapPutDelete/Int64/6-16              27.0ns ± 1%    38.1ns ± 2%    +40.78%  (p=0.000 n=10+10)
MapPutDelete/Int64/12-16             29.0ns ± 6%    28.1ns ± 6%       ~     (p=0.053 n=9+9)
MapPutDelete/Int64/18-16             28.2ns ± 5%    27.1ns ± 5%     -4.09%  (p=0.015 n=9+8)
MapPutDelete/Int64/24-16             30.9ns ± 7%    27.3ns ± 6%    -11.59%  (p=0.000 n=10+10)
MapPutDelete/Int64/30-16             27.1ns ± 7%    27.5ns ± 4%       ~     (p=0.165 n=10+10)
MapPutDelete/Int64/64-16             27.8ns ± 5%    27.9ns ± 5%       ~     (p=0.529 n=10+10)
MapPutDelete/Int64/128-16            28.3ns ± 4%    28.1ns ± 4%       ~     (p=0.645 n=10+10)
MapPutDelete/Int64/256-16            28.3ns ± 5%    28.8ns ± 3%       ~     (p=0.515 n=10+8)
MapPutDelete/Int64/512-16            28.2ns ± 2%    29.0ns ± 2%     +2.76%  (p=0.000 n=10+10)
MapPutDelete/Int64/1024-16           34.5ns ± 2%    30.1ns ± 3%    -12.81%  (p=0.000 n=10+10)
MapPutDelete/Int64/2048-16           47.7ns ± 1%    30.4ns ± 6%    -36.26%  (p=0.000 n=10+10)
MapPutDelete/Int64/4096-16           53.4ns ± 1%    36.5ns ± 5%    -31.77%  (p=0.000 n=9+10)
MapPutDelete/Int64/8192-16           56.3ns ± 1%    38.1ns ± 3%    -32.33%  (p=0.000 n=10+10)
MapPutDelete/Int64/65536-16          70.3ns ± 2%    59.8ns ± 3%    -14.97%  (p=0.000 n=10+9)
MapPutDelete/Int32/6-16              27.2ns ± 0%    32.1ns ± 3%    +17.93%  (p=0.000 n=9+10)
MapPutDelete/Int32/12-16             27.9ns ± 3%    28.6ns ± 3%     +2.71%  (p=0.012 n=9+10)
MapPutDelete/Int32/18-16             27.3ns ± 8%    27.9ns ± 6%       ~     (p=0.143 n=10+10)
MapPutDelete/Int32/24-16             29.9ns ±10%    27.8ns ± 4%     -6.85%  (p=0.000 n=10+10)
MapPutDelete/Int32/30-16             26.9ns ±14%    27.7ns ± 3%       ~     (p=0.247 n=10+10)
MapPutDelete/Int32/64-16             26.6ns ± 4%    27.8ns ± 6%     +4.17%  (p=0.001 n=10+10)
MapPutDelete/Int32/128-16            26.9ns ± 2%    28.8ns ± 3%     +7.09%  (p=0.000 n=10+10)
MapPutDelete/Int32/256-16            26.7ns ± 2%    28.7ns ± 2%     +7.75%  (p=0.000 n=10+9)
MapPutDelete/Int32/512-16            27.4ns ± 3%    29.6ns ± 4%     +8.28%  (p=0.000 n=10+10)
MapPutDelete/Int32/1024-16           32.7ns ± 3%    30.1ns ± 1%     -7.96%  (p=0.000 n=10+7)
MapPutDelete/Int32/2048-16           44.8ns ± 2%    31.0ns ± 6%    -30.79%  (p=0.000 n=10+10)
MapPutDelete/Int32/4096-16           51.4ns ± 1%    36.3ns ± 1%    -29.52%  (p=0.000 n=10+8)
MapPutDelete/Int32/8192-16           54.3ns ± 1%    37.2ns ± 1%    -31.48%  (p=0.000 n=9+10)
MapPutDelete/Int32/65536-16          60.5ns ± 1%    43.0ns ± 1%    -28.88%  (p=0.000 n=10+10)
MapPutDelete/String/6-16             32.5ns ± 1%    40.5ns ± 4%    +24.53%  (p=0.000 n=10+10)
MapPutDelete/String/12-16            33.6ns ± 1%    43.8ns ± 8%    +30.59%  (p=0.000 n=8+10)
MapPutDelete/String/18-16            32.2ns ± 5%    45.5ns ± 5%    +41.31%  (p=0.000 n=9+10)
MapPutDelete/String/24-16            34.9ns ±10%    45.2ns ± 6%    +29.43%  (p=0.000 n=10+10)
MapPutDelete/String/30-16            31.8ns ± 5%    45.6ns ± 2%    +43.49%  (p=0.000 n=9+10)
MapPutDelete/String/64-16            32.3ns ± 2%    45.9ns ± 6%    +42.14%  (p=0.000 n=10+10)
MapPutDelete/String/128-16           32.4ns ± 2%    47.0ns ± 1%    +45.22%  (p=0.000 n=10+9)
MapPutDelete/String/256-16           33.1ns ± 2%    47.8ns ± 3%    +44.29%  (p=0.000 n=10+10)
MapPutDelete/String/512-16           34.0ns ± 4%    47.7ns ± 2%    +40.27%  (p=0.000 n=10+10)
MapPutDelete/String/1024-16          49.0ns ± 2%    49.3ns ± 1%       ~     (p=0.447 n=10+10)
MapPutDelete/String/2048-16          64.3ns ± 2%    49.9ns ± 1%    -22.27%  (p=0.000 n=10+9)
MapPutDelete/String/4096-16          70.5ns ± 1%    56.3ns ± 1%    -20.14%  (p=0.000 n=10+10)
MapPutDelete/String/8192-16          72.3ns ± 1%    57.1ns ± 8%    -21.05%  (p=0.000 n=9+10)
MapPutDelete/String/65536-16          119ns ± 3%     154ns ± 1%    +29.42%  (p=0.000 n=10+9)
```
</details>

On top of the base Swiss Tables design, `swiss.Map` adds an [extendible
hashing](https://en.wikipedia.org/wiki/Extendible_hashing) layer in order to
enable incremental resizing of large maps which significantly reduces tail
latency for `Put` operations in maps with hundreds of thousands of entries or
more.

`swiss.Map` provides pseudo-randomized iteration (iteration order will change
from one iteration to the next) and iteration stability akin to Go's builtin
map if the map is mutated during iteration.

## Caveats

- The implementation currently requires a little endian CPU architecture. This
  is not a fundamental limitation of the implementation, merely a choice of
  expediency.
- Go's builtin map has a fast-path for comparing strings that [share their
  underlying
  storage](https://github.com/golang/go/blob/4a7f3ac8eb4381ea62caa1741eeeec28363245b4/src/runtime/map_faststr.go#L100).
  This fast-path is feasible because `map[string]T` is specialized for string
  keys which isn't currently possible with Go's generics.
- Go's builtin map has a fast-path for maps with int32, int64, and string keys
  that fit in a single bucket (8 entries) which avoids performing `hash(key)`
  and simply linearly searches through the bucket. Similar to the above, this
  fast-path is feasible because the runtime can specialize the implementation
  on the key type.

## TODO

- Add support for SIMD searching on x86 and [8-byte Neon SIMD searching on
  arm64](https://github.com/abseil/abseil-cpp/commit/6481443560a92d0a3a55a31807de0cd712cd4f88)
  - This appears to be somewhat difficult. Naively implementing the match
    routines in assembly isn't workable as the function call overhead
    dominates the performance improvement from the SIMD comparisons. The
    Abseil implementation is leveraring gcc/llvm assembly intrinsics which are
    not currently available in Go. In order to take advantage of SIMD we'll
    have to write most/all of the probing loop in assembly.
