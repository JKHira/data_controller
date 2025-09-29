package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/metadata"
	"github.com/trade-engine/data-controller/internal/restapi"
)

// EndpointInfo describes a Bitfinex configuration endpoint including refresh cadence.
type EndpointInfo struct {
	Endpoint    string
	FileName    string
	Description string
	TTL         time.Duration
}

var (
	essentialEndpoints = []EndpointInfo{
		{Endpoint: "pub:list:pair:exchange", FileName: "list_pair_exchange.json", Description: "Spot trading pairs", TTL: 45 * time.Minute},
		{Endpoint: "pub:list:pair:margin", FileName: "list_pair_margin.json", Description: "Margin trading pairs", TTL: 45 * time.Minute},
		{Endpoint: "pub:list:pair:futures", FileName: "list_pair_futures.json", Description: "Futures trading pairs", TTL: 45 * time.Minute},
		{Endpoint: "pub:info:pair", FileName: "info_pair.json", Description: "Pair trading parameters", TTL: 45 * time.Minute},
		{Endpoint: "pub:info:pair:futures", FileName: "info_pair_futures.json", Description: "Futures trading parameters", TTL: 45 * time.Minute},
		{Endpoint: "pub:spec:margin", FileName: "spec_margin.json", Description: "Margin specifications", TTL: 45 * time.Minute},
	}

	dailyEndpoints = []EndpointInfo{
		{Endpoint: "pub:map:currency:sym", FileName: "map_currency_sym.json", Description: "Symbol normalization map", TTL: 24 * time.Hour},
		{Endpoint: "pub:fees", FileName: "fees.json", Description: "Derivative fees", TTL: 24 * time.Hour},
		{Endpoint: "pub:map:currency:tx:fee", FileName: "map_currency_tx_fee.json", Description: "Withdrawal fees", TTL: 24 * time.Hour},
		{Endpoint: "pub:list:currency:margin", FileName: "list_currency_margin.json", Description: "Marginable currencies", TTL: 24 * time.Hour},
		{Endpoint: "pub:info:tx:status", FileName: "info_tx_status.json", Description: "Deposit/withdrawal status", TTL: 24 * time.Hour},
	}

	optionalEndpoints = []EndpointInfo{
		{Endpoint: "pub:map:currency:label", FileName: "map_currency_label.json", Description: "Currency display labels", TTL: 7 * 24 * time.Hour},
		{Endpoint: "pub:map:currency:unit", FileName: "map_currency_unit.json", Description: "Currency units", TTL: 7 * 24 * time.Hour},
		{Endpoint: "pub:map:currency:explorer", FileName: "map_currency_explorer.json", Description: "Block explorer URLs", TTL: 7 * 24 * time.Hour},
		{Endpoint: "pub:map:currency:pool", FileName: "map_currency_pool.json", Description: "Network pools", TTL: 7 * 24 * time.Hour},
		{Endpoint: "pub:list:competitions", FileName: "list_competitions.json", Description: "Competition listings", TTL: 7 * 24 * time.Hour},
		{Endpoint: "pub:map:currency:undl", FileName: "map_currency_undl.json", Description: "Underlying asset mappings", TTL: 7 * 24 * time.Hour},
	}
)

// EssentialEndpointInfos returns a copy of the essential and daily endpoint metadata.
func EssentialEndpointInfos() []EndpointInfo {
	return copyEndpointSlice(append(copyEndpointSlice(essentialEndpoints), dailyEndpoints...))
}

// OptionalEndpointInfos returns a copy of the optional endpoint metadata.
func OptionalEndpointInfos() []EndpointInfo {
	return copyEndpointSlice(optionalEndpoints)
}

func copyEndpointSlice(src []EndpointInfo) []EndpointInfo {
	out := make([]EndpointInfo, len(src))
	copy(out, src)
	return out
}

// ConfigRefreshManager coordinates metadata refresh, persistence, and state tracking.
type ConfigRefreshManager struct {
	logger    *zap.Logger
	client    *restapi.BitfinexClient
	state     *metadata.RefreshState
	statePath string
	lock      sync.Mutex
}

// NewConfigRefreshManager creates a refresh manager for Bitfinex configuration metadata.
func NewConfigRefreshManager(cfg *config.Config, logger *zap.Logger) (*ConfigRefreshManager, error) {
	rs, err := metadata.LoadRefreshState(cfg.StatePath)
	if err != nil {
		return nil, err
	}

	client := restapi.NewBitfinexClient(logger, cfg.Storage.BasePath)

	return &ConfigRefreshManager{
		logger:    logger,
		client:    client,
		state:     rs,
		statePath: cfg.StatePath,
	}, nil
}

// RefreshConfigEndpoints fetches the essential (45m) and daily (24h) metadata.
// When force is false, endpoints that are still fresh according to the recorded
// timestamps are skipped.
func (m *ConfigRefreshManager) RefreshConfigEndpoints(ctx context.Context, exchange string, force bool) ([]restapi.FetchResult, error) {
	specs := append(copyEndpointSlice(essentialEndpoints), dailyEndpoints...)
	return m.refresh(ctx, exchange, specs, force)
}

// RefreshOptionalEndpoints fetches the optional weekly metadata set.
func (m *ConfigRefreshManager) RefreshOptionalEndpoints(ctx context.Context, exchange string, force bool) ([]restapi.FetchResult, error) {
	return m.refresh(ctx, exchange, optionalEndpoints, force)
}

// EnsureFreshness checks essential+daily endpoints and refreshes those whose TTL
// has expired. Optional endpoints are not refreshed unless includeOptional is true.
func (m *ConfigRefreshManager) EnsureFreshness(ctx context.Context, exchange string, includeOptional bool) ([]restapi.FetchResult, error) {
	results, err := m.refresh(ctx, exchange, append(copyEndpointSlice(essentialEndpoints), dailyEndpoints...), false)
	if err != nil {
		return results, err
	}

	if includeOptional {
		optionalResults, errOpt := m.refresh(ctx, exchange, optionalEndpoints, false)
		results = append(results, optionalResults...)
		if errOpt != nil {
			return results, errOpt
		}
	}

	return results, nil
}

func (m *ConfigRefreshManager) refresh(ctx context.Context, exchange string, endpoints []EndpointInfo, force bool) ([]restapi.FetchResult, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	now := time.Now().UTC()
	executed := make([]restapi.FetchResult, 0, len(endpoints))

	for _, ep := range endpoints {
		if !force {
			if last, ok := m.state.LastRefresh(exchange, ep.Endpoint); ok && now.Sub(last) < ep.TTL {
				continue
			}
		}

		result := m.client.FetchAndStoreJSON(ctx, exchange, restapi.EndpointTask{Endpoint: ep.Endpoint, FileName: ep.FileName})
		if result.Success {
			m.state.Update(exchange, ep.Endpoint, result.Timestamp)
		}
		executed = append(executed, result)
	}

	if len(executed) > 0 {
		if err := m.state.Save(m.statePath); err != nil {
			m.logger.Warn("failed to save refresh state", zap.Error(err))
		}
	}

	return executed, nil
}

// SummarizeResults produces a compact summary describing which endpoints were
// refreshed successfully and which failed.
func SummarizeResults(exchange string, results []restapi.FetchResult) string {
	if len(results) == 0 {
		return ""
	}

	var success []string
	var failed []string
	for _, res := range results {
		if res.Success {
			success = append(success, res.Endpoint)
		} else if res.Error != "" {
			failed = append(failed, fmt.Sprintf("%s (%s)", res.Endpoint, res.Error))
		} else {
			failed = append(failed, res.Endpoint)
		}
	}

	parts := make([]string, 0, 2)
	if len(success) > 0 {
		parts = append(parts, fmt.Sprintf("updated %s", strings.Join(success, ", ")))
	}
	if len(failed) > 0 {
		parts = append(parts, fmt.Sprintf("failed %s", strings.Join(failed, "; ")))
	}

	if len(parts) == 0 {
		return ""
	}

	name := exchange
	if name == "" {
		name = "bitfinex"
	}
	if len(name) > 1 {
		name = strings.ToUpper(name[:1]) + strings.ToLower(name[1:])
	} else {
		name = strings.ToUpper(name)
	}

	return fmt.Sprintf("%s config %s", name, strings.Join(parts, " | "))
}
