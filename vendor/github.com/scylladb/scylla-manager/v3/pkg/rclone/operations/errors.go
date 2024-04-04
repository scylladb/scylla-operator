// Copyright (C) 2017 ScyllaDB

package operations

import (
	"encoding/xml"
	"errors"
	"strings"
)

// BackendXMLError is general error parsed from Error XML message as specified in
// https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html#ErrorCodeList
// https://cloud.google.com/storage/docs/xml-api/reference-status
// https://learn.microsoft.com/en-us/rest/api/storageservices/status-and-error-codes2
type BackendXMLError struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

// ParseBackendXMLError reads the error as string and ties to parse the XML structure.
// The reason is that the error returned from rclone is flattened ex. `*errors.fundamental s3 upload: 404 Not Found: <?xml version="1.0" encoding="UTF-8"?>`.
func ParseBackendXMLError(err error) (*BackendXMLError, error) {
	s := err.Error()

	idx := strings.Index(s, "<?xml")
	if idx < 0 {
		return nil, errors.New("not XML")
	}
	wrap := s[:idx]
	s = s[idx:]

	var e BackendXMLError
	if err := xml.Unmarshal([]byte(s), &e); err != nil {
		return nil, err
	}
	if e.Code == "" || e.Message == "" {
		return nil, errors.New("missing code or message field in parsed xml error")
	}
	// Append the non-xml beginning of an error
	e.Message = wrap + e.Message
	return &e, nil
}

func (e *BackendXMLError) Error() string {
	return e.Message + " (code:" + e.Code + ")"
}
