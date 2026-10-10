<div align="center">

![Build Passing](https://github.com/scylladb/gocql/workflows/Build/badge.svg)
[![Code Coverage](https://codecov.io/gh/scylladb/gocql/branch/master/graph/badge.svg)](https://codecov.io/gh/scylladb/gocql)
[![Read the Fork Driver Docs](https://img.shields.io/badge/Read_the_Docs-ScyllaDB-blue)](https://gocql-driver.docs.scylladb.com/stable/)
[![Protocol Specs](https://img.shields.io/badge/Protocol_Specs-ScyllaDB_Docs-blue)](https://github.com/scylladb/scylladb/blob/master/docs/dev/protocol-extensions.md)

</div>

<h1 align="center">

Scylla Shard-Aware Fork of [apache/cassandra-gocql-driver](https://github.com/apache/cassandra-gocql-driver)

</h1>


<img src="./.github/assets/logo.svg" width="200" align="left" />

This is a fork of [apache/cassandra-gocql-driver](https://github.com/apache/cassandra-gocql-driver) package that we created at Scylla.
It contains extensions to tokenAwareHostPolicy supported by the Scylla 2.3 and onwards.
It allows driver to select a connection to a particular shard on a host based on the token.
This eliminates passing data between shards and significantly reduces latency.

There are open pull requests to merge the functionality to the upstream project:

* [gocql/gocql#1210](https://github.com/gocql/gocql/pull/1210)
* [gocql/gocql#1211](https://github.com/gocql/gocql/pull/1211).

It also provides support for shard aware ports, a faster way to connect to all shards, details available in [blogpost](https://www.scylladb.com/2021/04/27/connect-faster-to-scylla-with-a-shard-aware-port/).

---

### Table of Contents

- [1. Sunsetting Model](#1-sunsetting-model)
- [2. Installation](#2-installation)
- [3. Quick Start](#3-quick-start)
- [4. Data Types](#4-data-types)
- [5. Configuration](#5-configuration)
  - [5.1 Advanced shard awareness (shard-aware port)](#51-advanced-shard-awareness-shard-aware-port)
  - [5.2 Client routes (PrivateLink)](#52-client-routes-privatelink)
  - [5.3 Iterator](#53-iterator)
  - [5.4 Compression](#54-compression)
- [6. Contributing](#6-contributing)

## 1. Sunsetting Model

> [!WARNING]
> In general, the gocql team will focus on supporting the current and previous versions of Go. gocql may still work with older versions of Go, but official support for these versions will have been sunset.

## 2. Installation

This is a drop-in replacement to gocql, it reuses the `github.com/gocql/gocql` import path.

Add the following line to your project `go.mod` file.

```mod
replace github.com/gocql/gocql => github.com/scylladb/gocql <version>
```

Replace `<version>` with a concrete released tag (for example `v1.20.0`) or a
pseudo-version; `latest` is not a valid version in a `replace` directive. Note
that the module path is `github.com/gocql/gocql` (no `/v2` suffix), so `v2.x`
tags are not valid replacement versions here — use a `v1` tag or a
pseudo-version.

and run

```sh
go mod tidy
```

Your project now uses the Scylla driver fork, make sure you are using the `TokenAwareHostPolicy` to enable the shard-awareness, continue reading for details.

## 3. Quick Start

Spawn a ScyllaDB Instance using Docker Run command:

```sh
docker run --name node1 --network your-network -p "9042:9042" -d scylladb/scylla:6.1.2 \
	--overprovisioned 1 \
	--smp 1
```

Then, create a new connection using ScyllaDB GoCQL following the example below:

```go
package main

import (
    "fmt"
    "github.com/gocql/gocql"
)

func main() {
    var cluster = gocql.NewCluster("localhost:9042")

    var session, err = cluster.CreateSession()
    if err != nil {
        panic("Failed to connect to cluster")
    }

    defer session.Close()

    var query = session.Query("SELECT * FROM system.clients")

    if rows, err := query.Iter().SliceMap(); err == nil {
        for _, row := range rows {
            fmt.Printf("%v\n", row)
        }
    } else {
        panic("Query error: " + err.Error())
    }
}
```

`SliceMap()` consumes and closes the iterator before it returns.

## 4. Data Types

Here's an list of all CQL Types reflected in the GoCQL environment:

| ScyllaDB Type    | Go Type            |
| ---------------- | ------------------ |
| `ascii`          | `string`           |
| `bigint`         | `int64`            |
| `blob`           | `[]byte`           |
| `boolean`        | `bool`             |
| `date`           | `time.Time`        |
| `decimal`        | `inf.Dec`          |
| `double`         | `float64`          |
| `duration`       | `gocql.Duration`   |
| `float`          | `float32`          |
| `uuid`           | `gocql.UUID`       |
| `int`            | `int32`            |
| `inet`           | `string`           |
| `list<int>`      | `[]int32`          |
| `map<int, text>` | `map[int32]string` |
| `set<int>`       | `[]int32`          |
| `smallint`       | `int16`            |
| `text`           | `string`           |
| `time`           | `time.Duration`    |
| `timestamp`      | `time.Time`        |
| `timeuuid`       | `gocql.UUID`       |
| `tinyint`        | `int8`             |
| `varchar`        | `string`           |
| `varint`         | `int64`            |

## 5. Configuration

In order to make shard-awareness work, token aware host selection policy has to be enabled.
Please make sure that the gocql configuration has `PoolConfig.HostSelectionPolicy` properly set like in the example below.

__When working with a Scylla cluster, `PoolConfig.NumConns` option has no effect - the driver opens one connection for each shard and completely ignores this option.__

```go
c := gocql.NewCluster(hosts...)

// Enable token aware host selection policy, if using multi-dc cluster set a local DC.
fallback := gocql.RoundRobinHostPolicy()
if localDC != "" {
	fallback = gocql.DCAwareRoundRobinPolicy(localDC)
}
c.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(fallback)

// If using multi-dc cluster use the "local" consistency levels.
if localDC != "" {
	c.Consistency = gocql.LocalQuorum
}

// When working with a Scylla cluster the driver always opens one connection per shard, so `NumConns` is ignored.
// c.NumConns = 4
```

### 5.1 Advanced shard awareness (shard-aware port)

Advanced shard awareness lets the driver choose a connection's source port so that Scylla assigns it to a desired shard. It uses the shard-aware native transport port.
It greatly reduces time and the number of connections needed to establish a connection per shard in some cases - ex. when many clients connect at once, or when there are non-shard-aware clients connected to the same cluster.

If you are using a custom Dialer and if your nodes expose the shard-aware port, it is highly recommended to update it so that it uses a specific source port when connecting.

* If you are using a custom `net.Dialer`, you can make your dialer honor the source port by wrapping it in a `gocql.ScyllaShardAwareDialer`:

  ```go
  oldDialer := net.Dialer{...}
  clusterConfig.Dialer := &gocql.ScyllaShardAwareDialer{oldDialer}
  ```

* If you are using a custom type implementing `gocql.Dialer`, you can get the source port by using the `gocql.ScyllaGetSourcePort` function.
  An example:

  ```go
  func (d *myDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
      sourcePort := gocql.ScyllaGetSourcePort(ctx)
      localAddr, err := net.ResolveTCPAddr(network, fmt.Sprintf(":%d", sourcePort))
      if err != nil {
          return nil, err
      }
	  d := &net.Dialer{LocalAddr: localAddr}
	  return d.DialContext(ctx, network, addr)
  }
  ```

  The source port might be already bound by another connection on your system.
  In such case, you should return an appropriate error so that the driver can retry with a different port suitable for the shard it tries to connect to.

  * If you are using `net.Dialer.DialContext`, this function will return an error in case the source port is unavailable, and you can just return that error from your custom `Dialer`.
  * Otherwise, if you detect that the source port is unavailable, you can either return `gocql.ErrScyllaSourcePortAlreadyInUse` or `syscall.EADDRINUSE`.

For this feature to work correctly, you need to make sure the following conditions are met:

* Your cluster nodes are configured to listen on the shard-aware port (`native_shard_aware_transport_port` option),
* Your cluster nodes are not behind a NAT which changes source ports,
* If you have a custom Dialer, it connects from the correct source port (see the guide above).

The feature is designed to gracefully fall back to the using the non-shard-aware port when it detects that some of the above conditions are not met.
The driver will print a warning about misconfigured address translation if it detects it.
Issues with shard-aware port not being reachable are not reported in non-debug mode, because there is no way to detect it without false positives.

Set `ClusterConfig.DisableShardAwarePort` to true to disable advanced shard awareness. Per-shard connection pooling and token-to-shard routing remain enabled through the regular CQL port.

### 5.2 Client routes (PrivateLink)

Scylla Cloud exposes a `system.client_routes` table that maps hosts to PrivateLink endpoints.
When configured, the driver can resolve and connect to the per-host PrivateLink address instead of using the public host IP.

This feature is also known as PrivateLink support, private link, private service connection, AWS PrivateLink (PL) and GCP Private Service Connect (PSC).
See the [Client routes (PrivateLink / Private Service Connect)](https://gocql-driver.docs.scylladb.com/stable/client-routes.html) documentation page for the full reference.

Use `WithClientRoutes` to enable it and pass the connection IDs you receive from Scylla Cloud:

```go
cluster := gocql.NewCluster("private-link.dns.name")
cluster.WithOptions(
	gocql.WithClientRoutes(
		gocql.WithEndpoints(
			gocql.ClientRoutesEndpoint{ConnectionID: "your-connection-id"},
		),
	),
)
```

If you also want to seed the cluster with PrivateLink hostnames, provide `ConnectionAddr` values in the endpoints list.

Advanced shard awareness is disabled by default when client routes are enabled because PrivateLink paths commonly use NAT. Basic shard awareness remains enabled: the driver still maintains per-shard connections and routes requests to the appropriate shard.

Opt in only when the client-route endpoint forwards to Scylla's Proxy Protocol v2 shard-aware CQL listener (`native_shard_aware_transport_port_proxy_protocol`, or `native_shard_aware_transport_port_ssl_proxy_protocol` for TLS) and the proxy sends the original client source port in the Proxy Protocol v2 header:

```go
cluster.WithOptions(
	gocql.WithClientRoutes(
		gocql.WithEndpoints(
			gocql.ClientRoutesEndpoint{ConnectionID: "your-connection-id"},
		),
		gocql.WithShardAwareness(true),
	),
)
```

`ClusterConfig.DisableShardAwarePort` takes precedence over `WithShardAwareness(true)`.

### 5.3 Iterator

Paging is a way to parse large result sets in smaller chunks.
The driver provides an iterator to simplify this process.

Use `Query.Iter()` to obtain iterator:

```go
iter := session.Query("SELECT id, value FROM my_table WHERE id > 100 AND id < 10000").Iter()
var results []int

var id, value int
for !iter.Scan(&id, &value) {
	if id%2 == 0 {
		results = append(results, value)
	}
}

if err := iter.Close(); err != nil {
    // handle error
}
```

In case of range and `ALLOW FILTERING` queries server can send empty responses for some pages.
That is why you should never consider empty response as the end of the result set.
Always check `iter.Scan()` result to know if there are more results, or `Iter.LastPage()` to know if the last page was reached.

### 5.4 Compression

To control network costs and traffic, you can enable compression.

Use `ClusterConfig.Compressor` to enable compression (either Snappy or LZ4):

```go
...
import (
    ...
    "github.com/gocql/gocql"
    "github.com/scylladb/gocql/lz4"
    ...
)

config := gocql.NewCluster("10.0.12.83", "10.0.13.04", "10.0.14.12")
config.Compressor = &gocql.SnappyCompressor{}
//or LZ4
config.Compressor = &lz4.LZ4Compressor{}
...
```

When explicitly using native protocol v5, use LZ4 compression or no compression.
`SnappyCompressor` does not support v5 transport segments and is rejected when
`ProtoVersion` is 5 or newer. Protocol v5 is not selected by automatic protocol
discovery; it must currently be configured explicitly.

LZ4 support lives in a sub-module with its own `go.mod`, published as
`github.com/scylladb/gocql/lz4`. Unlike the parent module, import it by that path directly --
it needs no `replace` directive of its own, only the one from the Installation section:

```mod
replace github.com/gocql/gocql => github.com/scylladb/gocql v1.20.0
```

Then run `go mod tidy`.

The path differs from the parent module's because upstream has folded its `lz4` package into
its main module: there is no `lz4/go.mod` there any more, so `github.com/gocql/gocql/lz4` is no
longer a module that can be released, and only its pre-merge versions exist. Every release that
supports native protocol v5 is published from this repository, under this repository's path.

**Upgrading from an earlier version:** delete the
`replace github.com/gocql/gocql/lz4 => ...` line from your `go.mod`, and change the import from
`github.com/gocql/gocql/lz4` to `github.com/scylladb/gocql/lz4`.

Keeping that `replace` stops working once gocql's own `go.mod` requires the module under its own
name, which it does from the first release carrying protocol v5 support onward
(`github.com/scylladb/gocql/lz4 v1.19.0`). The directive then points a second module path at a
module already in your graph under its own name, and Go rejects it outright:

```text
go: github.com/scylladb/gocql/lz4@v1.19.0 used for two different module paths
    (github.com/gocql/gocql/lz4 and github.com/scylladb/gocql/lz4)
```

gocql requires the module because its integration suite builds `LZ4Compressor` to exercise the
protocol v5 compressed-segment path, and Go has no way to mark a requirement as test-only. It
therefore appears in your module graph whether or not you compress -- but nothing outside
gocql's `_test.go` files imports it, so no lz4 code is linked into your binary unless you set
`ClusterConfig.Compressor` yourself. To build against a newer release than gocql asks for, add
your own `require github.com/scylladb/gocql/lz4 vX.Y.Z`; version selection takes the higher of
the two.

The two modules are released with the same version from one workflow run. The root module's
`go.mod` can still pin an older published LZ4 version until its post-release bump. The
repository tag for the nested module is
prefixed with its directory (`lz4/v1.19.0`), while the version in a `go.mod` directive is
`v1.19.0`.

## 6. Contributing

If you have any interest to be contributing in this GoCQL Fork, please read the [CONTRIBUTING.md](CONTRIBUTING.md) before initialize any Issue or Pull Request.
