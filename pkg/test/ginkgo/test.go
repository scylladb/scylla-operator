package ginkgo

import (
	"time"
)

type TestSuite struct {
	Name        string
	Description string
	TestTimeout time.Duration

	// Count indicates how many times each test in this suite should be executed.
	Count int

	DefaultParallelism int

	LabelFilter  string
	FocusStrings []string
	SkipStrings  []string
}

type TestSuites []*TestSuite

func (tss TestSuites) Names() []string {
	var suiteNames []string
	for _, s := range tss {
		suiteNames = append(suiteNames, s.Name)
	}
	return suiteNames
}

func (tss TestSuites) Find(name string) *TestSuite {
	for _, s := range tss {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func (tss TestSuites) Contains(name string) bool {
	return tss.Find(name) != nil
}
