package leaderelection

import (
	"context"
	"fmt"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

func Run(ctx context.Context, programName, lockName, lockNamespace string, client kubernetes.Interface, leaderelectionLeaseDuration, leaderelectionRenewDeadline, leaderelectionRetryPeriod time.Duration, f func(context.Context) error) error {
	criticalSectionTokenChan := make(chan struct{}, 1)

	// leCtx is special context to first cancel the business logic and then the LE can stop and release the lock
	leCtx, leCtxCancel := context.WithCancel(context.Background())
	defer leCtxCancel()
	go func() {
		<-ctx.Done()

		// Propagate cancel only if we are not in critical section
		criticalSectionTokenChan <- struct{}{} // Acquire token
		<-criticalSectionTokenChan             // release token

		leCtxCancel()
	}()

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id := hostname + "_" + string(uuid.NewUUID())
	klog.V(4).Infof("Leader election ID is %q", id)

	lock := &resourcelock.ConfigMapLock{
		ConfigMapMeta: metav1.ObjectMeta{
			Name:      lockName,
			Namespace: lockNamespace,
		},
		Client: client.CoreV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	var fErr error

	defer close(criticalSectionTokenChan)
	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   leaderelectionLeaseDuration,
		RenewDeadline:   leaderelectionRenewDeadline,
		RetryPeriod:     leaderelectionRetryPeriod,
		ReleaseOnCancel: true,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(localCtx context.Context) {
				mergedCtx, mergedCtxCancel := context.WithCancel(ctx)
				defer mergedCtxCancel()
				go func() {
					<-localCtx.Done()
					mergedCtxCancel()
				}()

				criticalSectionTokenChan <- struct{}{} // Acquire token
				fErr = f(mergedCtx)
				if err != nil {
					// We are passing it on but just in case there would be a failure on the way we should log it.
					klog.Error(err)
				}
				<-criticalSectionTokenChan // release token
			},
			OnStoppedLeading: func() {
				select {
				case criticalSectionTokenChan <- struct{}{}:
					// The critical section is already finished.
					klog.Info("Released leader election lock.")
				default:
					klog.Fatal("Leader election lost!")
				}
			},
		},
		Name: programName,
	})
	if err != nil {
		return fmt.Errorf("leaderelection: %w", err)
	}

	klog.Infof("Starting leader election")
	le.Run(leCtx)

	return fErr
}
