package repounit

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	apimachineryutilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// findCRDKindNames parses YAML content and returns the kind names of all CustomResourceDefinition objects found in it.
func findCRDKindNames(content []byte) ([]string, error) {
	var names []string
	decoder := apimachineryutilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(content), 4096)
	for {
		var obj map[string]interface{}
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decoding YAML document: %w", err)
		}
		if obj == nil {
			continue
		}

		kind, _ := obj["kind"].(string)
		if kind != "CustomResourceDefinition" {
			continue
		}

		spec, _ := obj["spec"].(map[string]interface{})
		if spec == nil {
			continue
		}
		n, _ := spec["names"].(map[string]interface{})
		if n == nil {
			continue
		}
		kindName, _ := n["kind"].(string)
		if kindName != "" {
			names = append(names, kindName)
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("no CRD kind names found")
	}

	return names, nil
}

type unquotedOccurrence struct {
	Line int
	Word string
	Text string
}

// findUnquotedOccurrences scans content line by line and returns occurrences of any of the given words that appear outside of backtick-quoted segments.
func findUnquotedOccurrences(content []byte, words []string) []unquotedOccurrence {
	patterns := make(map[string]*regexp.Regexp, len(words))
	for _, w := range words {
		patterns[w] = regexp.MustCompile(`\b` + regexp.QuoteMeta(w) + `\b`)
	}

	backtickContentRe := regexp.MustCompile("`[^`]+`")

	var results []unquotedOccurrence
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Remove all backtick-quoted segments to find unquoted words.
		stripped := backtickContentRe.ReplaceAllString(line, "")

		for word, pattern := range patterns {
			if pattern.MatchString(stripped) {
				results = append(results, unquotedOccurrence{
					Line: lineNum,
					Word: word,
					Text: strings.TrimSpace(line),
				})
			}
		}
	}

	return results
}

// TestChangelogCRDNamesAreBacktickQuoted verifies that all CRD kind names referenced in CHANGELOG.md are enclosed in backticks.
func TestChangelogCRDNamesAreBacktickQuoted(t *testing.T) {
	t.Parallel()

	operatorYAML, err := os.ReadFile("../../deploy/operator.yaml")
	if err != nil {
		t.Fatalf("failed to read operator.yaml: %v", err)
	}

	crdNames, err := findCRDKindNames(operatorYAML)
	if err != nil {
		t.Fatal(err)
	}

	changelog, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatalf("failed to read CHANGELOG.md: %v", err)
	}

	for _, v := range findUnquotedOccurrences(changelog, crdNames) {
		t.Errorf("CHANGELOG.md:%d: CRD name %q must be backtick-quoted: %s", v.Line, v.Word, v.Text)
	}
}
