// Copyright (C) 2024 ScyllaDB

package manager

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/scylladb/scylla-manager/v3/pkg/util/timeutc"
	"github.com/scylladb/scylla-operator/pkg/util/duration"
)

var (
	byteCountRe          = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)(B|[KMGTPE]iB)`)
	byteCountReValueIdx  = 1
	byteCountReSuffixIdx = 2
)

// ParseByteCount returns byte count parsed from input string.
// This is opposite of StringByteCount function.
func ParseByteCount(s string) (int64, error) {
	const unit = 1024
	var exps = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	parts := byteCountRe.FindStringSubmatch(s)
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", s, strings.Join(exps, ","))
	}

	v, err := strconv.ParseFloat(parts[byteCountReValueIdx], 64)
	if err != nil {
		return 0, fmt.Errorf("parsing value for byte size string %q: %w", s, err)
	}

	pow := 0
	for i, e := range exps {
		if e == parts[byteCountReSuffixIdx] {
			pow = i
		}
	}

	mul := math.Pow(unit, float64(pow))

	return int64(v * mul), nil
}

// ParseStartDate parses the supplied string as a strfmt.DateTime.
func ParseStartDate(value string) (strfmt.DateTime, error) {
	if strings.HasPrefix(value, "now") {
		now := timeutc.Now()
		if value == "now" {
			return strfmt.DateTime{}, nil
		}

		d, err := duration.ParseDuration(value[3:])
		if err != nil {
			return strfmt.DateTime{}, err
		}
		if d == 0 {
			return strfmt.DateTime{}, nil
		}

		return strfmt.DateTime(now.Add(d.Duration())), nil
	}

	// No more heuristics, assume the user passed a date formatted string
	t, err := timeutc.Parse(time.RFC3339, value)
	if err != nil {
		return strfmt.DateTime{}, err
	}

	return strfmt.DateTime(t), nil
}
