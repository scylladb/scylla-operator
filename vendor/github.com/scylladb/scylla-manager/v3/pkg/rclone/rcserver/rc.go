// Copyright (C) 2017 ScyllaDB

package rcserver

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/object"
	rcops "github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
	"github.com/rclone/rclone/fs/sync"
	"github.com/scylladb/scylla-manager/v3/pkg/rclone"
	"github.com/scylladb/scylla-manager/v3/pkg/rclone/operations"
	"github.com/scylladb/scylla-manager/v3/pkg/rclone/rcserver/internal"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"go.uber.org/multierr"
)

// rcJobInfo aggregates core, transferred, and job stats into a single call.
// If jobid parameter is provided but job is not found then nil is returned for
// all three aggregated stats.
// If jobid parameter is not provided then transferred and core stats are
// returned for all groups to allow access to global transfer stats.
func rcJobInfo(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	var (
		jobOut, statsOut, transOut map[string]interface{}
		jobErr, statsErr, transErr error
	)
	// Load Job status only if jobid is explicitly set.
	if jobid, err := in.GetInt64("jobid"); err == nil {
		wait, err := in.GetInt64("wait")
		if err != nil && !rc.IsErrParamNotFound(err) {
			jobErr = err
		} else if wait > 0 {
			jobErr = waitForJobFinish(ctx, jobid, wait)
		}
		if jobErr == nil {
			jobOut, jobErr = rcCalls.Get("job/status").Fn(ctx, in)
			in["group"] = fmt.Sprintf("job/%d", jobid)
		}
	}

	if jobErr == nil {
		statsOut, statsErr = rcCalls.Get("core/stats").Fn(ctx, in)
		transOut, transErr = rcCalls.Get("core/transferred").Fn(ctx, in)
	} else if errors.Is(jobErr, errJobNotFound) {
		jobErr = nil
		fs.Errorf(nil, "Job not found")
	}

	return rc.Params{
		"job":         jobOut,
		"stats":       statsOut,
		"transferred": transOut["transferred"],
	}, multierr.Combine(jobErr, statsErr, transErr)
}

func init() {
	rc.Add(rc.Call{
		Path:         "job/info",
		AuthRequired: true,
		Fn:           rcJobInfo,
		Title:        "Group all status calls into one",
		Help: `This takes the following parameters

- jobid - id of the job to get status of 
- wait  - seconds to wait for job operation to complete

Returns

job: job status
stats: running stats
transferred: transferred stats
`,
	})
}

// rcJobProgress aggregates and returns prepared job progress information.
func rcJobProgress(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	var jobOut, aggregatedOut map[string]interface{}
	jobid, err := in.GetInt64("jobid")
	if err != nil {
		return nil, err
	}
	wait, err := in.GetInt64("wait")
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, err
	}

	if wait > 0 {
		err = waitForJobFinish(ctx, jobid, wait)
		if err != nil {
			return nil, err
		}
	}

	jobOut, err = rcCalls.Get("job/status").Fn(ctx, in)
	if err != nil {
		return nil, err
	}
	in["group"] = fmt.Sprintf("job/%d", jobid)
	aggregatedOut, err = rcCalls.Get("core/aggregated").Fn(ctx, in)
	if err != nil {
		return nil, err
	}

	if err := rc.Reshape(&out, aggregateJobInfo(jobOut, aggregatedOut)); err != nil {
		return nil, err
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "job/progress",
		AuthRequired: true,
		Fn:           rcJobProgress,
		Title:        "Return job progress",
		Help: `This takes the following parameters

- jobid - id of the job to get progress of
- wait  - seconds to wait for job operation to complete

Returns

status: string
completed_at: string
started_at: string
error: string
failed: int64
skipped: int64
uploaded: int64
`,
	})
}

type jobProgress struct {
	// status of the job
	// Enum: [success error running not_found]
	Status JobStatus `json:"status"`
	// time at which job completed
	// Format: date-time
	CompletedAt time.Time `json:"completed_at"`
	// time at which job started
	// Format: date-time
	StartedAt time.Time `json:"started_at"`
	// string description of the error (empty if successful)
	Error string `json:"error"`
	// number of bytes that failed transfer
	Failed int64 `json:"failed"`
	// number of bytes that were skipped
	Skipped int64 `json:"skipped"`
	// number of bytes that are successfully uploaded
	Uploaded int64 `json:"uploaded"`
}

type jobFields struct {
	ID        int64  `mapstructure:"id"`
	StartTime string `mapstructure:"startTime"`
	EndTime   string `mapstructure:"endTime"`
	Finished  bool   `mapstructure:"finished"`
	Success   bool   `mapstructure:"success"`
	Error     string `mapstructure:"error"`
}

type aggFields struct {
	Aggregated accounting.AggregatedTransferInfo `mapstructure:"aggregated"`
}

func aggregateJobInfo(jobParam, aggregatedParam rc.Params) jobProgress {
	// Parse parameters
	var job jobFields
	if err := mapstructure.Decode(jobParam, &job); err != nil {
		panic(err)
	}
	var aggregated aggFields
	if err := mapstructure.Decode(aggregatedParam, &aggregated); err != nil {
		panic(err)
	}

	// Init job progress
	p := jobProgress{
		Status: statusOfJob(job),
		Error:  job.Error,
	}
	if t, err := timeutc.Parse(time.RFC3339, job.StartTime); err == nil && !t.IsZero() {
		p.StartedAt = t
	}
	if t, err := timeutc.Parse(time.RFC3339, job.EndTime); err == nil && !t.IsZero() {
		p.CompletedAt = t
	}

	p.Uploaded = aggregated.Aggregated.Uploaded
	p.Skipped = aggregated.Aggregated.Skipped
	p.Failed = aggregated.Aggregated.Failed

	return p
}

// JobStatus represents one of the available job statuses.
type JobStatus string

// JobStatus enumeration.
const (
	JobError    JobStatus = "error"
	JobSuccess  JobStatus = "success"
	JobRunning  JobStatus = "running"
	JobNotFound JobStatus = "not_found"
)

func statusOfJob(job jobFields) (status JobStatus) {
	status = JobRunning

	switch {
	case job.ID == 0:
		status = JobNotFound
	case job.Finished && job.Success:
		status = JobSuccess
	case job.Finished && !job.Success:
		status = JobError
	}

	return
}

var errJobNotFound = errors.New("job not found")

func waitForJobFinish(ctx context.Context, jobid, wait int64) error {
	w := time.Second * time.Duration(wait)
	done := make(chan struct{})

	stop, err := jobs.OnFinish(jobid, func() {
		close(done)
	})
	if err != nil {
		// Returning errJobNotFound because jobs.OnFinish can fail only if job
		// is not available and it doesn't return any specific error to signal
		// that higher up the call chain.
		return errJobNotFound
	}
	defer stop()

	timer := time.NewTimer(w)
	defer timer.Stop()

	select {
	case <-done:
		return nil
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// rcFileInfo returns basic object information.
func rcFileInfo(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(ctx, in)
	if err != nil {
		return nil, err
	}
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}
	out = rc.Params{
		"modTime": o.ModTime(ctx),
		"size":    o.Size(),
	}
	return
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/fileinfo",
		AuthRequired: true,
		Fn:           rcFileInfo,
		Title:        "Get basic file information",
		Help: `This takes the following parameters

- fs - a remote name string eg "s3:path/to/dir"`,
	})
}

// rcCat returns the whole remote object in body.
func rcCat(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(ctx, in)
	if err != nil {
		return nil, err
	}
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}
	w, err := in.GetHTTPResponseWriter()
	if err != nil {
		return nil, err
	}
	r, err := o.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Length", fmt.Sprint(o.Size()))

	n, err := io.Copy(w, r)
	if err != nil {
		if n == 0 {
			return nil, err
		}
		fs.Errorf(o, "copy error %s", err)
	}

	return nil, errResponseWritten
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/cat",
		AuthRequired: true,
		Fn:           wrap(rcCat, pathHasPrefix("backup/meta/")),
		Title:        "Concatenate any files and send them in response",
		Help: `This takes the following parameters

- fs - a remote name string eg "s3:path/to/dir"

Returns

- body - file content`,
		NeedsResponse: true,
	})

	// Adding it here because it is not part of the agent.json.
	// It should be removed once we are able to generate client for this call.
	internal.RcloneSupportedCalls.Add("operations/cat")
}

func rcPut(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(ctx, in)
	if err != nil {
		return nil, err
	}

	r, err := in.GetHTTPRequest()
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	info := object.NewStaticObjectInfo(remote, timeutc.Now(), r.ContentLength, true, nil, f)

	dst, err := f.NewObject(ctx, remote)
	if err == nil {
		if rcops.Equal(ctx, info, dst) {
			return nil, nil
		} else if rclone.GetConfig().Immutable {
			fs.Errorf(dst, "Source and destination exist but do not match: immutable file modified")
			return nil, fs.ErrorImmutableModified
		}
	} else if !errors.Is(err, fs.ErrorObjectNotFound) {
		return nil, err
	}

	obj, err := rcops.RcatSize(ctx, f, remote, r.Body, r.ContentLength, info.ModTime(ctx))
	if err != nil {
		return nil, err
	}
	fs.Debugf(obj, "Upload Succeeded")

	return nil, err
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/put",
		Fn:           rcPut,
		Title:        "Save provided content as file",
		AuthRequired: true,
		Help: `This takes the following parameters:

- fs - a remote name string eg "s3:path/to/file"
- body - file content`,
		NeedsRequest: true,
	})

	// Adding it here because it is not part of the agent.json.
	// It should be removed once we are able to generate client for this call.
	internal.RcloneSupportedCalls.Add("operations/put")
}

// rcCheckPermissions checks if location is available for listing, getting,
// creating, and deleting objects.
func rcCheckPermissions(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	l, err := rc.GetFs(ctx, in)
	if err != nil {
		return nil, errors.Wrap(err, "init location")
	}

	if err := operations.CheckPermissions(ctx, l); err != nil {
		fs.Errorf(nil, "Location check: error=%s", err)
		return nil, err
	}

	fs.Infof(nil, "Location check done")
	return nil, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "operations/check-permissions",
		AuthRequired: true,
		Fn:           rcCheckPermissions,
		Title:        "Checks listing, getting, creating, and deleting objects",
		Help: `This takes the following parameters

- fs - a remote name string eg "s3:repository"

`,
	})
}

func init() {
	c := rc.Calls.Get("operations/movefile")
	c.Fn = wrap(c.Fn, sameDir())
}

// VersionedFileRegex is a rclone formatted regex that can be used to distinguish versioned files.
const VersionedFileRegex = `{**.sm_*UTC}`

// rcChunkedList supports streaming output of the listing.
func rcChunkedList(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	f, remote, err := rc.GetFsAndRemote(ctx, in)
	if err != nil {
		return nil, err
	}

	var opt rcops.ListJSONOpt
	err = in.GetStruct("opt", &opt)
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}

	newest, err := in.GetBool("newestOnly")
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	versioned, err := in.GetBool("versionedOnly")
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	if newest && versioned {
		return nil, errors.New("newestOnly and versionedOnly parameters can't be specified at the same time")
	}
	if (newest || versioned) && opt.DirsOnly {
		return nil, errors.New("newestOnly and versionedOnly doesn't work on directories")
	}

	ctx, cfg := filter.AddConfig(ctx)
	if newest {
		if err := cfg.Add(false, VersionedFileRegex); err != nil {
			return nil, err
		}
	}
	if versioned {
		if err := cfg.Add(true, VersionedFileRegex); err != nil {
			return nil, err
		}
		if err := cfg.Add(false, `{**}`); err != nil {
			return nil, err
		}
	}

	w, err := in.GetHTTPResponseWriter()
	if err != nil {
		return nil, err
	}
	enc := newListJSONEncoder(w.(writerFlusher), defaultListEncoderMaxItems)
	err = rcops.ListJSON(ctx, f, remote, &opt, enc.Callback)
	if err != nil {
		return enc.Result(err)
	}
	// Localdir fs implementation ignores permission errors, but stores them in
	// statistics. We must inform user about them.
	if err := accounting.Stats(ctx).GetLastError(); err != nil {
		if os.IsPermission(errors.Cause(err)) {
			return enc.Result(err)
		}
	}

	enc.Close()

	return enc.Result(nil)
}

func init() {
	c := rc.Calls.Get("operations/list")
	c.Fn = rcChunkedList
	c.NeedsResponse = true
}

// rcMoveOrCopyDir returns an rc function that moves or copies files from
// source to destination directory depending on the constructor argument.
// Only works for directories with single level depth.
func rcMoveOrCopyDir(doMove bool) func(ctx context.Context, in rc.Params) (rc.Params, error) {
	return func(ctx context.Context, in rc.Params) (rc.Params, error) {
		srcFs, srcRemote, err := getFsAndRemoteNamed(ctx, in, "srcFs", "srcRemote")
		if err != nil {
			return nil, err
		}
		dstFs, dstRemote, err := getFsAndRemoteNamed(ctx, in, "dstFs", "dstRemote")
		if err != nil {
			return nil, err
		}

		// Set suffix for files that would be otherwise overwritten or deleted
		ctx, cfg := fs.AddConfig(ctx)
		cfg.Suffix, err = in.GetString("suffix")
		if err != nil && !rc.IsErrParamNotFound(err) {
			return nil, err
		}

		return nil, sync.CopyDir2(ctx, dstFs, dstRemote, srcFs, srcRemote, doMove)
	}
}

// rcCopyPaths returns rc function that copies paths from
// source to destination.
func rcCopyPaths() func(ctx context.Context, in rc.Params) (rc.Params, error) {
	return func(ctx context.Context, in rc.Params) (rc.Params, error) {
		srcFs, srcRemote, err := getFsAndRemoteNamed(ctx, in, "srcFs", "srcRemote")
		if err != nil {
			return nil, err
		}
		dstFs, dstRemote, err := getFsAndRemoteNamed(ctx, in, "dstFs", "dstRemote")
		if err != nil {
			return nil, err
		}
		paths, err := getStringSlice(in, "paths")
		if err != nil {
			return nil, err
		}

		return nil, sync.CopyPaths(ctx, dstFs, dstRemote, srcFs, srcRemote, paths, false)
	}
}

// getFsAndRemoteNamed gets fs and remote path from the params, but it doesn't
// fail if remote path is not provided.
// In that case it is assumed that path is empty and root of the fs is used.
func getFsAndRemoteNamed(ctx context.Context, in rc.Params, fsName, remoteName string) (f fs.Fs, remote string, err error) {
	remote, err = in.GetString(remoteName)
	if err != nil && !rc.IsErrParamNotFound(err) {
		return
	}
	f, err = rc.GetFsNamed(ctx, in, fsName)
	return
}

func getStringSlice(in rc.Params, key string) ([]string, error) {
	value, err := in.Get(key)
	if err != nil {
		return nil, err
	}

	tmp, ok := value.([]interface{})
	if !ok {
		return nil, errors.Errorf("expecting []interface{} value for key %q (was %T)", key, value)
	}

	var res []string
	for i, v := range tmp {
		str, ok := v.(string)
		if !ok {
			return nil, errors.Errorf("expecting string value for slice index nr %d (was %T)", i, str)
		}
		res = append(res, str)
	}

	return res, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "sync/movedir",
		AuthRequired: true,
		Fn:           wrap(rcMoveOrCopyDir(true), localToRemote()),
		Title:        "Move contents of source directory to destination",
		Help: `This takes the following parameters:

- srcFs - a remote name string eg "s3:" for the source
- srcRemote - a directory path within that remote for the source
- dstFs - a remote name string eg "gcs:" for the destination
- dstRemote - a directory path within that remote for the destination`,
	})

	rc.Add(rc.Call{
		Path:         "sync/copydir",
		AuthRequired: true,
		Fn:           wrap(rcMoveOrCopyDir(false), localToRemote()),
		Title:        "Copy contents from source directory to destination",
		Help: `This takes the following parameters:

- srcFs - a remote name string eg "s3:" for the source
- srcRemote - a directory path within that remote for the source
- dstFs - a remote name string eg "gcs:" for the destination
- dstRemote - a directory path within that remote for the destination`,
	})

	rc.Add(rc.Call{
		Path:         "sync/copypaths",
		AuthRequired: true,
		Fn:           wrap(rcCopyPaths(), remoteToLocal()),
		Title:        "Copy paths from source directory to destination",
		Help: `This takes the following parameters:

- srcFs - a remote name string eg "s3:" for the source
- srcRemote - a directory path within that remote for the source
- dstFs - a remote name string eg "gcs:" for the destination
- dstRemote - a directory path within that remote for the destination
- paths - slice of paths to be copied from source directory to destination`,
	})
}

// rcCalls contains the original rc.Calls before filtering with all the added
// custom calls in this file.
var rcCalls *rc.Registry

func init() {
	rcCalls = rc.Calls
	filterRcCalls()
}

// filterRcCalls disables all default calls and whitelists only supported calls.
func filterRcCalls() {
	rc.Calls = rc.NewRegistry()

	for _, c := range rcCalls.List() {
		if internal.RcloneSupportedCalls.Has(c.Path) {
			rc.Add(*c)
		}
	}
}
