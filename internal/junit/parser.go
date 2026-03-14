package junit

import "encoding/xml"

// TestSuites is the top-level JUnit XML element.
type TestSuites struct {
	XMLName    xml.Name    `xml:"testsuites"`
	TestSuites []TestSuite `xml:"testsuite"`
}

// TestSuite represents a single test suite.
type TestSuite struct {
	Name      string     `xml:"name,attr"`
	Tests     int        `xml:"tests,attr"`
	Failures  int        `xml:"failures,attr"`
	Errors    int        `xml:"errors,attr"`
	Time      float64    `xml:"time,attr"`
	TestCases []TestCase `xml:"testcase"`
}

// TestCase represents a single test case.
type TestCase struct {
	Name      string   `xml:"name,attr"`
	ClassName string   `xml:"classname,attr"`
	Time      float64  `xml:"time,attr"`
	Failure   *Failure `xml:"failure"`
	Skipped   bool     `xml:"-"`
	SkippedEl *struct{} `xml:"skipped"`
}

// Failure captures test failure details.
type Failure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

// Parse parses JUnit XML data into TestSuites.
func Parse(data []byte) (*TestSuites, error) {
	var suites TestSuites
	if err := xml.Unmarshal(data, &suites); err != nil {
		return nil, err
	}
	// Populate Skipped bool from XML element presence.
	for i := range suites.TestSuites {
		for j := range suites.TestSuites[i].TestCases {
			tc := &suites.TestSuites[i].TestCases[j]
			tc.Skipped = tc.SkippedEl != nil
		}
	}
	return &suites, nil
}

// Summary returns total tests, total failures, and pass rate.
func (ts *TestSuites) Summary() (tests, failures int, passRate float64) {
	for _, s := range ts.TestSuites {
		tests += s.Tests
		failures += s.Failures + s.Errors
	}
	if tests > 0 {
		passRate = float64(tests-failures) / float64(tests) * 100
	}
	return
}
