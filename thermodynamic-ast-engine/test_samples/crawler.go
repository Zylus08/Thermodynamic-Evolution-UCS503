// crawler.go — deliberately "hot" Go service for testing the engine.
// Contains: nested for-loops, sync.Mutex, blocking I/O, make() allocations.

package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// CrawlResult holds a single page's crawl outcome.
type CrawlResult struct {
	URL        string
	StatusCode int
	Body       []byte
	Links      []string
}

// Crawler manages the crawl state and concurrency primitives.
type Crawler struct {
	mu      sync.Mutex     // protects visited map
	visited map[string]bool
	client  *http.Client
}

// NewCrawler initialises the crawler with a bounded HTTP client.
func NewCrawler(timeout time.Duration) *Crawler {
	return &Crawler{
		visited: make(map[string]bool),      // HotAllocation: make inside constructor
		client:  &http.Client{Timeout: timeout},
	}
}

// CrawlBatch fetches a batch of URLs — heavy blocking I/O inside a loop.
func (c *Crawler) CrawlBatch(urls []string) []CrawlResult {
	results := make([]CrawlResult, 0, len(urls))     // make() allocation

	for _, url := range urls {                        // loop ×1
		c.mu.Lock()                                   // SyncContention inside loop
		if c.visited[url] {
			c.mu.Unlock()
			continue
		}
		c.visited[url] = true
		c.mu.Unlock()

		resp, err := http.Get(url)                    // BlockingIO
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		buf := make([]byte, 0, 64*1024)               // allocation in loop
		scanner := bufio.NewReader(resp.Body)         // BlockingIO

		for {                                         // loop ×2
			line, err := scanner.ReadString('\n')     // BlockingIO in nested loop
			if err != nil {
				break
			}
			buf = append(buf, []byte(line)...)
		}

		results = append(results, CrawlResult{
			URL:        url,
			StatusCode: resp.StatusCode,
			Body:       buf,
		})
	}
	return results
}

// BuildIndex creates an inverted-index from crawl results.
// Triple-nested loop — high DeepNesting entropy.
func BuildIndex(results []CrawlResult) map[string][]string {
	index := make(map[string][]string)               // make() allocation

	for _, result := range results {                  // loop ×1
		words := tokenise(result.Body)
		for _, word := range words {                  // loop ×2
			for _, existing := range index[word] {    // loop ×3 — triple nesting
				if existing == result.URL {
					goto nextWord
				}
			}
			index[word] = append(index[word], result.URL)
		nextWord:
		}
	}
	return index
}

// tokenise splits a byte slice into lowercase words.
func tokenise(data []byte) []string {
	tokens := make([]string, 0, len(data)/5)         // allocation
	var cur []byte
	for _, b := range data {                          // loop ×1
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') {
			cur = append(cur, b)
		} else if len(cur) > 0 {
			tokens = append(tokens, string(cur))
			cur = cur[:0]
		}
	}
	return tokens
}

// RecursiveMerge merges nested map structures — recursive call.
func RecursiveMerge(dst, src map[string]interface{}) map[string]interface{} {
	for k, sv := range src {
		if dv, ok := dst[k]; ok {
			if dmap, ok2 := dv.(map[string]interface{}); ok2 {
				if smap, ok3 := sv.(map[string]interface{}); ok3 {
					dst[k] = RecursiveMerge(dmap, smap) // recursive call
					continue
				}
			}
		}
		dst[k] = sv
	}
	return dst
}

// SaveResults writes results to disk — blocking file I/O.
func SaveResults(results []CrawlResult, path string) error {
	f, err := os.Create(path)                        // BlockingIO
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	w := bufio.NewReader(os.Stdin)                   // BlockingIO

	var wg sync.WaitGroup                            // SyncContention
	for _, r := range results {                      // loop ×1
		wg.Add(1)
		go func(res CrawlResult) {
			defer wg.Done()
			// simulate blocking write
			time.Sleep(1 * time.Millisecond)          // BlockingIO
			_ = w
			fmt.Fprintf(f, "%s %d\n", res.URL, res.StatusCode)
		}(r)
	}
	wg.Wait()
	return nil
}

func main() {
	crawler := NewCrawler(10 * time.Second)
	urls := []string{
		"https://example.com",
		"https://example.org",
	}
	results := crawler.CrawlBatch(urls)
	index   := BuildIndex(results)
	_        = index
	if err  := SaveResults(results, "out.txt"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
