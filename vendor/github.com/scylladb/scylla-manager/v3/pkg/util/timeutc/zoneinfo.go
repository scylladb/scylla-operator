// Copyright (C) 2021 ScyllaDB

package timeutc

import (
	"os"
	"strings"
	"time"
)

// LocalName is name of the local time zone conforming to the IANA Time Zone database, such as "America/New_York".
// Using time.Local if timezone is read from /etc/localtime its name is overwritten by "Local".
// LocalName provides the name as read from /etc/localtime static link.
var LocalName string

func init() {
	tz := time.Local.String() //nolint: gosmopolitan
	if tz == "Local" {
		p, err := os.Readlink("/etc/localtime")
		if err != nil {
			return
		}
		i := strings.LastIndex(p, "/zoneinfo/")
		if i < 0 {
			return
		}
		tz = p[i+len("/zoneinfo/"):]
		if _, err := time.LoadLocation(tz); err != nil {
			return
		}
	}

	LocalName = tz
}
