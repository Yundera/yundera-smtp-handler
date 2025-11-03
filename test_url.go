package main

import (
	"fmt"
	"strings"
)

func main() {
	// Test cases
	testCases := []string{
		"https://app.yundera.com/service/pcs/user",
		"https://nasselle.com/service/pcs/user",
		"https://app.yundera.com/service/pcs",
		"",
	}

	for _, url := range testCases {
		result := url
		if result == "" {
			result = "https://app.yundera.com/service/pcs"
			fmt.Printf("Input: <empty>\n  -> Using default: %s\n\n", result)
		} else {
			result = strings.TrimSuffix(result, "/user")
			fmt.Printf("Input: %s\n  -> After TrimSuffix: %s\n\n", url, result)
		}
	}
}
