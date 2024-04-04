// Copyright (C) 2017 ScyllaDB

// Package rcserver implements the HTTP endpoint to serve the remote control
package rcserver

//go:generate ./internalgen.sh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
	"github.com/scylladb/scylla-manager/v3/pkg/rclone"
	"github.com/scylladb/scylla-manager/v3/pkg/rclone/operations"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
)

var initOnce sync.Once

var (
	// ErrNotFound is returned when remote call is not available.
	ErrNotFound        = errors.New("not found")
	errResponseWritten = errors.New("response already written")
)

// Server implements http.Handler interface.
type Server struct {
	memoryPool *sync.Pool
}

// New creates new rclone server.
// Since we are overriding default behavior of saving remote configuration to
// files, we need to include code that was called in
// rclone/fs/config.LoadConfig, which initializes accounting processes but is
// no longer called.
// It's probably done this way to make sure that configuration has opportunity
// to modify global config object before these processes are started as they
// depend on it.
// We are initializing it once here to make sure it's executed only when server
// is needed and configuration is completely loaded.
func New() Server {
	initOnce.Do(func() {
		rclone.StartAccountingOperations()

		// Set jobs options
		opts := rc.DefaultOpt
		opts.JobExpireDuration = 12 * time.Hour
		opts.JobExpireInterval = 1 * time.Minute
		jobs.SetOpt(&opts)
		// Rewind job ID to new values
		jobs.SetInitialJobID(timeutc.Now().Unix())
	})
	return Server{
		memoryPool: &sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}
}

// writeError writes a formatted error to the output.
func (s Server) writeError(path string, in rc.Params, w http.ResponseWriter, err error, status int) {
	// Ignore if response was already written
	if errors.Is(err, errResponseWritten) {
		return
	}

	fs.Errorf(nil, "rc: %q: error: %v", path, err)
	// Adjust the error return for some well known errors
	if e, ok := err.(operations.PermissionError); ok { // nolint: errorlint
		status = e.StatusCode()
	} else {
		switch {
		case isNotFoundErr(err):
			status = http.StatusNotFound
		case isBadRequestErr(err):
			status = http.StatusBadRequest
		case isForbiddenErr(err):
			status = http.StatusForbidden
		}
	}
	// Try to parse xml errors for increased readability
	if xmlErr, e := operations.ParseBackendXMLError(err); e == nil {
		err = xmlErr
	}
	w.WriteHeader(status)
	err = s.writeJSON(w, rc.Params{
		"status":  status,
		"message": err.Error(),
		"input":   in,
		"path":    path,
	})
	if err != nil {
		// can't return the error at this point
		fs.Errorf(nil, "rc: write JSON output: %v", err)
	}
}

func (s Server) writeJSON(w http.ResponseWriter, out rc.Params) error {
	buf := s.memoryPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		s.memoryPool.Put(buf)
	}()

	if err := json.NewEncoder(buf).Encode(out); err != nil {
		return err
	}
	w.Header().Set("Content-Length", fmt.Sprint(buf.Len()))
	_, err := io.Copy(w, buf)
	return err
}

// nolint: errorlint
func isBadRequestErr(err error) bool {
	cause := errors.Cause(err)
	return rc.IsErrParamInvalid(err) ||
		rc.IsErrParamNotFound(err) ||
		IsErrParamInvalid(err) ||
		cause == fs.ErrorIsFile ||
		cause == fs.ErrorNotAFile ||
		cause == fs.ErrorDirectoryNotEmpty ||
		cause == fs.ErrorDirExists ||
		cause == fs.ErrorListBucketRequired
}

// nolint: errorlint
func isNotFoundErr(err error) bool {
	cause := errors.Cause(err)
	return cause == fs.ErrorDirNotFound ||
		cause == fs.ErrorObjectNotFound ||
		cause == fs.ErrorNotFoundInConfigFile ||
		cause == errJobNotFound
}

func isForbiddenErr(err error) bool {
	return os.IsPermission(errors.Cause(err))
}

const (
	bodySizeLimit int64 = 1024 * 1024
	notFoundJSON        = `{"message":"Not found","status":404}`
)

// ServeHTTP implements http.Handler interface.
func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimLeft(r.URL.Path, "/")

	if r.Method != http.MethodPost {
		s.writeError(path, nil, w, errors.Errorf("method %q not allowed", r.Method), http.StatusMethodNotAllowed)
		return
	}

	contentType, err := parseContentType(r.Header)
	if err != nil {
		s.writeError(path, nil, w, errors.Wrap(err, "parse Content-Type header"), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	values := r.URL.Query()
	if contentType == "application/x-www-form-urlencoded" || contentType == "application/octet-stream" {
		// Parse the POST and URL parameters into r.Form, for others r.Form will be empty value
		err := r.ParseForm()
		if err != nil {
			s.writeError(path, nil, w, errors.Wrap(err, "parse form/URL parameters"), http.StatusBadRequest)
			return
		}
		values = r.Form
	}

	// Merge POST and URL parameters into in
	in := make(rc.Params)
	for k, vs := range values {
		if len(vs) > 0 {
			in[k] = vs[len(vs)-1]
		}
	}
	// Add additional parameters to pass down to a function
	extra := make(rc.Params)

	// Parse a JSON blob from the input
	if contentType == "application/json" {
		j, err := io.ReadAll(&io.LimitedReader{R: r.Body, N: bodySizeLimit})
		if err != nil {
			s.writeError(path, in, w, errors.Wrap(err, "read request body"), http.StatusBadRequest)
			return
		}
		if len(j) > 0 {
			if err := json.Unmarshal(j, &in); err != nil {
				s.writeError(path, in, w, errors.Wrap(err, "read input JSON"), http.StatusBadRequest)
				return
			}
		}
	}

	// Find the call
	call := rc.Calls.Get(path)
	if call == nil {
		agentUnexposedAccess.With(prometheus.Labels{"addr": r.RemoteAddr, "path": path}).Inc()
		fs.Errorf(nil, "SECURITY call to unexported endpoint [path=%s, ip=%s]", path, r.RemoteAddr)
		http.Error(w, notFoundJSON, http.StatusNotFound)
		return
	}
	if call.NeedsRequest {
		extra["_request"] = r
	}
	if call.NeedsResponse {
		extra["_response"] = w
	}
	fn := call.Fn

	if err := validateFsName(in); err != nil {
		s.writeError(path, in, w, err, http.StatusBadRequest)
		return
	}

	// Check to see if it is async or not
	isAsync, err := in.GetBool("_async")
	if rc.NotErrParamNotFound(err) {
		s.writeError(path, in, w, err, http.StatusBadRequest)
		return
	}

	fs.Debugf(nil, "rc: %q: with parameters %+v", path, in)
	var (
		out   rc.Params
		jobID int64
	)

	// Merge in and extra to one
	var inExt rc.Params
	if len(extra) == 0 {
		inExt = in
	} else {
		inExt = in.Copy()
		for k, v := range extra {
			inExt[k] = v
		}
	}

	if isAsync {
		out, err = jobs.StartAsyncJob(fn, inExt)
		jobID = out["jobid"].(int64)
	} else {
		out, err = fn(r.Context(), inExt)
	}

	if rc.IsErrParamNotFound(err) || errors.Is(err, ErrNotFound) {
		s.writeError(path, in, w, err, http.StatusNotFound)
		return
	} else if err != nil {
		s.writeError(path, in, w, err, http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = make(rc.Params)
	}

	fs.Debugf(nil, "rc: %q: reply %+v: %v", path, out, err)
	w.Header().Add("x-rclone-jobid", fmt.Sprintf("%d", jobID))

	if err := s.writeJSON(w, out); err != nil {
		s.writeError(path, in, w, err, http.StatusInternalServerError)
		return
	}
}

func parseContentType(headers http.Header) (string, error) {
	if headers.Get("Content-Type") == "" {
		return "", nil
	}

	contentType, _, err := mime.ParseMediaType(headers.Get("Content-Type"))
	if err != nil {
		return "", err
	}

	return contentType, nil
}

// validateFsName ensures that only allowed file systems can be used in
// parameters with file system format.
func validateFsName(in rc.Params) error {
	for _, name := range []string{"fs", "srcFs", "dstFs"} {
		v, err := in.GetString(name)
		if err != nil {
			if rc.IsErrParamNotFound(err) {
				continue
			}
			return err
		}
		_, remote, _, err := fs.ParseRemote(v)
		if err != nil {
			return err
		}
		if !rclone.HasProvider(remote) {
			return errParamInvalid{errors.Errorf("invalid provider %s in %s param", remote, name)}
		}
	}
	return nil
}

type errParamInvalid struct {
	error
}

// IsErrParamInvalid checks if the provided error is invalid.
// Added as a workaround for private error field of fs.ErrParamInvalid.
func IsErrParamInvalid(err error) bool {
	_, ok := err.(errParamInvalid) // nolint: errorlint
	return ok
}
