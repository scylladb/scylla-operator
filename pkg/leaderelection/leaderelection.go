package leaderelection

import (
	"context"
	"fmt"
	"os"
	"time"

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

	lock, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		lockNamespace,
		lockName,
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		},
	)
	if err != nil {
		return fmt.Errorf("can't create resource lock: %w", err)
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
				if fErr != nil {
					// We are passing it on but just in case there would be a failure on the way we should log it.
					klog.Error(fErr)
				}
				// TODO: Handle the exit more gracefully, if possible, to also release the lock.
				//       At this point canceling the context prevents any new calls to kube apiserver as well.
				//       (We need to cancel the context so the program exits on setup errors.)
				leCtxCancel()
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
