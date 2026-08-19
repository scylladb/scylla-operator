package gocql

import (
	"encoding/json"
	"math"
	"reflect"
	"time"
)

// driverConfigStartupKey is the STARTUP option carrying the JSON description
// of the effective driver configuration. The configuration is identical for
// every connection of a session, so it is only sent on the control connection
// to keep the other STARTUP frames small.
const driverConfigStartupKey = "DRIVER_CONFIG"

// driverConfigVersion is the major version of the reported configuration schema.
// Adding keys to the report is backwards compatible and does not bump it, only
// changing or removing the meaning of an existing key does.
const driverConfigVersion = 1

// maxDriverConfigSize bounds the size of the DRIVER_CONFIG STARTUP option.
//
// writeStartupFrame serializes STARTUP options through writeString, which
// writes a 16-bit length prefix with no bounds check: a value over 65535
// bytes would silently truncate that prefix modulo 65536 while still
// appending the full body, corrupting the frame and failing the handshake.
// Custom policy objects allow implementation-specific properties, so a report
// built from user-supplied policy data could in principle grow large.
// Enforcing a limit here keeps "reporting must never prevent a connection
// from being established" a property of this code rather than of the user's
// configuration. 32KiB is generous for a configuration report and matches the
// limit csharp-driver and java-driver use for the same report.
const maxDriverConfigSize = 32 * 1024

// driverConfigReport is the value sent under driverConfigStartupKey, conforming
// to docs/driver-config-schema.json. It groups the effective configuration the
// way the schema does: connection-level settings, control-plane (system query
// and schema agreement) settings, and query-execution defaults.
// Field order here (and in the other report structs) is chosen for field
// alignment, not readability: it decides how much of the struct the GC has to
// scan, which govet's fieldalignment check enforces. The JSON key order it
// produces is not the schema's order, which is fine -- object key order is
// not significant in JSON.
type driverConfigReport struct {
	Connection   connectionReport   `json:"connection"`
	ControlPlane controlPlaneReport `json:"control-plane"`
	Query        queryReport        `json:"query"`
	Version      int                `json:"version"`
}

// connectionReport is the top-level "connection" group in the schema.
type connectionReport struct {
	// NodePreference is which nodes the driver opens connections to at all, a
	// different claim from query.load-balancing.node-preference (which of those
	// a query may be routed to). Omitted when nothing narrows pool membership;
	// see buildConnectionNodePreferenceReport.
	NodePreference any `json:"node-preference,omitempty"`
	// Read and Write are omitted entirely when their timeout is 0 (disabled),
	// matching the schema's "absent when unset or not applicable" convention
	// for optional groups. write.coalescing and the sibling connection.heartbeat
	// group are never populated: the schema declares both as
	// additionalProperties:false with no properties, an explicit placeholder
	// for a future schema version, so there is nothing a v1 report could put
	// there even though gocql does have both features (WriteCoalesceWaitTime,
	// the control connection's heartBeat()).
	Read  *readWriteTimeoutReport `json:"read,omitempty"`
	Write *readWriteTimeoutReport `json:"write,omitempty"`
	// TLS is omitted when TLS is not configured, or when a HostDialer is set:
	// HostDialer takes over the entire connection setup and SslOpts is ignored
	// (see ClusterConfig.SslOpts), so the effective TLS state is unknown.
	TLS          *tlsReport              `json:"tls,omitempty"`
	Reconnection reconnectionGroupReport `json:"reconnection"`
	Connect      connectReport           `json:"connect"`
	Requests     requestsReport          `json:"requests"`
	Pool         connectionPoolReport    `json:"pool"`
	Socket       socketReport            `json:"socket"`
}

// connectReport carries an optional timeout-ms: the schema does not list it
// under connect.required, so an empty "connect": {} is how a disabled connect
// timeout is reported. See positiveMillis.
type connectReport struct {
	TimeoutMs *int64 `json:"timeout-ms,omitempty"`
}

type readWriteTimeoutReport struct {
	TimeoutMs int64 `json:"timeout-ms"`
}

// requestsReport is connection.requests.
//
// Its orphaned group is never populated. Orphaned requests do exist here: a
// request whose caller stopped waiting keeps its stream reserved until the
// response arrives, precisely so the id is not reused (see StreamObserver's
// documentation, and the timeout branch of Conn.exec, which stops waiting
// without releasing the stream). What does not exist is any bound on them --
// no setting caps how many a connection may accumulate, and nothing closes and
// replaces a connection because too many did. The group asks for that bound,
// so there is nothing to report, and 0 would assert the opposite: close the
// connection on the very first orphan.
//
// The schema makes orphaned optional for exactly this case, and gives its
// absence that meaning, so omitting it produces a document a consumer
// validating against the schema accepts. See
// TestSchemaPermitsAnAbsentOrphanBound, which fails if that ever changes.
type requestsReport struct {
	InFlight inFlightReport `json:"in-flight"`
}

type inFlightReport struct {
	Max int `json:"max"`
}

type connectionPoolReport struct {
	ShardAware shardAwareReport `json:"shard-aware"`
}

type shardAwareReport struct {
	Enabled bool `json:"enabled"`
}

type socketReport struct {
	TCPNoDelay   bool `json:"tcp-no-delay"`
	KeepAlive    bool `json:"keep-alive"`
	ReuseAddress bool `json:"reuse-address"`
}

type reconnectionGroupReport struct {
	// Policy is one of reconnectionExponentialReport, reconnectionConstantReport,
	// reconnectionCustomReport, or nil (marshals to JSON null, the schema's "no
	// reconnection attempts will be made" branch).
	Policy any `json:"policy"`
}

// MaxAttempts is always present and always positive on both reports below: a
// policy that would attempt nothing is reported as no policy at all, so it
// never reaches these. The schema's "absent when attempts are unlimited" has
// no counterpart here -- GetMaxRetries returns a plain count, with no value
// standing for unlimited.
type reconnectionExponentialReport struct {
	Type        string `json:"type"`
	BaseMs      int64  `json:"base-ms"`
	MaxMs       int64  `json:"max-ms"`
	MaxAttempts int    `json:"max-attempts"`
}

type reconnectionConstantReport struct {
	Type        string `json:"type"`
	DelayMs     int64  `json:"delay-ms"`
	MaxAttempts int    `json:"max-attempts"`
}

type reconnectionCustomReport struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type tlsReport struct {
	HostnameVerification bool `json:"hostname-verification"`
}

// controlPlaneReport is the top-level "control-plane" group in the schema.
type controlPlaneReport struct {
	Queries controlPlaneQueriesReport `json:"queries"`
	Schema  controlPlaneSchemaReport  `json:"schema"`
}

type controlPlaneQueriesReport struct {
	System systemQueriesReport `json:"system"`
}

type systemQueriesReport struct {
	Timeout systemQueriesTimeoutReport `json:"timeout"`
}

type systemQueriesTimeoutReport struct {
	ClientSideMs *int64 `json:"client-side-ms,omitempty"`
	ServerSideMs *int64 `json:"server-side-ms,omitempty"`
}

type controlPlaneSchemaReport struct {
	Agreement schemaAgreementReport `json:"agreement"`
}

type schemaAgreementReport struct {
	TimeoutMs int64 `json:"timeout-ms"`
}

// queryReport is the top-level "query" group in the schema.
//
// It has no speculative-execution group: gocql has no session/cluster-wide
// default speculative execution policy, only a per-Query/per-Batch opt-in via
// SetSpeculativeExecutionPolicy. Every session's default is
// NonSpeculativeExecution (disabled), and the schema says to omit the group
// entirely when speculative execution is disabled -- which is always true of
// the session-wide default this report describes.
type queryReport struct {
	Retry         queryRetryReport         `json:"retry"`
	LoadBalancing queryLoadBalancingReport `json:"load-balancing"`
	Defaults      queryDefaultsReport      `json:"defaults"`
}

type queryDefaultsReport struct {
	Page *pageReport `json:"page,omitempty"`
	// Request is omitted entirely when Timeout is 0: the schema no longer
	// requires this group (only its inner timeout-ms was ever optional), so
	// there is no reason to send an empty {} object.
	Request *requestDefaultReport `json:"request,omitempty"`
	// Consistency carries one of the schema's nine non-serial levels, or is
	// omitted. Consistency's zero value (Any) is itself a real level, not a
	// sentinel for "unset", so absence here means only one thing: the
	// configured level is outside what the schema can express. See
	// consistencyName.
	Consistency string `json:"consistency,omitempty"`
	// SerialConsistency is omitted when 0, mirroring the ">0" check
	// ClusterConfig.Validate itself uses to treat the field as unset.
	SerialConsistency string `json:"serial-consistency,omitempty"`
	Idempotence       bool   `json:"idempotence"`
	// ClientTimestamps reports DefaultTimestamp as configured, not additionally
	// gated on the negotiated protocol version: the client-timestamp flag is
	// only suppressed below protocol v3 (frame.go), a corner case not worth
	// threading the negotiated version through the report for.
	ClientTimestamps bool `json:"client-timestamps"`
}

type pageReport struct {
	Size int `json:"size"`
}

type requestDefaultReport struct {
	TimeoutMs int64 `json:"timeout-ms"`
}

type queryRetryReport struct {
	// Policy is one of retryPolicySimpleReport, retryPolicyDowngradingReport, or
	// retryPolicyCustomReport.
	Policy any `json:"policy"`
	// Backoff is decoupled from Policy's discriminant on purpose: the schema
	// places it as a sibling of policy, not nested under a policy variant, so
	// a policy reported as "custom" (e.g. gocql's own
	// ExponentialBackoffRetryPolicy, which isn't one of the schema's built-in
	// retry types) can still usefully report the delay it's known to apply.
	// nil (omitted) when the effective policy has no such delay.
	Backoff any `json:"backoff,omitempty"`
}

type retryPolicySimpleReport struct {
	Type       string `json:"type"`
	MaxRetries int    `json:"max-retries"`
}

type retryPolicyDowngradingReport struct {
	Type string `json:"type"`
	// MaxRetries is DowngradingConsistencyRetryPolicy's own retry limit: its
	// Attempt method stops retrying once it has stepped through every entry of
	// ConsistencyLevelsToTry, so the slice's length is a concrete, always-known
	// retry limit -- not something to guess at or leave unset.
	MaxRetries int `json:"max-retries"`
}

type retryPolicyCustomReport struct {
	// MaxRetries is populated only for a custom policy whose retry limit the
	// driver can read. That is just the built-in
	// ExponentialBackoffRetryPolicy: the RetryPolicy interface exposes no
	// attempt limit, so a genuinely user-supplied policy has nothing to report
	// here.
	MaxRetries *int   `json:"max-retries,omitempty"`
	Type       string `json:"type"`
	Name       string `json:"name"`
}

type retryBackoffExponentialReport struct {
	Type   string `json:"type"`
	BaseMs int64  `json:"base-ms"`
	MaxMs  int64  `json:"max-ms"`
}

type queryLoadBalancingReport struct {
	// Policy is one of loadBalancingTokenAwareReport or loadBalancingCustomReport.
	Policy any `json:"policy"`
	// NodePreference is one of nodeLocationDCReport, nodeLocationRackReport, or
	// nil (omitted): gocql has no DC-inference feature, so the schema's
	// dc-auto/rack-auto branches are never produced.
	NodePreference any `json:"node-preference,omitempty"`
}

type loadBalancingTokenAwareReport struct {
	// AdaptiveOrdering is omitted when the policy orders candidates only by
	// the static rules above, which the schema spells "absent when adaptive
	// ordering is disabled".
	AdaptiveOrdering *adaptiveOrderingReport `json:"adaptive-ordering,omitempty"`
	Type             string                  `json:"type"`
	LoadDistribution string                  `json:"load-distribution"`
	// FallbackToNonPreferredNodes reports whether requests may fail over to
	// nodes outside of query.load-balancing.node-preference (DC or rack).
	FallbackToNonPreferredNodes bool `json:"fallback-to-non-preferred-nodes"`
}

// adaptiveOrderingReport names the runtime observations that reorder
// otherwise eligible candidates. The schema deliberately stops there: it
// records which signals feed the decision, not the algorithm or its
// thresholds, so AvoidSlowReplicas' MAX_IN_FLIGHT_THRESHOLD has nowhere to go
// and is not reported.
type adaptiveOrderingReport struct {
	Signals []string `json:"signals"`
}

type loadBalancingCustomReport struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type nodeLocationDCReport struct {
	Type    string `json:"type"`
	LocalDC string `json:"local-dc"`
}

type nodeLocationRackReport struct {
	Type      string `json:"type"`
	LocalDC   string `json:"local-dc"`
	LocalRack string `json:"local-rack"`
}

// driverConfigReporter builds the DRIVER_CONFIG STARTUP option describing a
// session's effective configuration to the cluster. It is created once per
// session and shared by all of its connections, but only ever contributes to
// the STARTUP options of the control connection.
//
// The report is rebuilt on every control connection (re-)establishment
// rather than cached, since some of what it describes (e.g. whether the
// server-side-ms control-plane timeout applies) is only known once a
// connection has completed its OPTIONS/SUPPORTED exchange.
//
// The reporter holds the *Session itself and builds the report lazily, at
// first use, rather than at construction time: newDriverConfigReporter runs
// inside newSessionCommon before fields such as s.policy are assigned, so a
// report built eagerly would see a partially initialized Session. For the
// same reason, the host selection policy is read off s.policy, not
// s.cfg.PoolConfig.HostSelectionPolicy: the latter is never assigned the
// default policy that newSessionCommon applies.
type driverConfigReporter struct {
	session *Session
}

func newDriverConfigReporter(s *Session) *driverConfigReporter {
	return &driverConfigReporter{session: s}
}

// updateStartupOptions adds the DRIVER_CONFIG STARTUP option.
//
// Only the control connection's startup holds a reporter, so this is not the
// place that decides which connections report: see Conn.init.
//
// isScyllaConn is known by the time this runs: startupCoordinator.options
// parses the OPTIONS/SUPPORTED exchange, which sets Conn.scyllaSupported,
// before calling startup, which is what builds these STARTUP options.
//
// Reporting is best effort: it must never prevent a connection from being
// established, so a report that cannot be built is logged and left out.
func (r *driverConfigReporter) updateStartupOptions(opts map[string]string, isScyllaConn bool) {
	report, err := r.buildReport(isScyllaConn)
	if err != nil {
		r.session.logger.Printf("gocql: unable to report driver configuration: %v", err)
		return
	}
	if len(report) > maxDriverConfigSize {
		r.session.logger.Printf("gocql: driver configuration report is %d bytes, exceeding the %d byte limit; omitting it", len(report), maxDriverConfigSize)
		return
	}
	opts[driverConfigStartupKey] = report
}

// buildReport returns the JSON configuration report of the session, marshalled
// fresh on every call so that it reflects the session's current state rather
// than a snapshot from whenever it was first requested.
func (r *driverConfigReporter) buildReport(isScyllaConn bool) (string, error) {
	cfg := &r.session.cfg
	report := driverConfigReport{
		Version:      driverConfigVersion,
		Connection:   buildConnectionReport(cfg),
		ControlPlane: buildControlPlaneReport(cfg, isScyllaConn),
		Query:        buildQueryReport(r.session),
	}
	data, err := json.Marshal(report)
	return string(data), err
}

func buildConnectionReport(cfg *ClusterConfig) connectionReport {
	report := connectionReport{
		Connect:      connectReport{TimeoutMs: positiveMillis(cfg.ConnectTimeout)},
		Requests:     buildRequestsReport(cfg),
		Pool:         connectionPoolReport{ShardAware: shardAwareReport{Enabled: shardAwareEnabled(cfg)}},
		Socket:       buildSocketReport(),
		Reconnection: reconnectionGroupReport{Policy: buildReconnectionPolicyReport(cfg.ReconnectionPolicy)},
	}
	if ms := positiveMillis(cfg.ReadTimeout); ms != nil {
		report.Read = &readWriteTimeoutReport{TimeoutMs: *ms}
	}
	if ms := positiveMillis(cfg.WriteTimeout); ms != nil {
		report.Write = &readWriteTimeoutReport{TimeoutMs: *ms}
	}
	report.TLS = buildTLSReport(cfg)
	report.NodePreference = buildConnectionNodePreferenceReport(cfg)
	return report
}

// buildConnectionNodePreferenceReport describes the part of the cluster the
// driver holds connections to.
//
// Only HostFilter narrows that: Session.init drops every host cfg.filterHost
// rejects, so such a host never gets a pool at all. The host selection policy's
// DC/rack preference is reported separately and does not belong here -- it
// orders and restricts routing among hosts that all still have pools.
//
// Of the filters shipped here only DataCenterHostFilter states a location.
// WhiteListHostFilter selects by address, which the schema has no branch for;
// AcceptAllFilter and DenyAllFilter state no location; and a caller-supplied
// HostFilterFunc is a closure this cannot see into. All of those report no
// preference.
func buildConnectionNodePreferenceReport(cfg *ClusterConfig) any {
	filter, ok := cfg.HostFilter.(dataCenterHostFilter)
	if !ok || filter.dataCenter == "" {
		return nil
	}
	return nodeLocationDCReport{Type: "dc", LocalDC: filter.dataCenter}
}

// buildTLSReport describes the TLS transport, or returns nil when there is
// nothing trustworthy to say: TLS is not configured, or a HostDialer has taken
// over connection setup entirely and SslOpts is ignored (see
// ClusterConfig.SslOpts).
//
// hostname-verification is the effective setting rather than
// SslOpts.EnableHostVerification, which is only one of its two inputs.
// setupTLSConfig only ever forces verification *on* -- it flips
// InsecureSkipVerify off when the caller asked to skip verification and set
// EnableHostVerification, and never the reverse. So a caller-supplied
// tls.Config with InsecureSkipVerify left false verifies the hostname even
// though EnableHostVerification is false, and reporting that flag alone
// inverts the truth for the most ordinary way of configuring TLS.
func buildTLSReport(cfg *ClusterConfig) *tlsReport {
	if cfg.SslOpts == nil || cfg.HostDialer != nil {
		return nil
	}
	// The config setupTLSConfig actually produced, once ValidateAndInitSSL has
	// run -- which is every session, but not a Session assembled by hand in a
	// test.
	if actual := cfg.getActualTLSConfig(); actual != nil {
		return &tlsReport{HostnameVerification: !actual.InsecureSkipVerify}
	}
	verify := cfg.SslOpts.EnableHostVerification
	if cfg.SslOpts.Config != nil {
		verify = !cfg.SslOpts.Config.InsecureSkipVerify || cfg.SslOpts.EnableHostVerification
	}
	return &tlsReport{HostnameVerification: verify}
}

// positiveMillis renders d for a schema field declared positiveInteger
// (minimum 1).
//
// It returns nil, so the key is omitted, when nothing is configured. Otherwise
// it floors at 1ms: durations under a millisecond truncate to 0, which the
// schema rejects, and a sub-millisecond timeout is still a timeout -- reporting
// the schema minimum is truer than either a 0 no consumer will accept or an
// absent key that would read as "disabled".
//
// Fields where 0 is itself meaningful (schema.agreement.timeout-ms, the
// constant reconnection policy's delay-ms) are nonNegativeInteger in the schema
// and must not go through here: they report their zero verbatim.
func positiveMillis(d time.Duration) *int64 {
	if d <= 0 {
		return nil
	}
	ms := max(d.Milliseconds(), 1)
	return &ms
}

// isNilPolicy reports whether v is nil, or a non-nil interface holding a nil
// pointer.
//
// A type switch routes only an untyped nil to `case nil`; a policy held as a
// typed nil pointer matches its own branch instead and panics on the first
// field it reads. Reporting runs on the goroutine that establishes a
// connection and must never do that to it, so every switch over a
// caller-supplied policy checks this first.
func isNilPolicy(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}

// nonNegativeMillis renders d for a schema field declared nonNegativeInteger.
// A negative duration is normalized to 0 rather than reported: time.Sleep
// returns immediately on one, so an immediate retry is what it means, and the
// schema has no encoding for a negative delay.
func nonNegativeMillis(d time.Duration) int64 {
	return max(d.Milliseconds(), 0)
}

func buildRequestsReport(cfg *ClusterConfig) requestsReport {
	return requestsReport{InFlight: inFlightReport{Max: effectiveInFlightMax(cfg.MaxRequestsPerConn)}}
}

// defaultMaxStreams mirrors streams.New(), the generator Session.streamIDGenerator
// uses when MaxRequestsPerConn is left unset.
const defaultMaxStreams = 32768

// maxReportableInFlight is the most requests a connection can ever have
// outstanding: the CQL v3+ stream id is a signed 16-bit value, so ids 0..32767
// are all there are. The schema constrains this field only to a positive
// integer, so this bound is the protocol's, not the schema's.
const maxReportableInFlight = 32767

// effectiveInFlightMax reports how many requests can actually be in flight on
// one connection, which is not the configured limit. streams.NewLimited rounds
// the limit up to a multiple of 64 and then reserves stream 0, so a configured
// 100 really permits 127, and the 32768-stream default really permits 32767.
//
// A configured value above the stream-id range is clamped: gocql does not
// reject it, but the protocol cannot address those streams, so reporting it
// verbatim would claim a concurrency the connection can never reach.
func effectiveInFlightMax(configured int) int {
	if configured <= 0 {
		configured = defaultMaxStreams
	}
	if configured >= maxReportableInFlight {
		// Short-circuit before the rounding below, which overflows to a
		// negative within 63 of the integer limit. The result is capped here
		// anyway, so nothing is lost. ClusterConfig.Validate rejects only a
		// negative MaxRequestsPerConn, so a value that large does reach this.
		return maxReportableInFlight
	}
	inFlight := (((configured + 63) / 64) * 64) - 1
	return min(inFlight, maxReportableInFlight)
}

// shardAwareEnabled reports whether the client can reach ScyllaDB's
// shard-aware port at all. Two independent things have to hold, and the
// schema has one boolean for both.
//
// DisableShardAwarePort must be unset: it makes scyllaConnPicker.NextShard
// return no shard, which sends every dial down the plain path.
//
// The dialer must also be able to target a shard. Session.dialWithoutObserver
// type-asserts HostDialer to ShardDialer and silently falls back to DialHost
// when the assertion fails, so a custom HostDialer that does not implement
// DialShard can never reach the shard-aware port however the flag is set.
// A nil HostDialer is capable: connConfig substitutes a *scyllaDialer, which
// implements ShardDialer.
func shardAwareEnabled(cfg *ClusterConfig) bool {
	if cfg.DisableShardAwarePort {
		return false
	}
	if cfg.HostDialer == nil {
		return true
	}
	_, ok := cfg.HostDialer.(ShardDialer)
	return ok
}

// buildSocketReport describes what gocql's own built-in dialer does with a
// TCP socket. A user-supplied Dialer/HostDialer can set one up differently;
// the driver has no way to observe that, so the report always describes the
// built-in dialer's behavior, per the schema's own framing of these fields as
// "the effective value ... the OS/platform default".
func buildSocketReport() socketReport {
	return socketReport{
		// Go's net package unconditionally disables Nagle's algorithm for every
		// TCP connection it creates (net.newTCPConn calls setNoDelay(fd, true));
		// gocql never overrides that.
		TCPNoDelay: true,
		// cfg.SocketKeepalive can't be negative (see ClusterConfig.Validate), and
		// net.Dialer.KeepAlive == 0 still means "enabled, OS default period" --
		// there is currently no way to disable keep-alive through gocql's
		// configuration.
		KeepAlive: true,
		// gocql never sets SO_REUSEADDR.
		ReuseAddress: false,
	}
}

// buildReconnectionPolicyReport describes the policy hostConnPool.connect will
// drive, or nil for the schema's null branch, "no reconnection attempts will
// be made".
//
// A policy whose retry limit is not positive takes that null branch whatever
// its type: connect loops `for i := 0; i < GetMaxRetries(); i++`, so it makes
// no attempt at all. Reporting it as a policy with max-attempts omitted would
// state the opposite, since the schema reads an absent max-attempts as
// unlimited attempts.
func buildReconnectionPolicyReport(rp ReconnectionPolicy) any {
	if isNilPolicy(rp) {
		return nil
	}
	switch p := rp.(type) {
	case *NoReconnectionPolicy:
		return nil
	case *ConstantReconnectionPolicy:
		if p.MaxRetries <= 0 {
			return nil
		}
		return reconnectionConstantReport{
			Type:        "constant",
			DelayMs:     nonNegativeMillis(p.Interval),
			MaxAttempts: p.MaxRetries,
		}
	case *ExponentialReconnectionPolicy:
		if p.MaxRetries <= 0 {
			return nil
		}
		maxInterval := p.MaxInterval
		if maxInterval < p.InitialInterval {
			// Matches ExponentialReconnectionPolicy.GetInterval's own fallback.
			maxInterval = math.MaxInt16 * time.Second
		}
		// GetInterval delegates to getExponentialTime, which substitutes its own
		// defaults for a non-positive bound, so report the window retries
		// actually use rather than a zero the schema rejects (both fields are
		// positiveInteger).
		base, maxDelay := exponentialDelayWindow(p.InitialInterval, maxInterval)
		return reconnectionExponentialReport{
			Type:        "exponential",
			BaseMs:      base,
			MaxMs:       maxDelay,
			MaxAttempts: p.MaxRetries,
		}
	default:
		return reconnectionCustomReport{Type: "custom", Name: customPolicyName(rp)}
	}
}

func buildControlPlaneReport(cfg *ClusterConfig, isScyllaConn bool) controlPlaneReport {
	var timeout systemQueriesTimeoutReport
	if clientMs := positiveMillis(cfg.MetadataSchemaRequestTimeout); clientMs != nil {
		timeout.ClientSideMs = clientMs
		if isScyllaConn {
			// The USING TIMEOUT clause is ScyllaDB-only, so this key only ever
			// applies against Scylla -- and it carries whatever
			// Conn.setSystemRequestTimeout put in the clause, which
			// truncates rather than floors: a sub-millisecond timeout is sent as
			// "USING TIMEOUT 0ms". Derive it from the same conversion so the
			// report cannot claim a clause the connection never sends, and omit
			// the key when that conversion yields 0, which the schema's
			// positiveInteger cannot carry.
			//
			// client-side-ms is deliberately not aligned with this: it bounds a
			// Go-side deadline that a sub-millisecond duration expresses just
			// fine, so it keeps positiveMillis' floor.
			if serverMs := cfg.MetadataSchemaRequestTimeout.Milliseconds(); serverMs > 0 {
				timeout.ServerSideMs = &serverMs
			}
		}
	}
	return controlPlaneReport{
		Queries: controlPlaneQueriesReport{System: systemQueriesReport{Timeout: timeout}},
		Schema: controlPlaneSchemaReport{
			// Always emitted, including 0: the schema documents 0 as meaningful
			// ("do not wait for agreement"), not as "unset".
			Agreement: schemaAgreementReport{TimeoutMs: cfg.MaxWaitSchemaAgreement.Milliseconds()},
		},
	}
}

func buildQueryReport(s *Session) queryReport {
	cfg := &s.cfg
	return queryReport{
		Defaults:      buildQueryDefaultsReport(cfg),
		Retry:         buildQueryRetryReport(cfg),
		LoadBalancing: buildLoadBalancingReport(s.policy),
	}
}

func buildQueryDefaultsReport(cfg *ClusterConfig) queryDefaultsReport {
	report := queryDefaultsReport{
		Consistency:      consistencyName(cfg.Consistency),
		Idempotence:      cfg.DefaultIdempotence,
		ClientTimestamps: cfg.DefaultTimestamp,
	}
	if cfg.PageSize > 0 {
		report.Page = &pageReport{Size: cfg.PageSize}
	}
	if cfg.SerialConsistency.IsSerial() {
		report.SerialConsistency = cfg.SerialConsistency.String()
	}
	if ms := positiveMillis(cfg.Timeout); ms != nil {
		report.Request = &requestDefaultReport{TimeoutMs: *ms}
	}
	return report
}

// consistencyName renders c for query.defaults.consistency, and returns "" for
// a value the schema's enum cannot carry so the key is omitted.
//
// The enum covers every level Consistency defines, serial ones included, so
// only an unrecognized numeric value is unreportable -- Consistency.String()
// renders those as "UNKNOWN_CONS_0x..", which no consumer can interpret.
// Nothing rejects such a value today; the fix belongs in ClusterConfig.Validate
// and is left to a follow-up.
func consistencyName(c Consistency) string {
	switch c {
	case Any, One, Two, Three, Quorum, All, LocalQuorum, EachQuorum, LocalOne, Serial, LocalSerial:
		return c.String()
	default:
		return ""
	}
}

func buildQueryRetryReport(cfg *ClusterConfig) queryRetryReport {
	rp := cfg.RetryPolicy
	if rp == nil {
		// Mirrors query_executor.go's own fallback for an unset RetryPolicy.
		rp = defaultRetryPolicy
	}
	return queryRetryReport{
		Policy:  buildRetryPolicyReport(rp),
		Backoff: buildRetryBackoffReport(rp),
	}
}

// nonNegativeRetries clamps a policy's retry count for max-retries, which the
// schema declares nonNegativeInteger.
//
// Nothing validates RetryPolicy -- ClusterConfig.Validate does not look at it
// at all -- so a negative count reaches the report. Clamping loses nothing:
// both built-in policies compare the attempt number against this field
// (Attempts() <= NumRetries, Attempts() > NumRetries), and since the first
// attempt is 1, every negative count refuses the first retry exactly as 0
// does.
func nonNegativeRetries(numRetries int) int {
	return max(numRetries, 0)
}

func buildRetryPolicyReport(rp RetryPolicy) any {
	if isNilPolicy(rp) {
		// Nothing can be read off it, but its type still names something.
		return retryPolicyCustomReport{Type: "custom", Name: customPolicyName(rp)}
	}
	switch p := rp.(type) {
	case *SimpleRetryPolicy:
		return retryPolicySimpleReport{Type: "simple", MaxRetries: nonNegativeRetries(p.NumRetries)}
	case *DowngradingConsistencyRetryPolicy:
		return retryPolicyDowngradingReport{Type: "downgrading-consistency", MaxRetries: len(p.ConsistencyLevelsToTry)}
	case *ExponentialBackoffRetryPolicy:
		// Not one of the schema's built-in retry-policy types, so it is reported
		// as custom like any other RetryPolicy implementation -- but its retry
		// limit and its backoff are both readable, and the schema's custom
		// branch accepts max-retries. The backoff surfaces separately, as a
		// sibling of the policy: see buildRetryBackoffReport.
		maxRetries := nonNegativeRetries(p.NumRetries)
		return retryPolicyCustomReport{
			Type:       "custom",
			Name:       customPolicyName(rp),
			MaxRetries: &maxRetries,
		}
	default:
		return retryPolicyCustomReport{Type: "custom", Name: customPolicyName(rp)}
	}
}

func buildRetryBackoffReport(rp RetryPolicy) any {
	backoff, ok := rp.(*ExponentialBackoffRetryPolicy)
	if !ok || isNilPolicy(rp) {
		return nil
	}
	base, maxDelay := exponentialDelayWindow(backoff.Min, backoff.Max)
	return retryBackoffExponentialReport{Type: "exponential", BaseMs: base, MaxMs: maxDelay}
}

// exponentialDelayWindow reports the delay window getExponentialTime
// (policies.go) actually produces for a configured min/max pair, in
// milliseconds.
//
// Both the exponential retry backoff and the exponential reconnection policy
// route their delays through getExponentialTime, and both describe the result
// with schema fields declared positiveInteger where max-ms must be >= base-ms.
// Neither invariant survives the configured values untouched:
//
//   - a non-positive bound is not used as-is; getExponentialTime substitutes
//     100ms for the minimum and 10s for the maximum, so reporting the raw 0
//     would name a delay the policy never waits and one the schema rejects.
//   - a maximum below the minimum caps every delay at the maximum, so the
//     minimum is never reached. Reporting it as base-ms would both misdescribe
//     the policy and break max-ms >= base-ms.
//   - a sub-millisecond bound truncates to 0, so both are floored the same way
//     positiveMillis floors the report's other positiveInteger durations.
func exponentialDelayWindow(configuredMin, configuredMax time.Duration) (baseMs, maxMs int64) {
	base := effectiveExpBackoff(configuredMin, 100*time.Millisecond)
	maxDelay := effectiveExpBackoff(configuredMax, 10*time.Second)
	if maxDelay < base {
		base = maxDelay
	}
	return max(base.Milliseconds(), 1), max(maxDelay.Milliseconds(), 1)
}

func effectiveExpBackoff(configured, fallback time.Duration) time.Duration {
	if configured <= 0 {
		return fallback
	}
	return configured
}

func buildLoadBalancingReport(policy HostSelectionPolicy) queryLoadBalancingReport {
	policy = unwrapHostSelectionPolicy(policy)
	return queryLoadBalancingReport{
		Policy:         buildLoadBalancingPolicyReport(policy),
		NodePreference: buildNodeLocationPreferenceReport(policy),
	}
}

func buildLoadBalancingPolicyReport(policy HostSelectionPolicy) any {
	tap, ok := policy.(*tokenAwareHostPolicy)
	if !ok || isNilPolicy(policy) {
		// The schema's built-in load-balancing type only ever admits
		// "token-aware" (matching csharp-driver/java-driver's own narrowing);
		// any other policy -- including gocql's plain RoundRobinHostPolicy or
		// DCAwareRoundRobinPolicy used without token-awareness -- is reported as
		// custom.
		return loadBalancingCustomReport{Type: "custom", Name: customPolicyName(policy)}
	}
	distribution := "replica-set"
	if tap.shuffleReplicas {
		distribution = "shuffle"
	}
	report := loadBalancingTokenAwareReport{
		Type:                        "token-aware",
		LoadDistribution:            distribution,
		FallbackToNonPreferredNodes: fallbackToNonPreferredNodesAllowed(tap),
	}
	if tap.avoidSlowReplicas {
		// AvoidSlowReplicas makes Pick partition the replica set so hosts at or
		// above MAX_IN_FLIGHT_THRESHOLD outstanding requests sort last: see
		// partitionHealthy, which orders by HostInfo.IsBusy. That is adaptive
		// ordering, and the count of outstanding requests is the one signal it
		// reads.
		report.AdaptiveOrdering = &adaptiveOrderingReport{Signals: []string{"in-flight-requests"}}
	}
	return report
}

// maxPolicyUnwrapDepth bounds the walk below. A wrapper cycle is not
// reachable through the constructors, but the report must never hang a
// connection over a policy someone assembled by hand.
const maxPolicyUnwrapDepth = 16

// unwrapHostSelectionPolicy peels driver-supplied wrappers off a host
// selection policy so the report describes the policy the caller actually
// configured rather than the wrapper around it.
//
// SingleHostReadyPolicy is the only such wrapper, and it is transparent to
// routing: it forwards every selection decision to the policy it holds and
// adds only a readiness signal. Left wrapped, both the load-balancing
// discriminant and the DC/rack preference behind it would be lost, which is
// everything the group exists to report.
func unwrapHostSelectionPolicy(p HostSelectionPolicy) HostSelectionPolicy {
	for range maxPolicyUnwrapDepth {
		wrapper, ok := p.(*singleHostReadyPolicy)
		if !ok || isNilPolicy(p) {
			return p
		}
		p = wrapper.HostSelectionPolicy
	}
	return p
}

// fallbackToNonPreferredNodesAllowed reports whether a request can reach a
// node outside the DC/rack preference this policy reports.
//
// Two separate things can let it. NonLocalReplicasFallback makes Pick serve
// replicas from remote tiers itself, out of its own `remote` buckets and
// before it ever delegates to the fallback policy, so requests escape the
// preference however the fallback is configured. Failing that it comes down to
// the fallback: only dcAwareRR and rackAwareRR confine requests at all, and
// only when DC failover is disabled. Any other fallback has no DC/rack notion,
// so nothing is confined.
func fallbackToNonPreferredNodesAllowed(tap *tokenAwareHostPolicy) bool {
	if tap.nonLocalReplicasFallback {
		return true
	}
	if isNilPolicy(tap.fallback) {
		return true
	}
	switch p := tap.fallback.(type) {
	case *dcAwareRR:
		// The preference is the whole local DC, and disabling failover confines
		// requests to exactly that.
		return !p.dcFailoverDisabled()
	case *rackAwareRR:
		// The preference reported for this fallback names a rack as well as a
		// DC, but Pick always serves tier 1 -- the local DC's other racks --
		// before giving up, whether or not DC failover is disabled. Requests
		// therefore always reach nodes outside the reported preference;
		// disabling failover only stops them leaving the DC, which the schema's
		// single boolean cannot express.
		return true
	default:
		// No DC/rack notion at all, so nothing is confined.
		return true
	}
}

// buildNodeLocationPreferenceReport looks for a DC/rack preference on policy
// itself or, if policy is token-aware, on its fallback -- there is no separate
// session-level DC/rack setting, only what a DCAwareRoundRobinPolicy or
// RackAwareRoundRobinPolicy carries. Returns nil (omitted) for any other
// policy: the driver infers no datacenter, so the schema's dc-auto/rack-auto
// branches are never produced.
//
// An empty datacenter or rack is also reported as no preference. The schema
// declares both local-dc and local-rack nonEmptyString and requires them on
// the branch that names them, so an empty one cannot be reported -- and a
// policy constructed with an empty name expresses no preference anyway.
func buildNodeLocationPreferenceReport(policy HostSelectionPolicy) any {
	if isNilPolicy(policy) {
		return nil
	}
	target := policy
	if tap, ok := policy.(*tokenAwareHostPolicy); ok {
		target = tap.fallback
	}
	if isNilPolicy(target) {
		return nil
	}
	switch p := target.(type) {
	case *dcAwareRR:
		if p.localDatacenter() == "" {
			return nil
		}
		return nodeLocationDCReport{Type: "dc", LocalDC: p.localDatacenter()}
	case *rackAwareRR:
		if p.localDatacenter() == "" || p.localRackName() == "" {
			return nil
		}
		return nodeLocationRackReport{Type: "rack", LocalDC: p.localDatacenter(), LocalRack: p.localRackName()}
	default:
		return nil
	}
}

// customPolicyName returns a short, package-qualifier-free name for a
// user-supplied policy value -- e.g. *mypkg.MyPolicy becomes "MyPolicy" --
// mirroring how java-driver/csharp-driver report a policy's simple class name
// rather than its fully qualified one.
func customPolicyName(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		// A nil policy: reflect.TypeOf returns a nil Type, which would panic
		// below. The schema declares name nonEmptyString, so there is no way to
		// say "no name" -- and a report that omitted it would be as wrong as one
		// that crashed the connection reporting it.
		return "unknown"
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if name := t.Name(); name != "" {
		return name
	}
	// An unnamed type, such as an anonymous struct embedding the policy
	// interface. Name() is empty for those, which nonEmptyString rejects.
	return t.String()
}
