package gocql

type DefaultWarningHandler struct {
	logger StdLogger
}

func DefaultWarningHandlerBuilder(session *Session) WarningHandler {
	return DefaultWarningHandler{
		logger: session.logger,
	}
}

func (d DefaultWarningHandler) HandleWarnings(qry ExecutableQuery, host *HostInfo, warnings []string) {
	if d.logger == nil {
		return
	}
	if host != nil && len(host.hostId) > 0 {
		d.logger.Printf("[%s] warnings: %v", host.hostId, warnings)
	} else {
		d.logger.Printf("Cluster warnings: %v", warnings)
	}
}

var _ WarningHandler = DefaultWarningHandler{}

func NoopWarningHandlerBuilder(session *Session) WarningHandler {
	return nil
}
