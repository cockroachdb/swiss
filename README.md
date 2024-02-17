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
MapPutGrow/Int64/6-16                83.5ns ± 0%   143.8ns ± 1%    +72.15%  (p=0.000 n=10+10)
MapPutGrow/Int64/12-16                422ns ± 1%     373ns ± 1%    -11.61%  (p=0.000 n=10+10)
MapPutGrow/Int64/18-16                913ns ± 1%     754ns ± 1%    -17.39%  (p=0.000 n=10+10)
MapPutGrow/Int64/24-16               1.14µs ± 1%    0.83µs ± 2%    -27.58%  (p=0.000 n=9+10)
MapPutGrow/Int64/30-16               2.02µs ± 1%    1.52µs ± 2%    -24.81%  (p=0.000 n=10+10)
MapPutGrow/Int64/64-16               4.64µs ± 1%    2.93µs ± 2%    -36.83%  (p=0.000 n=10+10)
MapPutGrow/Int64/128-16              9.42µs ± 1%    5.73µs ± 2%    -39.20%  (p=0.000 n=10+10)
MapPutGrow/Int64/256-16              18.4µs ± 1%    11.1µs ± 2%    -39.62%  (p=0.000 n=10+10)
MapPutGrow/Int64/512-16              35.8µs ± 1%    21.6µs ± 2%    -39.78%  (p=0.000 n=10+10)
MapPutGrow/Int64/1024-16             71.0µs ± 1%    42.3µs ± 3%    -40.42%  (p=0.000 n=10+10)
MapPutGrow/Int64/2048-16              141µs ± 1%      82µs ± 2%    -41.60%  (p=0.000 n=10+10)
MapPutGrow/Int64/4096-16              282µs ± 1%     184µs ± 1%    -34.69%  (p=0.000 n=10+10)
MapPutGrow/Int64/8192-16              571µs ± 0%     436µs ± 2%    -23.54%  (p=0.000 n=10+10)
MapPutGrow/Int64/65536-16            5.62ms ± 2%    3.96ms ± 2%    -29.51%  (p=0.000 n=10+10)
MapPutGrow/Int32/6-16                83.1ns ± 0%   140.4ns ± 2%    +68.95%  (p=0.000 n=10+10)
MapPutGrow/Int32/12-16                390ns ± 0%     330ns ± 2%    -15.39%  (p=0.000 n=10+10)
MapPutGrow/Int32/18-16                836ns ± 0%     616ns ± 1%    -26.28%  (p=0.000 n=9+10)
MapPutGrow/Int32/24-16               1.06µs ± 1%    0.70µs ± 2%    -33.75%  (p=0.000 n=10+10)
MapPutGrow/Int32/30-16               1.83µs ± 0%    1.23µs ± 2%    -32.95%  (p=0.000 n=10+10)
MapPutGrow/Int32/64-16               4.16µs ± 1%    2.51µs ± 2%    -39.63%  (p=0.000 n=10+10)
MapPutGrow/Int32/128-16              8.46µs ± 1%    5.03µs ± 2%    -40.53%  (p=0.000 n=10+10)
MapPutGrow/Int32/256-16              16.8µs ± 0%    10.0µs ± 3%    -40.30%  (p=0.000 n=9+10)
MapPutGrow/Int32/512-16              32.8µs ± 0%    19.9µs ± 3%    -39.32%  (p=0.000 n=10+10)
MapPutGrow/Int32/1024-16             64.3µs ± 0%    39.1µs ± 2%    -39.30%  (p=0.000 n=9+10)
MapPutGrow/Int32/2048-16              128µs ± 0%      78µs ± 2%    -39.40%  (p=0.000 n=10+10)
MapPutGrow/Int32/4096-16              252µs ± 0%     178µs ± 2%    -29.59%  (p=0.000 n=10+10)
MapPutGrow/Int32/8192-16              506µs ± 1%     424µs ± 2%    -16.19%  (p=0.000 n=10+10)
MapPutGrow/Int32/65536-16            4.57ms ± 3%    3.82ms ± 2%    -16.27%  (p=0.000 n=10+10)
MapPutGrow/String/6-16               90.0ns ± 0%   187.7ns ± 2%   +108.57%  (p=0.000 n=10+10)
MapPutGrow/String/12-16               518ns ± 2%     513ns ± 2%       ~     (p=0.066 n=10+10)
MapPutGrow/String/18-16              1.18µs ± 0%    1.06µs ± 2%    -10.44%  (p=0.000 n=6+10)
MapPutGrow/String/24-16              1.41µs ± 1%    1.17µs ± 2%    -16.95%  (p=0.000 n=10+10)
MapPutGrow/String/30-16              2.57µs ± 1%    2.17µs ± 2%    -15.36%  (p=0.000 n=10+10)
MapPutGrow/String/64-16              5.80µs ± 1%    4.45µs ± 2%    -23.29%  (p=0.000 n=10+10)
MapPutGrow/String/128-16             11.5µs ± 1%     8.6µs ± 3%    -25.27%  (p=0.000 n=10+10)
MapPutGrow/String/256-16             22.9µs ± 1%    16.3µs ± 1%    -28.71%  (p=0.000 n=10+10)
MapPutGrow/String/512-16             46.1µs ± 1%    31.4µs ± 2%    -31.91%  (p=0.000 n=10+10)
MapPutGrow/String/1024-16            92.1µs ± 1%    62.5µs ± 2%    -32.18%  (p=0.000 n=10+10)
MapPutGrow/String/2048-16             185µs ± 1%     122µs ± 2%    -34.18%  (p=0.000 n=10+10)
MapPutGrow/String/4096-16             392µs ± 2%     266µs ± 2%    -32.21%  (p=0.000 n=10+10)
MapPutGrow/String/8192-16             923µs ± 1%     612µs ± 1%    -33.68%  (p=0.000 n=10+8)
MapPutGrow/String/65536-16           9.55ms ± 3%    6.64ms ± 5%    -30.48%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/6-16         85.4ns ± 0%   135.0ns ± 2%    +58.12%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/12-16         344ns ± 1%     217ns ± 1%    -36.85%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/18-16         553ns ± 0%     310ns ± 1%    -43.93%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/24-16         785ns ± 1%     389ns ± 2%    -50.46%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/30-16         950ns ± 1%     535ns ± 3%    -43.69%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/64-16        2.19µs ± 0%    1.05µs ± 2%    -52.14%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/128-16       4.21µs ± 0%    2.03µs ± 2%    -51.68%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/256-16       8.02µs ± 1%    3.91µs ± 3%    -51.22%  (p=0.000 n=9+10)
MapPutPreAllocate/Int64/512-16       16.0µs ± 1%     7.3µs ± 2%    -54.08%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/1024-16      31.9µs ± 1%    14.1µs ± 1%    -55.87%  (p=0.000 n=10+10)
MapPutPreAllocate/Int64/2048-16      64.0µs ± 1%    27.5µs ± 1%    -57.09%  (p=0.000 n=9+10)
MapPutPreAllocate/Int64/4096-16       131µs ± 0%      65µs ± 1%    -50.64%  (p=0.000 n=8+10)
MapPutPreAllocate/Int64/8192-16       272µs ± 0%     136µs ± 1%    -50.06%  (p=0.000 n=10+9)
MapPutPreAllocate/Int64/65536-16     2.57ms ± 1%    1.49ms ± 4%    -42.08%  (p=0.000 n=8+10)
MapPutPreAllocate/Int32/6-16         86.8ns ± 0%   134.7ns ± 2%    +55.15%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/12-16         313ns ± 0%     210ns ± 2%    -33.12%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/18-16         501ns ± 0%     288ns ± 2%    -42.63%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/24-16         720ns ± 0%     367ns ± 2%    -49.01%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/30-16         850ns ± 1%     481ns ± 2%    -43.42%  (p=0.000 n=9+10)
MapPutPreAllocate/Int32/64-16        1.94µs ± 1%    0.92µs ± 2%    -52.84%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/128-16       3.80µs ± 1%    1.78µs ± 2%    -53.22%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/256-16       7.54µs ± 0%    3.50µs ± 3%    -53.60%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/512-16       14.5µs ± 1%     6.9µs ± 1%    -52.57%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/1024-16      28.6µs ± 0%    13.2µs ± 1%    -53.69%  (p=0.000 n=10+9)
MapPutPreAllocate/Int32/2048-16      58.0µs ± 1%    25.7µs ± 2%    -55.71%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/4096-16       118µs ± 0%      61µs ± 1%    -48.36%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/8192-16       242µs ± 1%     126µs ± 2%    -47.80%  (p=0.000 n=10+10)
MapPutPreAllocate/Int32/65536-16     2.48ms ± 2%    1.20ms ± 6%    -51.51%  (p=0.000 n=10+10)
MapPutPreAllocate/String/6-16        92.4ns ± 0%   179.5ns ± 2%    +94.25%  (p=0.000 n=9+10)
MapPutPreAllocate/String/12-16        412ns ± 0%     298ns ± 2%    -27.54%  (p=0.000 n=10+10)
MapPutPreAllocate/String/18-16        697ns ± 0%     474ns ± 2%    -32.03%  (p=0.000 n=9+10)
MapPutPreAllocate/String/24-16        943ns ± 1%     568ns ± 2%    -39.80%  (p=0.000 n=10+10)
MapPutPreAllocate/String/30-16       1.23µs ± 1%    0.81µs ± 2%    -33.98%  (p=0.000 n=9+10)
MapPutPreAllocate/String/64-16       2.68µs ± 1%    1.61µs ± 2%    -39.90%  (p=0.000 n=10+10)
MapPutPreAllocate/String/128-16      5.14µs ± 1%    3.12µs ± 1%    -39.33%  (p=0.000 n=10+10)
MapPutPreAllocate/String/256-16      10.3µs ± 1%     5.8µs ± 2%    -43.45%  (p=0.000 n=10+10)
MapPutPreAllocate/String/512-16      20.7µs ± 1%    11.0µs ± 2%    -46.91%  (p=0.000 n=10+10)
MapPutPreAllocate/String/1024-16     42.1µs ± 1%    22.4µs ± 1%    -46.84%  (p=0.000 n=10+10)
MapPutPreAllocate/String/2048-16     84.3µs ± 1%    44.8µs ± 3%    -46.86%  (p=0.000 n=10+10)
MapPutPreAllocate/String/4096-16      177µs ± 1%      99µs ± 2%    -44.37%  (p=0.000 n=10+10)
MapPutPreAllocate/String/8192-16      378µs ± 1%     227µs ± 6%    -40.02%  (p=0.000 n=10+10)
MapPutPreAllocate/String/65536-16    3.99ms ± 3%    3.42ms ±16%    -14.35%  (p=0.000 n=10+10)
MapPutReuse/Int64/6-16               96.4ns ± 1%    81.1ns ± 0%    -15.87%  (p=0.000 n=10+8)
MapPutReuse/Int64/12-16               283ns ± 0%     196ns ± 0%    -30.72%  (p=0.000 n=10+8)
MapPutReuse/Int64/18-16               449ns ± 1%     234ns ±34%    -47.95%  (p=0.000 n=10+10)
MapPutReuse/Int64/24-16               678ns ± 1%     336ns ±13%    -50.42%  (p=0.000 n=9+8)
MapPutReuse/Int64/30-16               775ns ± 0%     354ns ± 1%    -54.29%  (p=0.000 n=10+9)
MapPutReuse/Int64/64-16              1.71µs ± 1%    0.73µs ± 4%    -57.15%  (p=0.000 n=10+9)
MapPutReuse/Int64/128-16             3.44µs ± 1%    1.48µs ± 2%    -57.17%  (p=0.000 n=10+8)
MapPutReuse/Int64/256-16             6.90µs ± 1%    2.86µs ± 2%    -58.56%  (p=0.000 n=10+8)
MapPutReuse/Int64/512-16             13.7µs ± 0%     5.7µs ± 3%    -58.74%  (p=0.000 n=10+10)
MapPutReuse/Int64/1024-16            27.4µs ± 0%    11.4µs ± 1%    -58.53%  (p=0.000 n=10+8)
MapPutReuse/Int64/2048-16            56.9µs ± 0%    23.9µs ± 1%    -58.01%  (p=0.000 n=10+10)
MapPutReuse/Int64/4096-16             117µs ± 1%      61µs ± 3%    -48.15%  (p=0.000 n=10+10)
MapPutReuse/Int64/8192-16             240µs ± 1%     121µs ± 1%    -49.48%  (p=0.000 n=10+10)
MapPutReuse/Int64/65536-16           2.39ms ± 1%    1.13ms ± 1%    -52.53%  (p=0.000 n=10+10)
MapPutReuse/Int32/6-16               98.3ns ± 0%    81.2ns ± 0%    -17.48%  (p=0.000 n=10+10)
MapPutReuse/Int32/12-16               281ns ± 0%     195ns ± 0%    -30.78%  (p=0.000 n=10+8)
MapPutReuse/Int32/18-16               456ns ± 1%     209ns ± 1%    -54.07%  (p=0.000 n=10+10)
MapPutReuse/Int32/24-16               680ns ± 1%     289ns ± 1%    -57.55%  (p=0.000 n=10+10)
MapPutReuse/Int32/30-16               791ns ± 1%     355ns ± 0%    -55.09%  (p=0.000 n=10+9)
MapPutReuse/Int32/64-16              1.72µs ± 1%    0.72µs ± 1%    -58.03%  (p=0.000 n=10+9)
MapPutReuse/Int32/128-16             3.52µs ± 0%    1.54µs ±16%    -56.31%  (p=0.000 n=10+8)
MapPutReuse/Int32/256-16             7.06µs ± 1%    2.81µs ± 0%    -60.18%  (p=0.000 n=9+8)
MapPutReuse/Int32/512-16             14.1µs ± 0%     5.6µs ± 1%    -60.14%  (p=0.000 n=10+9)
MapPutReuse/Int32/1024-16            28.0µs ± 0%    11.4µs ± 2%    -59.32%  (p=0.000 n=10+8)
MapPutReuse/Int32/2048-16            55.2µs ± 0%    23.0µs ± 1%    -58.40%  (p=0.000 n=10+10)
MapPutReuse/Int32/4096-16             115µs ± 0%      59µs ± 1%    -48.79%  (p=0.000 n=10+10)
MapPutReuse/Int32/8192-16             237µs ± 1%     120µs ± 2%    -49.26%  (p=0.000 n=10+10)
MapPutReuse/Int32/65536-16           2.14ms ± 1%    1.02ms ± 1%    -52.16%  (p=0.000 n=9+10)
MapPutReuse/String/6-16               108ns ± 0%     105ns ± 0%     -1.88%  (p=0.000 n=9+9)
MapPutReuse/String/12-16              315ns ± 0%     232ns ±25%    -26.45%  (p=0.000 n=10+10)
MapPutReuse/String/18-16              519ns ± 1%     301ns ±13%    -42.04%  (p=0.000 n=10+10)
MapPutReuse/String/24-16              771ns ± 0%     378ns ±14%    -51.01%  (p=0.000 n=9+9)
MapPutReuse/String/30-16              898ns ± 1%     460ns ± 1%    -48.80%  (p=0.000 n=10+9)
MapPutReuse/String/64-16             1.93µs ± 1%    0.95µs ± 2%    -50.83%  (p=0.000 n=10+9)
MapPutReuse/String/128-16            3.90µs ± 0%    1.94µs ± 5%    -50.36%  (p=0.000 n=10+9)
MapPutReuse/String/256-16            7.80µs ± 1%    3.99µs ± 5%    -48.87%  (p=0.000 n=10+10)
MapPutReuse/String/512-16            15.5µs ± 1%     7.7µs ± 0%    -50.51%  (p=0.000 n=10+7)
MapPutReuse/String/1024-16           32.6µs ± 1%    17.5µs ± 7%    -46.17%  (p=0.000 n=10+10)
MapPutReuse/String/2048-16           66.9µs ± 1%    33.9µs ± 1%    -49.35%  (p=0.000 n=10+10)
MapPutReuse/String/4096-16            136µs ± 2%      82µs ± 1%    -39.56%  (p=0.000 n=10+8)
MapPutReuse/String/8192-16            282µs ± 1%     163µs ± 1%    -42.19%  (p=0.000 n=10+10)
MapPutReuse/String/65536-16          3.12ms ± 2%    1.83ms ± 2%    -41.26%  (p=0.000 n=9+9)
MapPutDelete/Int64/6-16              27.0ns ± 1%    24.7ns ± 6%     -8.63%  (p=0.000 n=10+10)
MapPutDelete/Int64/12-16             29.0ns ± 6%    28.0ns ± 8%     -3.76%  (p=0.023 n=9+10)
MapPutDelete/Int64/18-16             28.2ns ± 5%    26.2ns ±10%     -7.24%  (p=0.011 n=9+10)
MapPutDelete/Int64/24-16             30.9ns ± 7%    27.9ns ± 6%     -9.81%  (p=0.000 n=10+9)
MapPutDelete/Int64/30-16             27.1ns ± 7%    26.6ns ±15%       ~     (p=0.393 n=10+10)
MapPutDelete/Int64/64-16             27.8ns ± 5%    29.3ns ±17%       ~     (p=0.353 n=10+10)
MapPutDelete/Int64/128-16            28.3ns ± 4%    32.6ns ±34%       ~     (p=0.190 n=10+10)
MapPutDelete/Int64/256-16            28.3ns ± 5%    32.0ns ±11%    +13.17%  (p=0.000 n=10+10)
MapPutDelete/Int64/512-16            28.2ns ± 2%    32.2ns ±14%    +14.01%  (p=0.000 n=10+10)
MapPutDelete/Int64/1024-16           34.5ns ± 2%    36.4ns ± 9%     +5.53%  (p=0.001 n=10+10)
MapPutDelete/Int64/2048-16           47.7ns ± 1%    39.3ns ± 9%    -17.66%  (p=0.000 n=10+10)
MapPutDelete/Int64/4096-16           53.4ns ± 1%    49.6ns ± 5%     -7.26%  (p=0.000 n=9+10)
MapPutDelete/Int64/8192-16           56.3ns ± 1%    52.7ns ± 4%     -6.48%  (p=0.000 n=10+8)
MapPutDelete/Int64/65536-16          70.3ns ± 2%    66.1ns ± 5%     -5.99%  (p=0.000 n=10+10)
MapPutDelete/Int32/6-16              27.2ns ± 0%    24.0ns ± 0%    -11.87%  (p=0.000 n=9+9)
MapPutDelete/Int32/12-16             27.9ns ± 3%    28.1ns ± 2%       ~     (p=0.340 n=9+9)
MapPutDelete/Int32/18-16             27.3ns ± 8%    26.4ns ± 9%       ~     (p=0.280 n=10+10)
MapPutDelete/Int32/24-16             29.9ns ±10%    27.7ns ± 5%     -7.48%  (p=0.000 n=10+9)
MapPutDelete/Int32/30-16             26.9ns ±14%    27.1ns ±13%       ~     (p=0.929 n=10+10)
MapPutDelete/Int32/64-16             26.6ns ± 4%    29.5ns ±18%       ~     (p=0.063 n=10+10)
MapPutDelete/Int32/128-16            26.9ns ± 2%    29.8ns ±20%       ~     (p=0.079 n=10+10)
MapPutDelete/Int32/256-16            26.7ns ± 2%    33.7ns ±12%    +26.59%  (p=0.000 n=10+10)
MapPutDelete/Int32/512-16            27.4ns ± 3%    33.0ns ±16%    +20.44%  (p=0.000 n=10+10)
MapPutDelete/Int32/1024-16           32.7ns ± 3%    37.4ns ± 9%    +14.34%  (p=0.000 n=10+10)
MapPutDelete/Int32/2048-16           44.8ns ± 2%    38.6ns ± 5%    -13.98%  (p=0.000 n=10+8)
MapPutDelete/Int32/4096-16           51.4ns ± 1%    48.0ns ± 3%     -6.77%  (p=0.000 n=10+10)
MapPutDelete/Int32/8192-16           54.3ns ± 1%    52.1ns ± 2%     -3.98%  (p=0.000 n=9+10)
MapPutDelete/Int32/65536-16          60.5ns ± 1%    58.9ns ± 1%     -2.66%  (p=0.000 n=10+10)
MapPutDelete/String/6-16             32.5ns ± 1%    26.5ns ± 0%    -18.47%  (p=0.000 n=10+9)
MapPutDelete/String/12-16            33.6ns ± 1%    36.2ns ± 9%     +7.88%  (p=0.005 n=8+9)
MapPutDelete/String/18-16            32.2ns ± 5%    36.8ns ±15%    +14.34%  (p=0.001 n=9+10)
MapPutDelete/String/24-16            34.9ns ±10%    37.2ns ± 6%     +6.54%  (p=0.004 n=10+9)
MapPutDelete/String/30-16            31.8ns ± 5%    33.6ns ±30%       ~     (p=0.780 n=9+10)
MapPutDelete/String/64-16            32.3ns ± 2%    37.5ns ±18%    +16.22%  (p=0.002 n=10+10)
MapPutDelete/String/128-16           32.4ns ± 2%    38.7ns ±16%    +19.48%  (p=0.000 n=10+10)
MapPutDelete/String/256-16           33.1ns ± 2%    42.0ns ±10%    +26.68%  (p=0.000 n=10+10)
MapPutDelete/String/512-16           34.0ns ± 4%    40.3ns ±16%    +18.32%  (p=0.000 n=10+9)
MapPutDelete/String/1024-16          49.0ns ± 2%    45.7ns ± 8%     -6.87%  (p=0.000 n=10+10)
MapPutDelete/String/2048-16          64.3ns ± 2%    49.2ns ± 7%    -23.40%  (p=0.000 n=10+10)
MapPutDelete/String/4096-16          70.5ns ± 1%    60.0ns ± 4%    -14.85%  (p=0.000 n=10+10)
MapPutDelete/String/8192-16          72.3ns ± 1%    61.5ns ± 1%    -14.92%  (p=0.000 n=9+8)
MapPutDelete/String/65536-16          119ns ± 3%     108ns ± 7%     -9.55%  (p=0.000 n=10+10)
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
