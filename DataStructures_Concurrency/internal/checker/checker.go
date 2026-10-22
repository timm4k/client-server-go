package checker

import (
	"fmt"
	"net/http"
	"time"
)

type Result struct {
	URL        string
	StatusCode int
	Duration   time.Duration
	Err        error
}
type Checker interface {
	Check(url string) Result
}

type HTTPChecker struct {
	Client *http.Client
}

var _ Checker = (*HTTPChecker)(nil)

func NewHTTPChecker(timeout time.Duration) *HTTPChecker {
	return &HTTPChecker{Client: &http.Client{Timeout: timeout}}
}

func (c *HTTPChecker) Check(url string) Result {
	start := time.Now()
	result := Result{URL: url}
	response, err := c.Client.Get(url)
	result.Duration = time.Since(start)
	if err != nil {
		result.Err = fmt.Errorf("HTTP request: %w", err)
		return result
	}
	defer response.Body.Close()
	result.StatusCode = response.StatusCode
	return result
}

type Stats struct {
	Total         int
	OK            int
	HTTPProblems  int
	NetworkErrors int
}

func (s *Stats) Add(result Result) {
	s.Total++
	switch {
	case result.Err != nil:
		s.NetworkErrors++
	case result.StatusCode == http.StatusOK:
		s.OK++
	default:
		s.HTTPProblems++
	}
}
