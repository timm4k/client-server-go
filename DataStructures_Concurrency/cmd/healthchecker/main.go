package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"healthchecker/internal/checker"
	"healthchecker/internal/demo"
)

func main() {
	mode := flag.String("mode", "all", "all | sequential | mutex | channels")
	timeout := flag.Duration("timeout", 3*time.Second, "Timeout for each HTTP request")
	demoMode := flag.Bool("demo", false, "Local demo without external internet access")
	flag.Parse()
	if *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "Error: timeout must be greater than zero")
		os.Exit(2)
	}
	if *mode != "all" && *mode != "sequential" && *mode != "mutex" && *mode != "channels" {
		fmt.Fprintln(os.Stderr, "Error: mode must be all, sequential, mutex or channels")
		os.Exit(2)
	}
	if *demoMode && flag.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "Error: use either -demo or custom URLs, not both")
		os.Exit(2)
	}
	urls := []string{
		"https://go.dev",
		"https://github.com",
		"https://example.com",
		"https://httpbin.org/status/404",
		"https://health-check.invalid",
	}
	if *demoMode {
		var cleanup func()
		urls, cleanup = demo.Targets()
		defer cleanup()
	} else if flag.NArg() > 0 {
		urls = flag.Args()
	}
	client := checker.NewHTTPChecker(*timeout)
	defer client.Client.CloseIdleConnections()
	fmt.Println("=== HTTP URL HEALTH CHECKER ===")
	fmt.Printf("URLs: %d | Timeout: %s | Local demo: %t\n", len(urls), *timeout, *demoMode)
	modes := []string{*mode}
	if *mode == "all" {
		modes = []string{"sequential", "mutex", "channels"}
	}
	timings := make(map[string]time.Duration)
	for _, current := range modes {
		client.Client.CloseIdleConnections()
		fmt.Printf("\n--- %s ---\n", modeTitle(current))
		start := time.Now()
		var stats checker.Stats
		if current == "channels" {
			for result := range checker.RunChannels(client, urls) {
				stats.Add(result)
				printResult(result)
			}
		} else {
			run := checker.RunSequential
			if current == "mutex" {
				run = checker.RunMutex
			}
			results, summary := run(client, urls)
			stats = summary
			for _, result := range results {
				printResult(result)
			}
		}
		timings[current] = time.Since(start)
		fmt.Printf("Summary: %d URLs | 200 OK: %d | Non-200 HTTP: %d | Request errors: %d\n",
			stats.Total, stats.OK, stats.HTTPProblems, stats.NetworkErrors)
		fmt.Printf("HTTP responses received: %d | Total time: %s\n",
			stats.Total-stats.NetworkErrors, timings[current].Round(time.Millisecond))
	}
	if *mode == "all" {
		fmt.Println("\n=== TIMING COMPARISON ===")
		for _, current := range modes {
			fmt.Printf("%-12s %s\n", current, timings[current].Round(time.Millisecond))
		}
		ratio := float64(timings["sequential"]) / float64(timings["channels"])
		fmt.Printf("Sequential / channels ratio: %.2fx\n", ratio)
		if ratio > 1 {
			fmt.Printf("Channel mode speedup: %.2fx\n", ratio)
		} else {
			fmt.Println("Concurrent mode provided no speedup in this run")
		}
	}
}
