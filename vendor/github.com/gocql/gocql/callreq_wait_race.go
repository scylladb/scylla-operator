//go:build race

package gocql

import (
	"fmt"
	"time"
)

func waitCallReqDone(call *callReq, where string) {
	done := make(chan struct{})
	go func() {
		call.done.Wait()
		close(done)
	}()

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		panic(fmt.Sprintf("gocql: timed out waiting for exec cleanup in %s (stream=%d)", where, call.streamID))
	}
}
