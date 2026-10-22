package main

import (
	"fmt"
	"net/http"
	"time"

	"healthchecker/internal/checker"
)

func modeTitle(mode string) string {
	switch mode {
	case "sequential":
		return "Level 1: sequential checks"
	case "mutex":
		return "Level 2: goroutine + WaitGroup + Mutex"
	default:
		return "Level 3: goroutine + WaitGroup + chan"
	}
}

func printResult(result checker.Result) {
	if result.Err != nil {
		fmt.Printf("[ERR] %s | %s\n      %v\n", result.URL, result.Duration.Round(time.Millisecond), result.Err)
		return
	}
	label := "HTTP"
	if result.StatusCode == http.StatusOK {
		label = " OK "
	}
	fmt.Printf("[%s] %-38s | %d %-19s | %s\n", label, result.URL,
		result.StatusCode, http.StatusText(result.StatusCode), result.Duration.Round(time.Millisecond))
}
