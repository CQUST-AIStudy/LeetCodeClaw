package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"leetcodeclaw/internal/leetcode"
	"leetcodeclaw/internal/render"
)

func main() {
	var (
		slugsFlag   = flag.String("slugs", "", "comma-separated LeetCode title slugs, for example: two-sum,add-two-numbers")
		outDir      = flag.String("out", "output", "output directory")
		timeoutFlag = flag.Duration("timeout", 20*time.Second, "HTTP request timeout")
		retries     = flag.Int("retries", 2, "retry count for transient HTTP failures")
		delayFlag   = flag.Duration("delay", 500*time.Millisecond, "delay between problem crawls")
		formatFlag  = flag.String("format", "md,json", "output formats: md,json, md, or json")
	)
	flag.Parse()

	slugs := parseSlugs(*slugsFlag)
	if len(slugs) == 0 {
		fmt.Fprintln(os.Stderr, "missing required --slugs, for example: --slugs two-sum")
		os.Exit(2)
	}

	formats, err := render.ParseFormats(*formatFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --format: %v\n", err)
		os.Exit(2)
	}

	httpClient := &http.Client{Timeout: *timeoutFlag}
	client := leetcode.NewClient(httpClient, *retries)
	service := leetcode.NewProblemService(client)
	renderer := render.NewRenderer(*outDir, formats)

	result := leetcode.CrawlResult{
		OutputDir: *outDir,
	}

	ctx := context.Background()
	for i, slug := range slugs {
		if i > 0 && *delayFlag > 0 {
			time.Sleep(*delayFlag)
		}

		fmt.Printf("crawling %s...\n", slug)
		problem, err := service.CrawlProblem(ctx, slug)
		if err != nil {
			if cleanupErr := renderer.DeleteProblem(slug); cleanupErr != nil {
				fmt.Printf("  cleanup failed: %v\n", cleanupErr)
			}
			result.Failed = append(result.Failed, leetcode.FailedProblem{Slug: slug, Error: err.Error()})
			fmt.Printf("  failed: %v\n", err)
			continue
		}

		if err := leetcode.ValidateSolutionComplete(problem); err != nil {
			cleanupOutput(renderer, slug, problem.TitleSlug)
			message := err.Error()
			if warnings := leetcode.FormatProblemWarnings(problem.Errors); warnings != "" {
				message += "; " + warnings
			}
			result.Failed = append(result.Failed, leetcode.FailedProblem{Slug: problem.TitleSlug, Error: message})
			fmt.Printf("  failed: %s\n", message)
			continue
		}

		if err := renderer.WriteProblem(problem); err != nil {
			cleanupOutput(renderer, slug, problem.TitleSlug)
			result.Failed = append(result.Failed, leetcode.FailedProblem{Slug: slug, Error: err.Error()})
			fmt.Printf("  render failed: %v\n", err)
			continue
		}

		result.Succeeded = append(result.Succeeded, problem.TitleSlug)
		if len(problem.Errors) > 0 {
			fmt.Printf("  done with %d warning(s)\n", len(problem.Errors))
		} else {
			fmt.Println("  done")
		}
	}

	printSummary(result)
	if len(result.Failed) > 0 {
		os.Exit(1)
	}
}

func parseSlugs(raw string) []string {
	parts := strings.Split(raw, ",")
	slugs := make([]string, 0, len(parts))
	for _, part := range parts {
		slug := strings.TrimSpace(part)
		if slug != "" {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

func cleanupOutput(renderer *render.Renderer, slugs ...string) {
	seen := map[string]bool{}
	for _, slug := range slugs {
		slug = strings.TrimSpace(slug)
		key := strings.ToLower(slug)
		if slug == "" || seen[key] {
			continue
		}
		seen[key] = true
		if err := renderer.DeleteProblem(slug); err != nil {
			fmt.Printf("  cleanup failed for %s: %v\n", slug, err)
		}
	}
}

func printSummary(result leetcode.CrawlResult) {
	fmt.Println()
	fmt.Println("crawl summary")
	fmt.Printf("  output: %s\n", result.OutputDir)
	fmt.Printf("  succeeded: %d\n", len(result.Succeeded))
	for _, slug := range result.Succeeded {
		fmt.Printf("    - %s\n", slug)
	}
	fmt.Printf("  failed: %d\n", len(result.Failed))
	for _, item := range result.Failed {
		fmt.Printf("    - %s: %s\n", item.Slug, item.Error)
	}
}
