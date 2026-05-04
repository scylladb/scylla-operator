//go:build !race

package gocql

func waitCallReqDone(call *callReq, where string) {
	call.done.Wait()
}
