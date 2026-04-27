# pkg/helpers

General-purpose utility library. Imported by ~45 packages across the repo. **Before writing any utility function, check here first.**

## Packages

| Import path | Alias | Purpose |
|---|---|---|
| `github.com/scylladb/scylla-operator/pkg/helpers` | (none) | Collections, networking, merge/patch, fs, args, status, cache |
| `github.com/scylladb/scylla-operator/pkg/helpers/slices` | `oslices` | Generic slice utilities |
| `github.com/scylladb/scylla-operator/pkg/helpers/managerclienterrors` | — | Scylla Manager HTTP error helpers |

Always import the slices subpackage as `oslices` — that is the established alias across the entire codebase.

---

## Collections

```go
MergeMaps[Key comparable, Value any](maps ...map[Key]Value) map[Key]Value
// Merge multiple maps into a new map. Later maps win on key collisions.
```

---

## Slices (`oslices`)

```go
ToSlice[T any](objs ...T) []T
ConvertToSlice[To, From any](convert func(From) To, objs ...From) []To
ConvertSlice[To, From any](slice []From, convert func(From) To) []To

Filter[T any](array []T, filterFunc func(T) bool) []T
FilterOut[T any](array []T, filterOutFunc func(T) bool) []T
FilterOutNil[T any](array []*T) []*T

Contains[T any](array []T, cmp func(v T) bool) bool
ContainsItem[T comparable](slice []T, item T) bool
IdentityFunc[T comparable](item T) func(T) bool   // returns a cmp func for ContainsItem

Find[T any](slice []T, filterFunc func(T) bool) (T, int, bool)
FindItem[T comparable](slice []T, item T) (T, int, bool)

ToString[T ~string](v T) string
```

---

## Networking

```go
ParseIP(s string) (net.IP, error)
ParseIPs(ipStrings []string) ([]net.IP, error)
NormalizeIPs(ips []net.IP) []net.IP          // converts each to 16-byte To16() form
IsIPv6(ip net.IP) bool                        // ip.To4() == nil
IsIPv4(ip net.IP) bool                        // ip.To4() != nil
GetIPFamily(ip net.IP) corev1.IPFamily

// Pick preferred IP from a list
GetPreferredIP(ips []string, preferredFamily *corev1.IPFamily) (string, error)
GetPreferredPodIP(pod *corev1.Pod, preferredFamily *corev1.IPFamily) (string, error)
GetPreferredServiceIP(svc *corev1.Service, preferredFamily *corev1.IPFamily) (string, bool)
GetPreferredServiceIPFamily(svc *corev1.Service) corev1.IPFamily
DetectEndpointsIPFamily(endpoints []discoveryv1.Endpoint) corev1.IPFamily
IPFamilyToAddressType(ipFamily corev1.IPFamily) discoveryv1.AddressType
```

---

## Merge / patch

```go
CreateTwoWayMergePatch[T runtime.Object](original, modified T) ([]byte, error)
// Strategic merge patch (JSON bytes) between two k8s objects.
```

---

## Args parsing

```go
ParseScyllaArguments(scyllaArguments string) map[string]*string
// Parse Scylla daemon argument string.
// nil *string = flag present without value (e.g. "--developer-mode")
// non-nil *string = raw parsed value (may still contain surrounding quotes)
```

---

## Filesystem

```go
TouchFile(filePath string) error
// Create file if missing, or update mtime. Does NOT create parent directories.
```

---

## Status conditions

```go
IsStatusConditionPresentAndTrue(conditions []metav1.Condition, conditionType string, generation int64) bool
IsStatusConditionPresentAndFalse(conditions []metav1.Condition, conditionType string, generation int64) bool
IsStatusConditionPresentAndEqual(conditions []metav1.Condition, conditionType string, status metav1.ConditionStatus, generation int64) bool

// NodeConfig-specific variants:
IsStatusNodeConfigConditionPresentAndTrue(...)
IsStatusNodeConfigConditionPresentAndFalse(...)
IsStatusNodeConfigConditionPresentAndEqual(...)
```

All condition helpers validate that `ObservedGeneration` matches `generation`.

---

## Cache

```go
UncachedListFunc(f func(options metav1.ListOptions) (runtime.Object, error)) func(...)
// Wraps a List function so ResourceVersion "0" becomes "" — bypasses apiserver watch cache.
// Use when you need a strictly-current list, not a cached snapshot.
```

---

## Misc

```go
Must[T any](r T, err error) T
// Panics on non-nil error. Use ONLY in init() or test helpers — never in reconcile loops.
```

---

## managerclienterrors

```go
IsBadRequest(err error) bool   // HTTP 400
IsNotFound(err error) bool     // HTTP 404
GetPayloadMessage(err error) string  // payload.Message or err.Error()
```

---

## Gotchas

| Function | Pitfall |
|---|---|
| `NormalizeIPs` | Returns 16-byte (`To16`) form. IPv4 addresses become IPv4-mapped IPv6 — don't compare with `To4()` results. |
| `ParseScyllaArguments` | Quoted values keep their quotes. Strip them if passing to downstream code that expects unquoted strings. |
| `GetPreferredIP` | Silently skips invalid IPs. If none match `preferredFamily`, returns the first valid IP — not an error. |
| `GetPreferredServiceIP` | Returns `("", false)` (not an error) when no suitable ClusterIP exists. |
| `Must` | **Panics.** Only for test/init contexts. |
| `TouchFile` | Parent directory must exist. Returns an error if it doesn't. |
| `UncachedListFunc` | Changes `ResourceVersion` semantics — only use when you explicitly need to bypass watch cache. |
