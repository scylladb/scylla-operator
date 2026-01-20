package gocql

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocql/gocql/events"
	"github.com/gocql/gocql/internal/debug"
	"github.com/gocql/gocql/internal/eventbus"
)

type ClientRoutesEndpoint struct {
	// Scylla Cloud ConnectionID to read from `system.client_routes`
	ConnectionID string

	// Ip Address or DNS name of the AWS endpoint
	// Could stay empty, in this case driver will pick it up from system.client_routes table
	ConnectionAddr string
}

func (e ClientRoutesEndpoint) Validate() error {
	if e.ConnectionID == "" {
		return errors.New("missing ConnectionID")
	}
	return nil
}

type ClientRoutesEndpointList []ClientRoutesEndpoint

func (l *ClientRoutesEndpointList) GetAllConnectionIDs() []string {
	var ids []string
	for _, endpoint := range *l {
		ids = append(ids, endpoint.ConnectionID)
	}
	return ids
}

func (l *ClientRoutesEndpointList) GetConnectionAddr(connectionID string) string {
	for _, endpoint := range *l {
		if endpoint.ConnectionID == connectionID {
			return endpoint.ConnectionAddr
		}
	}
	return ""
}

func (l *ClientRoutesEndpointList) Validate() error {
	for id, endpoint := range *l {
		if err := endpoint.Validate(); err != nil {
			return fmt.Errorf("endpoint #%d is invalid: %w", id, err)
		}
	}
	return nil
}

type ClientRoutesConfig struct {
	TableName                    string
	Endpoints                    ClientRoutesEndpointList
	MaxResolverConcurrency       int
	ResolveHealthyEndpointPeriod time.Duration
	BlockUnknownEndpoints        bool
	ResolverCacheDuration        time.Duration
}

func (cfg *ClientRoutesConfig) Validate() error {
	if cfg == nil {
		return nil
	}
	if len(cfg.Endpoints) == 0 {
		return errors.New("no endpoints specified")
	}

	if err := cfg.Endpoints.Validate(); err != nil {
		return fmt.Errorf("failed to validate endpoints: %w", err)
	}
	if cfg.ResolveHealthyEndpointPeriod < 0 {
		return errors.New("resolve healthy endpoint period must be >= 0")
	}
	if cfg.MaxResolverConcurrency <= 0 {
		return errors.New("max resolver concurrency must be > 0")
	}
	return nil
}

type UnresolvedClientRoute struct {
	ConnectionID  string
	HostID        string
	Address       string
	CQLPort       uint16
	SecureCQLPort uint16
}

// Similar returns true if both records targets same host and connection id
func (r UnresolvedClientRoute) Similar(o UnresolvedClientRoute) bool {
	return r.ConnectionID == o.ConnectionID && r.HostID == o.HostID
}

// Equal returns true if both records are exactly the same
func (r UnresolvedClientRoute) Equal(o UnresolvedClientRoute) bool {
	return r == o
}

func (r UnresolvedClientRoute) String() string {
	return fmt.Sprintf(
		"UnresolvedClientRoute{ConnectionID=%s, HostID=%s, Address=%s, CQLPort=%d, SecureCQLPort=%d}",
		r.ConnectionID,
		r.HostID,
		r.Address,
		r.CQLPort,
		r.SecureCQLPort,
	)
}

type UnresolvedClientRouteList []UnresolvedClientRoute

func (l *UnresolvedClientRouteList) Len() int {
	return len(*l)
}

type ResolvedClientRoute struct {
	updateTime time.Time
	UnresolvedClientRoute
	allKnownIPs   []net.IP
	currentIP     net.IP
	forcedResolve bool
}

func (r ResolvedClientRoute) String() string {
	var ip string
	if r.currentIP == nil {
		ip = "<nil>"
	} else {
		ip = r.currentIP.String()
	}

	return fmt.Sprintf(
		"ResolvedClientRoute{ConnectionID=%s, HostID=%s, Address=%s, CQLPort=%d, SecureCQLPort=%d, CurrentIP=%s}",
		r.ConnectionID,
		r.HostID,
		r.Address,
		r.CQLPort,
		r.SecureCQLPort,
		ip,
	)
}

func (r ResolvedClientRoute) Clone() ResolvedClientRoute {
	res := r
	if res.allKnownIPs != nil {
		res.allKnownIPs = make([]net.IP, 0, len(r.allKnownIPs))
		for _, ip := range r.allKnownIPs {
			res.allKnownIPs = append(res.allKnownIPs, slices.Clone(ip))
		}
	}
	if len(res.currentIP) != 0 {
		copy(res.currentIP, r.currentIP)
		res.currentIP = slices.Clone(res.currentIP)
	}
	return res
}

// Newer returns true if o is newer than r
func (r ResolvedClientRoute) Newer(o ResolvedClientRoute) bool {
	if len(r.currentIP) == 0 && len(o.currentIP) != 0 {
		return true
	}
	if len(r.allKnownIPs) == 0 && len(o.allKnownIPs) != 0 {
		return true
	}
	return r.updateTime.Compare(o.updateTime) == -1
}

// Similar returns true if both records targets same host and connection id
func (r ResolvedClientRoute) Similar(o ResolvedClientRoute) bool {
	return r.ConnectionID == o.ConnectionID && r.HostID == o.HostID
}

func (r ResolvedClientRoute) NeedsUpdate() bool {
	return r.currentIP == nil || len(r.allKnownIPs) == 0 || r.forcedResolve
}

func (r ResolvedClientRoute) GetCQLPort() uint16 {
	if r.SecureCQLPort != 0 {
		return r.SecureCQLPort
	}
	return r.CQLPort
}

type ResolvedClientRouteList []ResolvedClientRoute

func (l *ResolvedClientRouteList) Len() int {
	return len(*l)
}

func (l *ResolvedClientRouteList) MergeWithUnresolved(unresolved UnresolvedClientRouteList) {
	for _, unres := range unresolved {
		found := false
		for id, res := range *l {
			if res.UnresolvedClientRoute.Similar(unres) {
				found = true
				if res.Equal(unres) {
					// Records are the same, no information has changed
					break
				}
				// Records are not the same, add unresolved record
				// It will be picked up by resolver on very next iteration
				(*l)[id] = ResolvedClientRoute{
					UnresolvedClientRoute: unres,
					forcedResolve:         true,
				}
				break
			}
		}
		if !found {
			*l = append(*l, ResolvedClientRoute{
				UnresolvedClientRoute: unres,
				forcedResolve:         true,
			})
		}
	}
}

func (l *ResolvedClientRouteList) MergeWithResolved(o *ResolvedClientRouteList) {
	for id, rec := range *l {
		for _, otherRec := range *o {
			if rec.Similar(otherRec) {
				if rec.Newer(otherRec) {
					(*l)[id] = otherRec
				}
				break
			}
		}
	}

	for _, otherRec := range *o {
		if !slices.ContainsFunc(*l, otherRec.Similar) {
			*l = append(*l, otherRec)
		}
	}
}

func (l *ResolvedClientRouteList) UpdateIfNewer(route ResolvedClientRoute) bool {
	for id, r := range *l {
		if r.Similar(route) {
			if !r.Newer(route) {
				return false
			}
			(*l)[id] = route
			return true
		}
	}
	*l = append(*l, route)
	return true
}

func (l *ResolvedClientRouteList) FindByHostID(hostID string) *ResolvedClientRoute {
	for i := range *l {
		if (*l)[i].HostID == hostID {
			return &(*l)[i]
		}
	}
	return nil
}

func (l *ResolvedClientRouteList) Clone() ResolvedClientRouteList {
	if len(*l) == 0 {
		return make(ResolvedClientRouteList, 0)
	}
	cpy := make(ResolvedClientRouteList, len(*l))
	copy(cpy, *l)
	return cpy
}

type ResolvedEndpoint struct {
	updateTime    time.Time
	connectionID  string
	dc            string
	rack          string
	address       string
	allKnown      []net.IP
	currentIP     net.IP
	forcedResolve bool
}

type ClientRoutesResolver interface {
	Resolve(endpoint ResolvedClientRoute) ([]net.IP, net.IP, error)
}

type resolvedCacheRecord struct {
	lastTimeResolved time.Time
	lastResult       []net.IP
}

func (r resolvedCacheRecord) WasResolvedLessThan(cachingTime time.Duration) bool {
	return time.Now().UTC().Sub(r.lastTimeResolved) < cachingTime
}

// simpleClientRoutesResolver resolves endpoints using the provided lookup function while enforcing
// a minimal period between successive resolutions of the same address.
type simpleClientRoutesResolver struct {
	resolver    DNSResolver
	cache       map[string]resolvedCacheRecord
	cachingTime time.Duration
	mu          sync.RWMutex
}

func newSimpleClientRoutesResolver(cachingTime time.Duration, resolver DNSResolver) *simpleClientRoutesResolver {
	if resolver == nil {
		resolver = defaultDnsResolver
	}
	return &simpleClientRoutesResolver{
		resolver:    resolver,
		cachingTime: cachingTime,
		cache:       make(map[string]resolvedCacheRecord),
	}
}

func (r *simpleClientRoutesResolver) Resolve(endpoint ResolvedClientRoute) (allKnown []net.IP, current net.IP, err error) {
	r.mu.RLock()
	cache, ok := r.cache[endpoint.Address]
	r.mu.RUnlock()
	if ok && cache.WasResolvedLessThan(r.cachingTime) {
		allKnown = cache.lastResult
	}

	if len(allKnown) == 0 {
		allKnown, err = r.resolver.LookupIP(endpoint.Address)
		if err != nil {
			return endpoint.allKnownIPs, endpoint.currentIP, err
		}
		if len(allKnown) == 0 {
			return endpoint.allKnownIPs, endpoint.currentIP, fmt.Errorf("no addresses returned for %s", endpoint.Address)
		}
	}

	for _, addr := range allKnown {
		if endpoint.currentIP != nil && endpoint.currentIP.Equal(addr) {
			current = addr
			break
		}
	}
	if current == nil {
		current = allKnown[0]
	}

	r.mu.Lock()
	r.cache[endpoint.Address] = resolvedCacheRecord{
		lastTimeResolved: time.Now().UTC(),
		lastResult:       allKnown,
	}
	r.mu.Unlock()
	return allKnown, current, nil
}

type ClientRoutesHandler struct {
	log               StdLogger
	c                 controlConnection
	resolver          ClientRoutesResolver
	sub               *eventbus.Subscriber[events.Event]
	resolvedEndpoints atomic.Pointer[ResolvedClientRouteList]
	updateTasks       chan updateTask
	closeChan         chan struct{}
	cfg               ClientRoutesConfig
	pickTLSPorts      bool
	initialized       bool
}

var _ AddressTranslatorV2 = (*ClientRoutesHandler)(nil)

// Translate implements old AddressTranslator interface
// should not be uses since driver prefer AddressTranslatorV2 API if it is implemented
func (p *ClientRoutesHandler) Translate(addr net.IP, port int) (net.IP, int) {
	panic("should never be called")
}

func pickProperPort(pickTLSPorts bool, rec *ResolvedClientRoute) uint16 {
	if pickTLSPorts {
		return rec.SecureCQLPort
	}
	return rec.CQLPort
}

// TranslateWithHost implements AddressTranslatorV2 interface
func (p *ClientRoutesHandler) TranslateHost(host AddressTranslatorHostInfo, addr AddressPort) (AddressPort, error) {
	hostID := host.HostID()
	if hostID == "" {
		return addr, nil
	}

	current := p.resolvedEndpoints.Load()
	rec := current.FindByHostID(hostID)
	if rec == nil {
		return addr, fmt.Errorf("no address found for host %s", hostID)
	}

	if rec.currentIP != nil {
		port := pickProperPort(p.pickTLSPorts, rec)
		if port == 0 {
			return addr, fmt.Errorf("record %s/%s has target port empty", rec.HostID, rec.ConnectionID)
		}
		return AddressPort{
			Address: rec.currentIP,
			Port:    port,
		}, nil
	}

	all, currentIP, err := p.resolver.Resolve(*rec)
	if err != nil {
		return addr, fmt.Errorf("failed to resolve DNS resolver for host %s: %v", hostID, err)
	}
	rec.allKnownIPs = all
	rec.currentIP = currentIP

	for {
		updated := current.Clone()
		if updated.UpdateIfNewer(*rec) {
			if p.resolvedEndpoints.CompareAndSwap(current, &updated) {
				port := pickProperPort(p.pickTLSPorts, rec)
				if port == 0 {
					return addr, fmt.Errorf("record %s/%s has target port empty", rec.HostID, rec.ConnectionID)
				}
				return AddressPort{
					Address: rec.currentIP,
					Port:    port,
				}, nil
			}
			continue
		}

		rec = current.FindByHostID(hostID)
		if rec == nil {
			return addr, fmt.Errorf("no address found for host %s", hostID)
		}
		port := pickProperPort(p.pickTLSPorts, rec)
		if port == 0 {
			return addr, fmt.Errorf("record %s/%s has target port empty", rec.HostID, rec.ConnectionID)
		}
		return AddressPort{
			Address: rec.currentIP,
			Port:    port,
		}, nil
	}
}

var never = time.Unix(1<<63-1, 0)

type updateTask struct {
	result        chan error
	connectionIDs []string
	hostIDs       []string
}

func (p *ClientRoutesHandler) Initialize(s *Session) error {
	if p.initialized {
		return errors.New("already initialized")
	}
	connectionIDs := make([]string, 0, len(p.cfg.Endpoints))
	for _, ep := range p.cfg.Endpoints {
		if ep.ConnectionID != "" {
			connectionIDs = append(connectionIDs, ep.ConnectionID)
		}
	}
	p.c = s.control
	p.sub = s.eventBus.Subscribe("port-mux", 1024, func(event events.Event) bool {
		switch event.Type() {
		case events.SessionEventTypeControlConnectionRecreated, events.ClusterEventTypeClientRoutesChanged:
			return true
		default:
			return false
		}
	})
	p.startUpdateWorker()
	p.startReadingEvents()
	err := p.updateHostPortMappingSync(connectionIDs, nil)
	if err != nil {
		p.log.Printf("error updating host ports: %v\n", err)
	}
	return nil
}

func (p *ClientRoutesHandler) Stop() {
	if p.updateTasks != nil {
		close(p.updateTasks)
	}
	if p.closeChan != nil {
		close(p.closeChan)
	}
	if p.sub != nil {
		p.sub.Stop()
	}
}

// resolveAndUpdateInPlace updates provided list of resolved endpoint in place
// If it can't resolve it keeps old record as is.
// Logic to pick a single address from all available addresses is delegated to ClientRoutesResolver at p.endpointResolver
// It does not resolve everything, it picks endpoints that are:
// 1. Marked via forcedResolve=true,
// 2. Have not resolved previously and have no ip address information
// 3. Was resolved more than cfg.ResolveHealthyEndpointPeriod ago.
func (p *ClientRoutesHandler) resolveAndUpdateInPlace(records ResolvedClientRouteList) error {
	if len(records) == 0 {
		return nil
	}

	errs := make([]error, len(records))
	tasks := make(chan int, len(records))

	var cutoffTimeForHealthy time.Time
	if p.cfg.ResolveHealthyEndpointPeriod == 0 {
		cutoffTimeForHealthy = never
	} else {
		cutoffTimeForHealthy = time.Now().UTC().Add(-p.cfg.ResolveHealthyEndpointPeriod)
	}

	scheduled := false
	for id, endpoint := range records {
		if endpoint.currentIP == nil || len(endpoint.allKnownIPs) == 0 || endpoint.forcedResolve {
			scheduled = true
			tasks <- id
		} else if endpoint.updateTime.Before(cutoffTimeForHealthy) {
			scheduled = true
			tasks <- id
		}
	}

	if !scheduled {
		return nil
	}

	var wg sync.WaitGroup
	for i := 0; i < p.cfg.MaxResolverConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for id := range tasks {
				all, currentIP, err := p.resolver.Resolve(records[id])
				records[id].updateTime = time.Now().UTC()
				if err != nil {
					errs[id] = fmt.Errorf("resolve %s failed: %w", records[id].currentIP, err)
					continue
				} else if len(all) == 0 {
					errs[id] = fmt.Errorf("resolve %s: no addresses returned", records[id].currentIP)
				} else if currentIP == nil {
					errs[id] = fmt.Errorf("resolve %s: no current addres has been set, should not happen, please report a bug", records[id].currentIP)
				} else {
					// Reset forcedResolve is it was resolved successfully
					records[id].forcedResolve = false
				}
				records[id].allKnownIPs = all
				records[id].currentIP = currentIP
			}
		}()
	}

	close(tasks)
	wg.Wait()

	return errors.Join(errs...)
}

func (p *ClientRoutesHandler) updateHostPortMappingAsync(connectionIDs []string, hostIDs []string) {
	p.updateTasks <- updateTask{
		connectionIDs: connectionIDs,
		hostIDs:       hostIDs,
	}
}

func (p *ClientRoutesHandler) updateHostPortMappingSync(connectionIDs []string, hostIDs []string) error {
	result := make(chan error, 1)
	p.updateTasks <- updateTask{
		connectionIDs: connectionIDs,
		hostIDs:       hostIDs,
		result:        result,
	}
	return <-result
}

func (p *ClientRoutesHandler) startReadingEvents() {
	var connectionIDs []string
	if p.cfg.BlockUnknownEndpoints {
		connectionIDs = p.cfg.Endpoints.GetAllConnectionIDs()
	}

	go func() {
		for event := range p.sub.Events() {
			switch evt := event.(type) {
			case *events.ClientRoutesChangedEvent:
				if debug.Enabled {
					if len(evt.ConnectionIDs) == 0 {
						p.log.Printf("got CLIENT_ROUTES_CHANGE event with no connection IDs")
						continue
					}
					if len(evt.HostIDs) == 0 {
						p.log.Printf("got CLIENT_ROUTES_CHANGE event with no host IDs")
						continue
					}
				}
				var newConnectionIDs []string
				if p.cfg.BlockUnknownEndpoints {
					for _, connectionID := range evt.ConnectionIDs {
						if connectionID == "" {
							continue
						}
						if slices.ContainsFunc(p.cfg.Endpoints, func(ep ClientRoutesEndpoint) bool {
							return ep.ConnectionID == connectionID
						}) {
							newConnectionIDs = append(newConnectionIDs, connectionID)
						}
					}
					if len(newConnectionIDs) != 0 {
						p.updateHostPortMappingAsync(newConnectionIDs, evt.HostIDs)
					}
				} else {
					p.updateHostPortMappingAsync(newConnectionIDs, evt.HostIDs)
				}
			case *events.ControlConnectionRecreatedEvent:
				p.updateHostPortMappingAsync(connectionIDs, nil)
			}
		}
	}()
}

func (p *ClientRoutesHandler) startUpdateWorker() {
	go func() {
		for task := range p.updateTasks {
			err := p.updateHostPortMapping(task.connectionIDs, task.hostIDs)
			if err != nil {
				if debug.Enabled {
					p.log.Printf("failed to update host port mapping: %v", err)
				}
			}
			if task.result != nil {
				task.result <- err
				close(task.result)
			}
		}
	}()
}

func (p *ClientRoutesHandler) updateHostPortMapping(connectionIDs []string, hostIDs []string) error {
	unresolved, err := getHostPortMappingFromCluster(p.c, p.cfg.TableName, connectionIDs, hostIDs)
	if err != nil {
		return err
	}
	current := p.resolvedEndpoints.Load()
	updated := slices.Clone(*current)
	updated.MergeWithUnresolved(unresolved)
	err = p.resolveAndUpdateInPlace(updated)
	if err != nil {
		p.log.Printf("failed to resolve endpoints: %v", err)
		// Despite an error it is better to save results, it should not corrupt existing and resolved records
	}

	// Try to update until it successes
	// 10 times is more than enough, if it fails
	for range 10 {
		if p.resolvedEndpoints.CompareAndSwap(current, &updated) {
			return nil
		}

		current = p.resolvedEndpoints.Load()
		updated.MergeWithResolved(current)
	}
	p.log.Printf("failed to update host port mapping due to collisions")

	return nil
}

func NewClientRoutesAddressTranslator(
	cfg ClientRoutesConfig,
	resolver DNSResolver,
	pickTLSPorts bool,
	log StdLogger,
) *ClientRoutesHandler {
	res := &ClientRoutesHandler{
		cfg:          cfg,
		log:          log,
		pickTLSPorts: pickTLSPorts,
		closeChan:    make(chan struct{}),
		updateTasks:  make(chan updateTask, 1024),
		resolver:     newSimpleClientRoutesResolver(cfg.ResolverCacheDuration, resolver),
	}
	res.resolvedEndpoints.Store(&ResolvedClientRouteList{})
	return res
}

var _ AddressTranslator = &ClientRoutesHandler{}

func getHostPortMappingFromCluster(c controlConnection, table string, connectionIDs []string, hostIDs []string) (UnresolvedClientRouteList, error) {
	var res UnresolvedClientRouteList

	stmt := []string{fmt.Sprintf("select connection_id, host_id, address, port, tls_port from %s", table)}
	var bounds []interface{}
	if len(connectionIDs) != 0 {
		var inClause []string
		for _, connectionID := range connectionIDs {
			bounds = append(bounds, connectionID)
			inClause = append(inClause, "?")
		}
		if len(stmt) == 1 {
			stmt = append(stmt, "where")
		}
		stmt = append(stmt, fmt.Sprintf("connection_id in (%s)", strings.Join(inClause, ",")))
	}

	if len(hostIDs) != 0 {
		var inClause []string
		for _, hostID := range hostIDs {
			bounds = append(bounds, hostID)
			inClause = append(inClause, "?")
		}
		if len(stmt) == 1 {
			stmt = append(stmt, "where")
		} else {
			stmt = append(stmt, "and")
		}
		stmt = append(stmt, fmt.Sprintf("host_id in (%s)", strings.Join(inClause, ",")))
	}

	isFullScan := len(hostIDs) == 0 || len(connectionIDs) == 0
	if isFullScan {
		stmt = append(stmt, "allow filtering")
	}

	iter := c.query(strings.Join(stmt, " "), bounds...)
	var rec UnresolvedClientRoute
	for iter.Scan(&rec.ConnectionID, &rec.HostID, &rec.Address, &rec.CQLPort, &rec.SecureCQLPort) {
		res = append(res, rec)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("error reading %s table: %v", table, err)
	}
	return res, nil
}
