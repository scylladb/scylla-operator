# Log

This is a wrapper over [Uber zap](https://github.com/uber-go/zap) library that replaces the sugared logger. 

Features:

* Syslog integration
* Automatic stacktraces for errors
* Context aware tracing ID
* Easy to use
* Fast!

Example:

```go
logger, err := log.NewProduction(log.Config{
	Mode:  log.SyslogMode,
	Level: zapcore.InfoLevel,
})
if err != nil {
	t.Fatal(err)
}
logger.Info(ctx, "Could not connect to database",
	"sleep", 5*time.Second,
	"error", errors.New("I/O error"),
)
logger.Named("sub").Error(ctx, "Unexpected error", "error", errors.New("unexpected"))
```

## Benchmarks

Benchmark results of running against zap and zap sugared loggers on Intel(R) Core(TM) i7-7500U CPU @ 2.70GHz.

```
BenchmarkZap-4                   2000000               978 ns/op             256 B/op          1 allocs/op
BenchmarkZapSugared-4            1000000              1353 ns/op             528 B/op          2 allocs/op
BenchmarkLogger-4                1000000              1167 ns/op             256 B/op          1 allocs/op
```
