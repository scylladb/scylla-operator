// Copyright (C) 2017 ScyllaDB

package managerclient

import (
	"fmt"
	"io"
	"regexp"
)

// PrintError handles ErrorResponse and can format multierror strings.
func PrintError(w io.Writer, err error) {
	v, ok := err.(interface { // nolint: errorlint
		GetPayload() *ErrorResponse
	})
	if ok {
		p := v.GetPayload()

		if len(p.Details) > 0 {
			fmt.Fprintf(w, "%s\n\n", p.Details)
		}
		fmt.Fprintf(w, "Error: %s\n", FormatError(p.Message))
		fmt.Fprintf(w, "Trace ID: %s (grep in scylla-manager logs)\n", p.TraceID)
	} else {
		fmt.Fprintf(w, "Error: %s\n", err)
	}
	fmt.Fprintln(w)
}

// FormatError formats messages created by using multierror with
// errors wrapped with host IP so that each host error is in it's own line.
func FormatError(msg string) string {
	const prefix = " "

	// Fairly relaxed IPv4 and IPv6 heuristic pattern, a proper pattern can
	// be very complex
	const ipRegex = `([0-9A-Fa-f]{1,4}:){7}[0-9A-Fa-f]{1,4}|(\d{1,3}\.){3}\d{1,3}`

	// Move host errors to newline
	r := regexp.MustCompile(`(^|: |; )(` + ipRegex + `): `)

	return r.ReplaceAllString(msg, "\n"+prefix+"${2}: ")
}
