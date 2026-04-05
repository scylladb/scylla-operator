# pkg/cmd

All cobra/flag wiring for the operator binaries. Contains **no business logic** — only command scaffolding, client construction, informer startup, and controller orchestration. Business logic lives in `pkg/controller/`.

## Layout

```
pkg/cmd/
├── operator/           # scylla-operator binary commands
│   ├── cmd.go          # root command: composes subcommands, PersistentPreRunE, klog, feature gates
│   ├── operator.go     # "operator" subcommand: leader election, informers, NewController calls
│   ├── webhooks.go     # "webhooks" subcommand: TLS, admission validation server
│   ├── sidecar/        # "sidecar" subcommand (runs inside Scylla pods)
│   ├── probeserver/    # "serve-probes" subcommand
│   ├── bootstrapbarrier/ # "bootstrap-barrier" subcommand
│   ├── nodesetupdaemon.go
│   ├── ignition.go
│   ├── mustgather.go
│   ├── cleanupjob.go
│   └── rlimitsjob.go
├── tests/              # scylla-operator-tests binary
│   ├── tests.go        # root command
│   ├── tests_run.go    # "run" subcommand
│   └── options.go      # TestFrameworkOptions
├── generateapireference/ # gen-api-reference binary
└── version/            # version subcommand (shared)
```

## Cobra command pattern

Every command follows the same structure:

```go
// 1. Options struct with sensible defaults
func NewFooOptions(streams genericclioptions.IOStreams) *FooOptions {
    return &FooOptions{
        ClientConfig: genericclioptions.NewClientConfig("scylla-operator"),
        LeaderElection: genericclioptions.NewLeaderElection(),
        // ...
    }
}

// 2. AddFlags registers all flags (including embedded options)
func (o *FooOptions) AddFlags(cmd *cobra.Command) {
    o.ClientConfig.AddFlags(cmd)
    o.LeaderElection.AddFlags(cmd)
    cmd.Flags().IntVar(&o.ConcurrentSyncs, "concurrent-syncs", ...)
}

// 3. NewFooCmd builds the cobra.Command
func NewFooCmd(streams genericclioptions.IOStreams) *cobra.Command {
    o := NewFooOptions(streams)
    cmd := &cobra.Command{
        Use: "foo", Short: "...",
        SilenceErrors: true, SilenceUsage: true,
        RunE: func(cmd *cobra.Command, args []string) error {
            if err := o.Complete(); err != nil { return err }
            if err := o.Validate(); err != nil { return err }
            return o.Run(streams, cmd)
        },
    }
    o.AddFlags(cmd)
    return cmd
}

// 4. Validate — check flag constraints only
// 5. Complete — build clientsets from ClientConfig.RestConfig / ProtoConfig
// 6. Run — set up signals, ctx, and start the main logic
```

Add new commands to the root in `pkg/cmd/operator/cmd.go`:
```go
cmd.AddCommand(NewFooCmd(streams))
```

## Generic CLI options (pkg/genericclioptions)

Embed these into your `Options` struct rather than rolling your own:

| Type | Provides | Key method |
|---|---|---|
| `ClientConfig` | `RestConfig`, `ProtoConfig`, QPS/Burst, kubeconfig flag | `Complete()` loads kubeconfig |
| `MultiDatacenterClientConfig` | Worker kubeconfigs map (e2e tests) | `Complete()` builds per-worker configs |
| `InClusterReflection` | `--namespace` flag, auto-detects from pod | `Complete()` |
| `LeaderElection` | Leader election duration flags | `AddFlags(cmd)` |
| `IOStreams` | `In`, `Out`, `ErrOut` writers | — |

```go
// Build clientsets in Complete():
kubeClient, err := kubernetes.NewForConfig(o.ProtoConfig)  // use ProtoConfig for k8s
scyllaClient, err := scyllav.NewForConfig(o.RestConfig)    // use RestConfig for CRD clients
```

## Environment variable → flag mapping

`pkg/cmdutil.ReadFlagsFromEnv(prefix, cmd)` is called in each root's `PersistentPreRunE`. It maps every unflagged flag to an env var:

```
env var = uppercase(prefix + flag-name).replace('-', '_')

Example: prefix="SCYLLA_OPERATOR_", flag="concurrent-syncs"
      → SCYLLA_OPERATOR_CONCURRENT_SYNCS
```

The operator uses `naming.OperatorEnvVarPrefix`; tests use `"SCYLLA_OPERATOR_TESTS_"`.

## Feature gates & logging

Both root commands call these in `NewOperatorCommand` / `NewTestsCommand`:

```go
cmdutil.InstallKlog(cmd)                                  // adds --loglevel persistent flag
utilfeature.DefaultMutableFeatureGate.AddFlag(cmd.PersistentFlags()) // adds --feature-gates
```

These are persistent flags — all subcommands inherit them automatically.

## Operator startup sequence

```
PersistentPreRunE:
  maxprocs.Set()
  cmdutil.ReadFlagsFromEnv(naming.OperatorEnvVarPrefix, cmd)

Complete():
  o.ClientConfig.Complete()         // load kubeconfig, set RestConfig/ProtoConfig
  o.InClusterReflection.Complete()  // detect namespace
  o.LeaderElection.Complete()
  kubernetes.NewForConfig(o.ProtoConfig)   → kubeClient
  scyllaversionedclient.NewForConfig(...)  → scyllaClient
  // ... monitoring, remote clients ...

Run() → Execute() → leaderelection.Run(ctx, ..., func(ctx) { o.run(ctx) })

run():
  rsaKeyGenerator := ocrypto.NewRSAKeyGenerator(...)
  kubeInformers  := informers.NewSharedInformerFactory(kubeClient, resyncPeriod)
  scyllaInformers := scyllainformers.NewSharedInformerFactory(scyllaClient, resyncPeriod)
  // ...create all controllers via NewController(...)...
  go rsaKeyGenerator.Run(ctx)
  go kubeInformers.Start(ctx.Done())
  go scyllaInformers.Start(ctx.Done())
  go controller.Run(ctx, o.ConcurrentSyncs)  // one goroutine per controller
  <-ctx.Done()
```

## Adding a controller command

1. Create `pkg/cmd/operator/myfeature.go` with `NewMyFeatureOptions`, `AddFlags`, `NewMyFeatureCmd`, `Validate`, `Complete`, `Run`.
2. In `Complete()`, use `o.ClientConfig.Complete()` then build clientsets.
3. In `Run()`, create informer factories → `NewController(...)` → `factory.Start(ctx.Done())` → `controller.Run(ctx, workers)`.
4. Register in `pkg/cmd/operator/cmd.go`: `cmd.AddCommand(NewMyFeatureCmd(streams))`.
5. Env-var support is automatic via the root `PersistentPreRunE`.

## Hard rules

- **Never put business logic here.** If it's more than wiring, it belongs in `pkg/controller/`.
- **`cmd/`** contains only thin `main()` entrypoints — actual cobra setup lives in `pkg/cmd/`.
- Always start informer factories **before** calling `controller.Run`.
- Pass `o.ProtoConfig` to Kubernetes core clients; `o.RestConfig` to CRD/custom clients.
