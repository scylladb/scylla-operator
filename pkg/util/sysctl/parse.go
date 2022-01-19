package sysctl

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ParseConfig parses sysctl config and returns an extracted map of key/value stored in config.
// The syntax is as follows:
//	 # comment
//	 ; comment
//
//	 token = value
//
// Blank lines are ignored, and whitespace before and after a token or value is ignored,
// although a value can contain whitespace within.
// Lines which begin with a # or ; are considered comments and ignored.
func ParseConfig(r io.Reader) (map[string]string, error) {
	kv := map[string]string{}

	scanner := bufio.NewScanner(r)
	for i := 1; scanner.Scan(); i++ {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid syntax at line %d", i)
		}

		kv[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	return kv, nil
}
