// Fetches public Groq + Gemini documentation, extracts model ids (Groq: full Free Plan limit columns),
// maps them to BiFrost-style provider/model strings, and writes YAML with source + limits in comments.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/lynn/claudia-gateway/internal/freecatalog"
)

const (
	defaultGroqURL   = "https://console.groq.com/docs/rate-limits"
	defaultGeminiURL = "https://ai.google.dev/gemini-api/docs/pricing"
	defaultOut       = "config/free-tier-catalog.snapshot.yaml"
)

func main() {
	outPath := flag.String("out", defaultOut, "output YAML path")
	groqURL := flag.String("groq-url", defaultGroqURL, "Groq rate limits documentation URL")
	geminiURL := flag.String("gemini-url", defaultGeminiURL, "Gemini API pricing documentation URL")
	intersectPath := flag.String("intersect", "", "optional BiFrost models list (JSON or YAML, OpenAI-style data[].id); keep only entries that fuzzy-match, align ids to catalog spelling")
	timeout := flag.Duration("timeout", 60*time.Second, "per-fetch timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout*3)
	defer cancel()

	client := &http.Client{Timeout: *timeout}

	groqBody, err := freecatalog.FetchURL(ctx, client, *groqURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "free-tier-catalog: fetch groq: %v\n", err)
		os.Exit(1)
	}
	gemBody, err := freecatalog.FetchURL(ctx, client, *geminiURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "free-tier-catalog: fetch gemini: %v\n", err)
		os.Exit(1)
	}

	var entries []freecatalog.Entry
	for _, row := range freecatalog.ParseGroqRateLimitRows(string(groqBody)) {
		bf := freecatalog.ToGroqBiFrost(row.SourceID)
		if bf == "" {
			continue
		}
		lim := row.GroqLimits
		entries = append(entries, freecatalog.Entry{
			Provider:   "groq",
			SourceID:   row.SourceID,
			BiFrostID:  bf,
			SourcePage: *groqURL,
			Groq:       &lim,
		})
	}
	for _, src := range freecatalog.ParseGeminiPricingFreeInputModels(string(gemBody)) {
		bf := freecatalog.ToGeminiBiFrost(src)
		if bf == "" {
			continue
		}
		entries = append(entries, freecatalog.Entry{
			Provider:   "gemini",
			SourceID:   src,
			BiFrostID:  bf,
			SourcePage: *geminiURL,
		})
	}
	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "free-tier-catalog: no models extracted (page layout may have changed)\n")
		os.Exit(1)
	}

	if *intersectPath != "" {
		raw, err := os.ReadFile(*intersectPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "free-tier-catalog: read intersect catalog: %v\n", err)
			os.Exit(1)
		}
		catIDs, err := freecatalog.ParseCatalogIntersect(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "free-tier-catalog: parse intersect catalog: %v\n", err)
			os.Exit(1)
		}
		entries = freecatalog.FilterEntriesByCatalog(entries, catIDs)
		if len(entries) == 0 {
			fmt.Fprintf(os.Stderr, "free-tier-catalog: intersect removed all models (check intersect file or parsing)\n")
			os.Exit(1)
		}
		entries = freecatalog.AlignEntriesToCatalog(entries, catIDs)
	}

	n, err := freecatalog.WriteCatalogYAML(*outPath, time.Now(), *groqURL, *geminiURL, entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "free-tier-catalog: write: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "free-tier-catalog: wrote %d models -> %s\n", n, *outPath)
}
