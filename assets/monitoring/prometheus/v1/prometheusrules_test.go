package v1

import (
	"testing"
)

func TestPrometheusRulesFS(t *testing.T) {
	prs, err := NewPrometheusRulesFromFS(prometheusRulesFS)
	if err != nil {
		t.Fatal(err)
	}

	rulesCount := len(prs)
	if rulesCount == 0 {
		t.Fatal("no prometheusRules found")
	}
	t.Logf("found %d prometheus rules", rulesCount)

	for fileName, pr := range prs {
		if pr.Accessed() {
			t.Errorf("promeheus rule file %q should not have been accessed yet", fileName)
		}

		if len(pr.Get()) == 0 {
			t.Errorf("promeheus rule file %q value can't be empty", fileName)
		}
	}
}
