// Copyright (C) 2017 ScyllaDB

package scyllaclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/scylladb/scylla-manager/v3/pkg/rclone/rcserver"
	"github.com/scylladb/scylla-manager/v3/pkg/util/pointer"
	agentClient "github.com/scylladb/scylla-manager/v3/swagger/gen/agent/client"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/agent/client/operations"
	"github.com/scylladb/scylla-manager/v3/swagger/gen/agent/models"
)

// RcloneSetBandwidthLimit sets bandwidth limit of all the current and future
// transfers performed under current client session.
// Limit is expressed in MiB per second.
// To turn off limitation set it to 0.
func (c *Client) RcloneSetBandwidthLimit(ctx context.Context, host string, limit int) error {
	p := operations.CoreBwlimitParams{
		Context:       forceHost(ctx, host),
		BandwidthRate: &models.Bandwidth{Rate: fmt.Sprintf("%dM", limit)},
	}
	_, err := c.agentOps.CoreBwlimit(&p) // nolint: errcheck
	return err
}

// RcloneJobStop stops running job.
func (c *Client) RcloneJobStop(ctx context.Context, host string, jobID int64) error {
	p := operations.JobStopParams{
		Context: forceHost(ctx, host),
		Jobid:   &models.Jobid{Jobid: jobID},
	}
	_, err := c.agentOps.JobStop(&p) // nolint: errcheck
	return err
}

// RcloneJobInfo groups stats for job, running, and completed transfers.
type RcloneJobInfo = models.JobInfo

// RcloneJobProgress aggregates job progress stats.
type RcloneJobProgress = models.JobProgress

// RcloneTransfer represents a single file transfer in RcloneJobProgress Transferred.
type RcloneTransfer = models.Transfer

// GlobalProgressID represents empty job id.
// Use this value to return global stats by job info.
var GlobalProgressID int64

// RcloneJobInfo returns job stats, and transfers info about running stats and
// completed transfers.
// If waitSeconds > 0 then long polling will be used with number of seconds.
func (c *Client) RcloneJobInfo(ctx context.Context, host string, jobID int64, waitSeconds int) (*RcloneJobInfo, error) {
	ctx = customTimeout(forceHost(ctx, host), c.longPollingTimeout(waitSeconds))

	p := operations.JobInfoParams{
		Context: ctx,
		Jobinfo: &models.JobInfoParams{
			Jobid: jobID,
			Wait:  int64(waitSeconds),
		},
	}
	resp, err := c.agentOps.JobInfo(&p)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// RcloneJobProgress returns aggregated stats for the job along with its status.
func (c *Client) RcloneJobProgress(ctx context.Context, host string, jobID int64, waitSeconds int) (*RcloneJobProgress, error) {
	ctx = customTimeout(forceHost(ctx, host), c.longPollingTimeout(waitSeconds))

	p := operations.JobProgressParams{
		Context: ctx,
		Jobinfo: &models.JobInfoParams{
			Jobid: jobID,
			Wait:  int64(waitSeconds),
		},
	}
	resp, err := c.agentOps.JobProgress(&p)
	if StatusCodeOf(err) == http.StatusNotFound {
		// If we got 404 then return empty progress with not found status.
		return &RcloneJobProgress{
			Status: string(rcserver.JobNotFound),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// RcloneJobStatus returns status of the job.
// There is one running state and three completed states error, not_found, and
// success.
type RcloneJobStatus string

const (
	// JobError signals that job completed with error.
	JobError RcloneJobStatus = "error"
	// JobSuccess signals that job completed with success.
	JobSuccess RcloneJobStatus = "success"
	// JobRunning signals that job is still running.
	JobRunning RcloneJobStatus = "running"
	// JobNotFound signals that job is no longer available.
	JobNotFound RcloneJobStatus = "not_found"
)

// WorthWaitingForJob checks if rclone job can (or already did) succeed.
func WorthWaitingForJob(status string) bool {
	return status == string(JobSuccess) || status == string(JobRunning)
}

// RcloneDeleteJobStats deletes job stats group.
func (c *Client) RcloneDeleteJobStats(ctx context.Context, host string, jobID int64) error {
	p := operations.CoreStatsDeleteParams{
		Context: forceHost(ctx, host),
		StatsParams: &models.StatsParams{
			Group: rcloneDefaultGroup(jobID),
		},
	}
	_, err := c.agentOps.CoreStatsDelete(&p) // nolint: errcheck
	return err
}

// RcloneResetStats resets stats.
func (c *Client) RcloneResetStats(ctx context.Context, host string) error {
	p := operations.CoreStatsResetParams{
		Context: forceHost(ctx, host),
	}
	_, err := c.agentOps.CoreStatsReset(&p) // nolint: errcheck
	return err
}

// RcloneDefaultGroup returns default group name based on job id.
func rcloneDefaultGroup(jobID int64) string {
	return fmt.Sprintf("job/%d", jobID)
}

// RcloneMoveFile moves file from srcRemotePath to dstRemotePath.
// Remotes need to be registered with the server first.
// Remote path format is "name:bucket/path".
// Both dstRemotePath and srRemotePath must point to a file.
func (c *Client) RcloneMoveFile(ctx context.Context, host, dstRemotePath, srcRemotePath string) error {
	return c.rcloneMoveOrCopyFile(ctx, host, dstRemotePath, srcRemotePath, true)
}

// RcloneCopyFile copies file from srcRemotePath to dstRemotePath.
// Remotes need to be registered with the server first.
// Remote path format is "name:bucket/path".
// Both dstRemotePath and srRemotePath must point to a file.
func (c *Client) RcloneCopyFile(ctx context.Context, host, dstRemotePath, srcRemotePath string) error {
	return c.rcloneMoveOrCopyFile(ctx, host, dstRemotePath, srcRemotePath, false)
}

func (c *Client) rcloneMoveOrCopyFile(ctx context.Context, host, dstRemotePath, srcRemotePath string, doMove bool) error {
	dstFs, dstRemote, err := rcloneSplitRemotePath(dstRemotePath)
	if err != nil {
		return err
	}
	srcFs, srcRemote, err := rcloneSplitRemotePath(srcRemotePath)
	if err != nil {
		return err
	}

	if doMove {
		p := operations.OperationsMovefileParams{
			Context: forceHost(ctx, host),
			Options: &models.MoveOrCopyFileOptions{
				DstFs:     dstFs,
				DstRemote: dstRemote,
				SrcFs:     srcFs,
				SrcRemote: srcRemote,
			},
		}
		_, err = c.agentOps.OperationsMovefile(&p)
	} else {
		p := operations.OperationsCopyfileParams{
			Context: forceHost(ctx, host),
			Options: &models.MoveOrCopyFileOptions{
				DstFs:     dstFs,
				DstRemote: dstRemote,
				SrcFs:     srcFs,
				SrcRemote: srcRemote,
			},
		}
		_, err = c.agentOps.OperationsCopyfile(&p)
	}

	return err
}

// RcloneMoveDir moves contents of the directory pointed by srcRemotePath to
// the directory pointed by dstRemotePath.
// Remotes need to be registered with the server first.
// Returns ID of the asynchronous job.
// Remote path format is "name:bucket/path".
// If specified, a suffix will be added to otherwise overwritten or deleted files.
func (c *Client) RcloneMoveDir(ctx context.Context, host, dstRemotePath, srcRemotePath, suffix string) (int64, error) {
	return c.rcloneMoveOrCopyDir(ctx, host, dstRemotePath, srcRemotePath, true, suffix)
}

// RcloneCopyDir copies contents of the directory pointed by srcRemotePath to
// the directory pointed by dstRemotePath.
// Remotes need to be registered with the server first.
// Returns ID of the asynchronous job.
// Remote path format is "name:bucket/path".
// If specified, a suffix will be added to otherwise overwritten or deleted files.
func (c *Client) RcloneCopyDir(ctx context.Context, host, dstRemotePath, srcRemotePath, suffix string) (int64, error) {
	return c.rcloneMoveOrCopyDir(ctx, host, dstRemotePath, srcRemotePath, false, suffix)
}

func (c *Client) rcloneMoveOrCopyDir(ctx context.Context, host, dstRemotePath, srcRemotePath string, doMove bool, suffix string) (int64, error) {
	dstFs, dstRemote, err := rcloneSplitRemotePath(dstRemotePath)
	if err != nil {
		return 0, err
	}
	srcFs, srcRemote, err := rcloneSplitRemotePath(srcRemotePath)
	if err != nil {
		return 0, err
	}
	m := models.MoveOrCopyFileOptions{
		DstFs:     dstFs,
		DstRemote: dstRemote,
		SrcFs:     srcFs,
		SrcRemote: srcRemote,
		Suffix:    suffix,
	}

	var jobID int64
	if doMove {
		p := operations.SyncMoveDirParams{
			Context: forceHost(ctx, host),
			Options: &m,
			Async:   true,
		}
		resp, err := c.agentOps.SyncMoveDir(&p)
		if err != nil {
			return 0, err
		}
		jobID = resp.Payload.Jobid
	} else {
		p := operations.SyncCopyDirParams{
			Context: forceHost(ctx, host),
			Options: &m,
			Async:   true,
		}
		resp, err := c.agentOps.SyncCopyDir(&p)
		if err != nil {
			return 0, err
		}
		jobID = resp.Payload.Jobid
	}

	return jobID, nil
}

// RcloneCopyPaths copies paths from srcRemoteDir/path to dstRemoteDir/path.
// Remotes need to be registered with the server first.
// Remote path format is "name:bucket/path".
// Both dstRemoteDir and srRemoteDir must point to a directory.
func (c *Client) RcloneCopyPaths(ctx context.Context, host, dstRemoteDir, srcRemoteDir string, paths []string) (int64, error) {
	dstFs, dstRemote, err := rcloneSplitRemotePath(dstRemoteDir)
	if err != nil {
		return 0, err
	}
	srcFs, srcRemote, err := rcloneSplitRemotePath(srcRemoteDir)
	if err != nil {
		return 0, err
	}

	p := operations.SyncCopyPathsParams{
		Context: forceHost(ctx, host),
		Options: &models.CopyPathsOptions{
			DstFs:     dstFs,
			DstRemote: dstRemote,
			SrcFs:     srcFs,
			SrcRemote: srcRemote,
			Paths:     paths,
		},
		Async: true,
	}

	resp, err := c.agentOps.SyncCopyPaths(&p)
	if err != nil {
		return 0, err
	}

	return resp.Payload.Jobid, nil
}

// RcloneDeleteDir removes a directory or container and all of its contents
// from the remote.
// Remote path format is "name:bucket/path".
func (c *Client) RcloneDeleteDir(ctx context.Context, host, remotePath string) error {
	fs, remote, err := rcloneSplitRemotePath(remotePath)
	if err != nil {
		return err
	}
	p := operations.OperationsPurgeParams{
		Context: forceHost(ctx, host),
		RemotePath: &models.RemotePath{
			Fs:     fs,
			Remote: remote,
		},
		Async: false,
	}
	_, err = c.agentOps.OperationsPurge(&p) // nolint: errcheck
	return err
}

// RcloneDeleteFile removes the single file pointed to by remotePath
// Remote path format is "name:bucket/path".
func (c *Client) RcloneDeleteFile(ctx context.Context, host, remotePath string) error {
	fs, remote, err := rcloneSplitRemotePath(remotePath)
	if err != nil {
		return err
	}
	p := operations.OperationsDeletefileParams{
		Context: forceHost(ctx, host),
		RemotePath: &models.RemotePath{
			Fs:     fs,
			Remote: remote,
		},
		Async: false,
	}
	_, err = c.agentOps.OperationsDeletefile(&p) // nolint: errcheck
	return err
}

// RcloneDeletePathsInBatches divides paths into batches and calls RcloneDeletePaths for each of them.
// This method should be used when deleting large number of files to improve granularity and timeout handling.
func (c *Client) RcloneDeletePathsInBatches(ctx context.Context, host, remoteDir string, paths []string, batchSize int) (int64, error) {
	toDelete := append([]string{}, paths...)
	var deleted int64
	for len(toDelete) != 0 {
		size := min(len(toDelete), batchSize)
		batch := toDelete[:size]
		toDelete = toDelete[size:]

		cnt, err := c.RcloneDeletePaths(ctx, host, remoteDir, batch)
		if err != nil {
			return deleted, errors.Wrap(err, "delete paths")
		}
		deleted += cnt
	}
	return deleted, nil
}

// RcloneDeletePaths deletes paths from remoteDir/path.
// It does not return error when some paths are not present on the remote,
// and it returns the amount of actually deleted files.
// Paths cannot be empty.
// RemoteDir:
//   - needs to be registered with the server first
//   - has "name:bucket/path" format
//   - must point to a directory
func (c *Client) RcloneDeletePaths(ctx context.Context, host, remoteDir string, paths []string) (int64, error) {
	fs, remote, err := rcloneSplitRemotePath(remoteDir)
	if err != nil {
		return 0, err
	}
	p := operations.OperationsDeletepathsParams{
		Context: customTimeout(forceHost(ctx, host), 15*time.Minute),
		Options: &models.DeletePathsOptions{
			Fs:     fs,
			Remote: remote,
			Paths:  paths,
		},
	}
	res, err := c.agentOps.OperationsDeletepaths(&p)
	if err != nil {
		return 0, err
	}
	return res.Payload.Deletes, nil
}

// RcloneDiskUsage get disk space usage.
// Remote path format is "name:bucket/path".
func (c *Client) RcloneDiskUsage(ctx context.Context, host, remotePath string) (*models.FileSystemDetails, error) {
	p := operations.OperationsAboutParams{
		Context: forceHost(ctx, host),
		RemotePath: &models.RemotePath{
			Fs: remotePath,
		},
	}
	resp, err := c.agentOps.OperationsAbout(&p)
	if err != nil {
		return nil, err
	}
	return resp.Payload, nil
}

// RcloneFileInfo returns basic remote object information.
func (c *Client) RcloneFileInfo(ctx context.Context, host, remotePath string) (*models.FileInfo, error) {
	fs, remote, err := rcloneSplitRemotePath(remotePath)
	if err != nil {
		return nil, err
	}

	p := operations.OperationsFileInfoParams{
		Context: forceHost(ctx, host),
		RemotePath: &models.RemotePath{
			Fs:     fs,
			Remote: remote,
		},
	}
	resp, err := c.agentOps.OperationsFileInfo(&p)
	if err != nil {
		return nil, err
	}
	return resp.Payload, nil
}

// RcloneOpen streams remote file content. The stream is an HTTP body.
// Callers must close the body after use.
func (c *Client) RcloneOpen(ctx context.Context, host, remotePath string) (io.ReadCloser, error) {
	fs, remote, err := rcloneSplitRemotePath(remotePath)
	if err != nil {
		return nil, err
	}

	// Due to missing generator for Swagger 3.0, and poor implementation of 2.0
	// raw file download we are downloading manually.
	const urlPath = agentClient.DefaultBasePath + "/rclone/operations/cat"

	// Body
	b, err := (&models.RemotePath{
		Fs:     fs,
		Remote: remote,
	}).MarshalBinary()
	if err != nil {
		return nil, err
	}

	u := c.newURL(host, urlPath)
	req, err := http.NewRequestWithContext(forceHost(ctx, host), http.MethodPost, u.String(), bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do("OperationsCat", req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// RcloneCat returns a content of a remote path.
func (c *Client) RcloneCat(ctx context.Context, host, remotePath string) ([]byte, error) {
	r, err := c.RcloneOpen(ctx, host, remotePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// RcloneListDirOpts specifies options for RcloneListDir.
type RcloneListDirOpts struct {
	// Show only directories in the listing
	DirsOnly bool
	// Show only files in the listing
	FilesOnly bool
	// Recurse into the listing
	Recurse bool
	// Read the modification time.
	// For rclone backends declared as SlowModTime setting this to true will
	// send additional HEAD request for each listed file if the rclone
	// configuration option UseServerModTime is false.
	ShowModTime bool
	// Show only the newest versions of files in the listing (no snapshot tag suffix attached)
	NewestOnly bool
	// Show only older versions of files in the listing (snapshot tag suffix attached)
	VersionedOnly bool
}

func (opts *RcloneListDirOpts) asModelOpts() *models.ListOptionsOpt {
	if opts == nil {
		return &models.ListOptionsOpt{
			NoModTime:  true,
			NoMimeType: true,
		}
	}

	return &models.ListOptionsOpt{
		DirsOnly:   opts.DirsOnly,
		FilesOnly:  opts.FilesOnly,
		Recurse:    opts.Recurse,
		NoModTime:  !opts.ShowModTime,
		NoMimeType: true,
	}
}

// RcloneListDirItem represents a file in a listing with RcloneListDir.
type RcloneListDirItem = models.ListItem

// RcloneListDir returns contents of a directory specified by remotePath.
// The remotePath is given in the following format "provider:bucket/path".
// Resulting item path is relative to the remote path.
//
// LISTING REMOTE DIRECTORIES IS KNOWN TO BE EXTREMELY SLOW.
// DO NOT USE THIS FUNCTION FOR ANYTHING BUT A FLAT DIRECTORY LISTING.
// FOR OTHER NEEDS, OR WHEN THE NUMBER OF FILES CAN BE BIG, USE RcloneListDirIter.
//
// This function must execute in the standard Timeout (15s by default) and
// will be retried if failed.
func (c *Client) RcloneListDir(ctx context.Context, host, remotePath string, opts *RcloneListDirOpts) ([]*RcloneListDirItem, error) {
	p := operations.OperationsListParams{
		Context: forceHost(ctx, host),
		ListOpts: &models.ListOptions{
			Fs:            &remotePath,
			Remote:        pointer.StringPtr(""),
			Opt:           opts.asModelOpts(),
			NewestOnly:    opts != nil && opts.NewestOnly,
			VersionedOnly: opts != nil && opts.VersionedOnly,
		},
	}
	resp, err := c.agentOps.OperationsList(&p)
	if err != nil {
		return nil, err
	}

	return resp.Payload.List, nil
}

// RcloneListDirIter returns contents of a directory specified by remotePath.
// The remotePath is given in the following format "provider:bucket/path".
// Resulting item path is relative to the remote path.
func (c *Client) RcloneListDirIter(ctx context.Context, host, remotePath string, opts *RcloneListDirOpts, f func(item *RcloneListDirItem)) error {
	ctx = noTimeout(ctx)
	ctx = noRetry(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Due to OpenAPI limitations we manually construct and sent the request
	// object to stream process the response body.
	const urlPath = agentClient.DefaultBasePath + "/rclone/operations/list"

	listOpts := &models.ListOptions{
		Fs:            &remotePath,
		Remote:        pointer.StringPtr(""),
		Opt:           opts.asModelOpts(),
		NewestOnly:    opts != nil && opts.NewestOnly,
		VersionedOnly: opts != nil && opts.VersionedOnly,
	}
	b, err := listOpts.MarshalBinary()
	if err != nil {
		return err
	}

	u := c.newURL(host, urlPath)
	req, err := http.NewRequestWithContext(forceHost(ctx, host), http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.client.Do("OperationsList", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	// Skip tokens down to array opening
	expected := []string{"{", "list", "["}
	for i := range expected {
		tok, err := dec.Token()
		if err != nil {
			return errors.Wrap(err, "read token")
		}
		if fmt.Sprint(tok) != expected[i] {
			return errors.Errorf("json unexpected token %s expected %s", tok, expected[i])
		}
	}

	errCh := make(chan error)
	go func() {
		var v RcloneListDirItem
		for dec.More() {
			// Detect context cancellation
			if ctx.Err() != nil {
				errCh <- ctx.Err()
				return
			}

			// Read value
			v = RcloneListDirItem{}
			if err := dec.Decode(&v); err != nil {
				errCh <- err
				return
			}
			f(&v)

			errCh <- nil
		}
		close(errCh)
	}()

	// Rclone filters versioned files on its side.
	// Since the amount of versioned files is little (usually 0),
	// the timer won't be refreshed even though rclone is correctly iterating over
	// remote files. To solve that, we use the MaxTimeout instead of the usual ListTimeout (#3615).
	resetTimeout := c.config.ListTimeout
	if listOpts.VersionedOnly {
		resetTimeout = c.config.MaxTimeout
	}
	timer := time.NewTimer(resetTimeout)
	defer timer.Stop()

	for {
		select {
		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			if err != nil {
				return err
			}
			timer.Reset(resetTimeout)
		case <-timer.C:
			return errors.Errorf("rclone list dir timeout")
		}
	}
}

// RcloneCheckPermissions checks if location is available for listing, getting,
// creating, and deleting objects.
func (c *Client) RcloneCheckPermissions(ctx context.Context, host, remotePath string) error {
	p := operations.OperationsCheckPermissionsParams{
		Context: forceHost(ctx, host),
		RemotePath: &models.RemotePath{
			Fs:     remotePath,
			Remote: "",
		},
	}
	_, err := c.agentOps.OperationsCheckPermissions(&p)
	return err
}

// RclonePut uploads file with provided content under remotePath.
// WARNING: This API call doesn't compare checksums. It relies on sizes only. This call cannot be used for moving sstables.
func (c *Client) RclonePut(ctx context.Context, host, remotePath string, body *bytes.Buffer) error {
	fs, remote, err := rcloneSplitRemotePath(remotePath)
	if err != nil {
		return err
	}

	// Due to missing generator for Swagger 3.0, and poor implementation of 2.0 file upload
	// we are uploading manually.
	const urlPath = agentClient.DefaultBasePath + "/rclone/operations/put"

	u := c.newURL(host, urlPath)
	req, err := http.NewRequestWithContext(forceHost(ctx, host), http.MethodPost, u.String(), body)
	if err != nil {
		return err
	}

	q := req.URL.Query()
	q.Add("fs", fs)
	q.Add("remote", remote)
	req.URL.RawQuery = q.Encode()
	req.Header.Add("Content-Type", "application/octet-stream")

	resp, err := c.client.Do("OperationsPut", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err := io.CopyN(io.Discard, resp.Body, resp.ContentLength); err != nil {
		return err
	}
	return nil
}

// rcloneSplitRemotePath splits string path into file system and file path.
func rcloneSplitRemotePath(remotePath string) (fs, path string, err error) {
	parts := strings.Split(remotePath, ":")
	if len(parts) != 2 {
		err = errors.New("remote path without file system name")
		return
	}

	dirParts := strings.SplitN(parts[1], "/", 2)
	root := dirParts[0]
	fs = fmt.Sprintf("%s:%s", parts[0], root)
	if len(dirParts) > 1 {
		path = dirParts[1]
	}
	return
}
