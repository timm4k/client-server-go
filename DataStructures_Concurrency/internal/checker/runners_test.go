package checker

import (
	"testing"
	"time"

	"healthchecker/internal/demo"
)

func TestAllModesPreserveResults(t *testing.T) {
	urls, cleanup := demo.Targets()
	defer cleanup()
	checker := NewHTTPChecker(2 * time.Second)
	defer checker.Client.CloseIdleConnections()
	expected := Stats{Total: 5, OK: 2, HTTPProblems: 2, NetworkErrors: 1}
	for _, mode := range []string{"sequential", "mutex", "channels"} {
		t.Run(mode, func(t *testing.T) {
			var results []Result
			var stats Stats
			switch mode {
			case "sequential":
				results, stats = RunSequential(checker, urls)
			case "mutex":
				results, stats = RunMutex(checker, urls)
			case "channels":
				for result := range RunChannels(checker, urls) {
					results = append(results, result)
					stats.Add(result)
				}
			}
			if stats != expected || len(results) != len(urls) {
				t.Fatalf("lost results or incorrect stats: %+v, count=%d", stats, len(results))
			}
			seen := make(map[string]int)
			for _, result := range results {
				seen[result.URL]++
			}
			for _, url := range urls {
				if seen[url] != 1 {
					t.Fatalf("URL %q occurred %d times", url, seen[url])
				}
			}
		})
	}
}

func TestEmptyInputs(t *testing.T) {
	checker := NewHTTPChecker(time.Second)
	if results, stats := RunSequential(checker, nil); len(results) != 0 || stats != (Stats{}) {
		t.Fatal("sequential mode did not handle empty input")
	}
	if results, stats := RunMutex(checker, nil); len(results) != 0 || stats != (Stats{}) {
		t.Fatal("mutex mode did not handle empty input")
	}
	select {
	case _, open := <-RunChannels(checker, nil):
		if open {
			t.Fatal("empty channel was not closed")
		}
	case <-time.After(time.Second):
		t.Fatal("channel close deadlocked")
	}
}
