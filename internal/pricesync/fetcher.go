package pricesync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MarketPrice represents a market price data point
type MarketPrice struct {
	MarketID  string
	Price     float64
	Timestamp time.Time
}

// Fetcher handles fetching market prices from HTTP endpoint
type Fetcher struct {
	baseURL string
	client  *http.Client
}

// NewFetcher creates a new Fetcher instance
func NewFetcher(baseURL string, timeout time.Duration) *Fetcher {
	return &Fetcher{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// FetchMarketPrices fetches prices from the /markets endpoint
// Returns only perp markets that have a mark_price
func (f *Fetcher) FetchMarketPrices(ctx context.Context) ([]MarketPrice, error) {
	url := fmt.Sprintf("%s/markets", f.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTP_REQUEST_CREATE_ERROR | URL=%s | Error: %w", url, err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP_REQUEST_FAILED | URL=%s | Timeout=%v | Error: %w", url, f.client.Timeout, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP_INVALID_STATUS | Status=%d | URL=%s", resp.StatusCode, url)
	}

	var markets []struct {
		UUID      string `json:"uuid"`
		Kind      string `json:"kind"`
		PerpState *struct {
			MarkPrice          *float64 `json:"mark_price"`
			MarkPriceTimestamp *int64   `json:"mark_price_timestamp"`
		} `json:"perp_state"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&markets); err != nil {
		return nil, fmt.Errorf("JSON_DECODE_ERROR | URL=%s | ContentType=%s | Error: %w",
			url, resp.Header.Get("Content-Type"), err)
	}

	var prices []MarketPrice
	now := time.Now().UTC()

	for _, m := range markets {
		// Only include perp markets with mark_price
		if m.Kind != "perp" || m.PerpState == nil || m.PerpState.MarkPrice == nil {
			continue
		}

		ts := now
		if m.PerpState.MarkPriceTimestamp != nil {
			// Engine timestamp is in seconds since epoch
			ts = time.Unix(*m.PerpState.MarkPriceTimestamp, 0).UTC()
		}

		prices = append(prices, MarketPrice{
			MarketID:  m.UUID,
			Price:     *m.PerpState.MarkPrice,
			Timestamp: ts,
		})
	}

	return prices, nil
}
