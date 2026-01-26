package pricesync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Service orchestrates the price sync process
type Service struct {
	fetcher *Fetcher
	writer  *Writer
	logger  *zap.Logger
	config  *Config

	// State tracking
	stateMu           sync.Mutex
	state             map[string]*marketState
	consecutiveErrors int
	lastErrorTime     time.Time
	totalErrors       int64
	totalSuccess      int64
}

type marketState struct {
	lastPrice  float64
	lastInsert time.Time
}

// NewService creates a new Service instance
func NewService(fetcher *Fetcher, writer *Writer, logger *zap.Logger, config *Config) *Service {
	return &Service{
		fetcher: fetcher,
		writer:  writer,
		logger:  logger,
		config:  config,
		state:   make(map[string]*marketState),
	}
}

// Start begins the polling loop
func (s *Service) Start(ctx context.Context) error {
	s.logger.Info("Starting price sync service",
		zap.String("endpoint", s.config.MarketEndpoint),
		zap.Duration("poll_interval", s.config.PollInterval),
		zap.Duration("heartbeat_interval", s.config.HeartbeatInterval),
	)

	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	heartbeatTicker := time.NewTicker(s.config.HeartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Context canceled, stopping price sync service")
			return ctx.Err()
		case <-ticker.C:
			s.pollOnce(ctx)
		case <-heartbeatTicker.C:
			s.logHealthStatus()
		}
	}
}

// pollOnce performs a single poll cycle
func (s *Service) pollOnce(ctx context.Context) {
	fetchStart := time.Now()

	prices, err := s.fetcher.FetchMarketPrices(ctx)
	fetchDuration := time.Since(fetchStart)

	if err != nil {
		s.recordError(err)
		return
	}

	if fetchDuration > 1*time.Second {
		s.logger.Warn("Slow HTTP request",
			zap.Duration("duration", fetchDuration),
			zap.String("url", s.config.MarketEndpoint),
		)
	}

	s.logger.Debug("Fetched market prices",
		zap.Int("total_markets", len(prices)),
		zap.Duration("duration", fetchDuration),
	)

	if len(prices) == 0 {
		s.logger.Warn("No perp markets with mark prices found")
		return
	}

	// Filter prices that need insertion
	var pricesToInsert []MarketPrice
	now := time.Now()

	s.stateMu.Lock()
	for _, p := range prices {
		st, ok := s.state[p.MarketID]
		if !ok {
			st = &marketState{}
			s.state[p.MarketID] = st
		}

		// Insert if price changed or it's been a while since last insert
		shouldInsert := s.shouldInsert(st.lastPrice, p.Price) ||
			st.lastInsert.IsZero() ||
			now.Sub(st.lastInsert) >= s.config.HeartbeatInterval

		if shouldInsert {
			pricesToInsert = append(pricesToInsert, p)
			st.lastInsert = now
		}
		st.lastPrice = p.Price
	}
	s.stateMu.Unlock()

	if len(pricesToInsert) == 0 {
		s.logger.Debug("No price changes detected, skipping insert",
			zap.Int("markets_checked", len(prices)),
		)
		s.recordSuccess()
		return
	}

	// Write prices to database
	inserted, errors := s.writer.WritePrices(ctx, pricesToInsert)

	if errors > 0 {
		s.recordError(fmt.Errorf("%d database insert errors occurred (succeeded=%d, failed=%d)",
			errors, inserted, errors))
		s.tryReconnectDB(ctx)
	} else {
		s.recordSuccess()
		skippedCount := len(prices) - len(pricesToInsert)
		if skippedCount > 0 {
			s.logger.Info("Successfully inserted market prices",
				zap.Int("inserted", inserted),
				zap.Int("skipped", skippedCount),
				zap.Int("total_markets", len(prices)),
			)
		} else {
			s.logger.Info("Successfully inserted market prices",
				zap.Int("inserted", inserted),
				zap.Int("total_markets", len(prices)),
			)
		}
	}
}

// shouldInsert determines if a price change warrants insertion
func (s *Service) shouldInsert(prev, next float64) bool {
	if prev == 0 {
		return true
	}
	return next != prev
}

// recordError tracks errors and logs them with categorization
func (s *Service) recordError(err error) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	s.consecutiveErrors++
	s.totalErrors++
	now := time.Now()
	timeSinceLastError := now.Sub(s.lastErrorTime)
	s.lastErrorTime = now

	// Categorize error type
	errStr := err.Error()
	var category string
	switch {
	case containsAny(errStr, "HTTP_REQUEST_FAILED", "HTTP_REQUEST_CREATE_ERROR", "HTTP_INVALID_STATUS"):
		category = "NETWORK"
	case containsAny(errStr, "database insert", "Database insert failed"):
		category = "DATABASE"
	case containsAny(errStr, "JSON_DECODE_ERROR"):
		category = "PARSING"
	default:
		category = "UNKNOWN"
	}

	s.logger.Error("Price sync error",
		zap.String("category", category),
		zap.Int64("total_errors", s.totalErrors),
		zap.Int("consecutive_errors", s.consecutiveErrors),
		zap.Duration("time_since_last_error", timeSinceLastError),
		zap.Error(err),
	)

	// Progressive alerting
	if s.consecutiveErrors == 3 {
		s.logger.Warn("3 consecutive errors detected", zap.String("category", category))
	} else if s.consecutiveErrors == 5 {
		s.logger.Warn("5 consecutive errors - system may be unhealthy", zap.String("category", category))
	} else if s.consecutiveErrors == 10 {
		s.logger.Error("10 consecutive errors - manual intervention may be required", zap.String("category", category))
	} else if s.consecutiveErrors == 20 {
		s.logger.Error("20 consecutive errors - check network/database connectivity immediately!", zap.String("category", category))
	}
}

// recordSuccess resets consecutive error counter
func (s *Service) recordSuccess() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.consecutiveErrors > 0 {
		timeSinceError := time.Since(s.lastErrorTime)
		s.logger.Info("Recovered from errors",
			zap.Int("consecutive_errors", s.consecutiveErrors),
			zap.Duration("time_since_error", timeSinceError),
		)
		s.consecutiveErrors = 0
	}
	s.totalSuccess++
}

// tryReconnectDB attempts to reconnect to the database
func (s *Service) tryReconnectDB(ctx context.Context) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	// Only try reconnect if we have consecutive errors
	if s.consecutiveErrors < 3 {
		return
	}

	s.logger.Info("Attempting database reconnection",
		zap.Int("consecutive_errors", s.consecutiveErrors),
	)

	if err := s.writer.Reconnect(ctx); err != nil {
		s.logger.Error("Database reconnection failed", zap.Error(err))
		return
	}

	s.logger.Info("Database connection re-established successfully",
		zap.Int("cleared_errors", s.consecutiveErrors),
	)
	s.consecutiveErrors = 0
}

// logHealthStatus logs current health and statistics
func (s *Service) logHealthStatus() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	marketCount := len(s.state)
	successRate := float64(0)
	if s.totalSuccess+s.totalErrors > 0 {
		successRate = float64(s.totalSuccess) / float64(s.totalSuccess+s.totalErrors) * 100
	}

	s.logger.Info("Heartbeat",
		zap.Int("markets", marketCount),
		zap.Int64("success", s.totalSuccess),
		zap.Int64("errors", s.totalErrors),
		zap.Float64("success_rate", successRate),
		zap.Int("consecutive_errors", s.consecutiveErrors),
	)

	if s.consecutiveErrors > 10 {
		s.logger.Warn("High consecutive error count",
			zap.Int("consecutive_errors", s.consecutiveErrors),
		)
	}
}

// Stop gracefully stops the service
func (s *Service) Stop() error {
	s.logger.Info("Stopping price sync service")
	if s.writer != nil {
		return s.writer.Close()
	}
	return nil
}

// containsAny checks if string contains any of the substrings
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
