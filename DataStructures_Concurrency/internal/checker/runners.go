package checker

import "sync"

func RunSequential(checker Checker, urls []string) ([]Result, Stats) {
	results := make([]Result, 0, len(urls))
	var stats Stats
	for _, url := range urls {
		result := checker.Check(url)
		results = append(results, result)
		stats.Add(result)
	}
	return results, stats
}
func RunMutex(checker Checker, urls []string) ([]Result, Stats) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var stats Stats
	results := make([]Result, 0, len(urls))
	for _, url := range urls {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			result := checker.Check(address)
			mu.Lock()
			results = append(results, result)
			stats.Add(result)
			mu.Unlock()
		}(url)
	}
	wg.Wait()
	return results, stats
}
func RunChannels(checker Checker, urls []string) <-chan Result {
	results := make(chan Result)
	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			results <- checker.Check(address)
		}(url)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	return results
}
