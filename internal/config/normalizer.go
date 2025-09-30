package config

import (
	"fmt"
	"regexp"
	"strings"
)

// Normalizer handles currency and pair normalization for consistent internal representation
type Normalizer struct {
	currencyLabels map[string]string // Short symbol -> Full name (e.g., BTC -> Bitcoin)
	pairFormat     string             // Internal format (e.g., "base-quote")
	uppercase      bool
}

// NewNormalizer creates a normalizer with currency label mappings
func NewNormalizer(currencyLabels map[string]string) *Normalizer {
	return &Normalizer{
		currencyLabels: currencyLabels,
		pairFormat:     "base-quote", // internal standard
		uppercase:      true,
	}
}

// NormalizedPair represents a parsed trading pair
type NormalizedPair struct {
	Original     string // Original format (e.g., "BTCUSD", "tBTCUSD")
	Base         string // Base currency (e.g., "BTC")
	Quote        string // Quote currency (e.g., "USD")
	MarketType   string // "spot", "perp", "futures", "margin"
	Prefix       string // "t", "f", etc.
	Internal     string // Normalized internal format (e.g., "BTC-USD")
	BaseFull     string // Full base name (e.g., "Bitcoin")
	QuoteFull    string // Full quote name (e.g., "US Dollar")
	IsTrading    bool   // true if trading pair (t prefix)
	IsFunding    bool   // true if funding currency (f prefix)
	ContractSize string // For futures (e.g., "F0")
}

// NormalizePair converts exchange-specific pair format to internal format
func (n *Normalizer) NormalizePair(pairStr string) (*NormalizedPair, error) {
	original := pairStr
	pair := &NormalizedPair{
		Original: original,
	}

	// Handle Bitfinex prefixes
	if strings.HasPrefix(pairStr, "t") {
		pair.IsTrading = true
		pair.Prefix = "t"
		pairStr = strings.TrimPrefix(pairStr, "t")
	} else if strings.HasPrefix(pairStr, "f") {
		pair.IsFunding = true
		pair.Prefix = "f"
		pairStr = strings.TrimPrefix(pairStr, "f")
		// Funding currencies are single currency codes
		pair.Base = pairStr
		pair.Quote = "USD" // Default for funding
	}

	// Parse pair components (skip if already set for funding)
	if pair.Base == "" {
		if err := n.parsePairComponents(pairStr, pair); err != nil {
			return nil, fmt.Errorf("parse pair %s: %w", original, err)
		}
	}

	// Determine market type
	pair.MarketType = n.determineMarketType(pair)

	// Build internal format
	if n.uppercase {
		pair.Base = strings.ToUpper(pair.Base)
		pair.Quote = strings.ToUpper(pair.Quote)
	}
	pair.Internal = fmt.Sprintf("%s-%s", pair.Base, pair.Quote)

	// Add full names if available
	if fullName, ok := n.currencyLabels[pair.Base]; ok {
		pair.BaseFull = fullName
	} else {
		pair.BaseFull = pair.Base
	}
	if fullName, ok := n.currencyLabels[pair.Quote]; ok {
		pair.QuoteFull = fullName
	} else {
		pair.QuoteFull = pair.Quote
	}

	return pair, nil
}

// parsePairComponents splits pair string into base/quote
func (n *Normalizer) parsePairComponents(pairStr string, pair *NormalizedPair) error {
	// Check for colon separator (e.g., "BTC:USD", "AVAX:BTC")
	if strings.Contains(pairStr, ":") {
		parts := strings.Split(pairStr, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid colon-separated pair: %s", pairStr)
		}
		pair.Base = parts[0]
		pair.Quote = parts[1]
		return nil
	}

	// Check for futures notation (e.g., "BTCF0:USTF0")
	futuresRe := regexp.MustCompile(`([A-Z]+)(F\d+)`)
	if matches := futuresRe.FindStringSubmatch(pairStr); matches != nil {
		pair.Base = matches[1]
		pair.ContractSize = matches[2]
		// Try to extract quote if present
		remaining := strings.TrimPrefix(pairStr, matches[0])
		if remaining != "" {
			pair.Quote = remaining
		} else {
			pair.Quote = "USD" // default
		}
		return nil
	}

	// Common patterns: 3-letter base + 3-4 letter quote
	// Try USD first (most common)
	if strings.HasSuffix(pairStr, "USD") && len(pairStr) > 3 {
		pair.Base = pairStr[:len(pairStr)-3]
		pair.Quote = "USD"
		return nil
	}
	if strings.HasSuffix(pairStr, "USDT") && len(pairStr) > 4 {
		pair.Base = pairStr[:len(pairStr)-4]
		pair.Quote = "USDT"
		return nil
	}
	if strings.HasSuffix(pairStr, "UST") && len(pairStr) > 3 {
		pair.Base = pairStr[:len(pairStr)-3]
		pair.Quote = "UST"
		return nil
	}

	// Try other common quote currencies
	commonQuotes := []string{"EUR", "GBP", "JPY", "BTC", "ETH"}
	for _, quote := range commonQuotes {
		if strings.HasSuffix(pairStr, quote) && len(pairStr) > len(quote) {
			pair.Base = pairStr[:len(pairStr)-len(quote)]
			pair.Quote = quote
			return nil
		}
	}

	// Fallback: assume first 3 chars are base, rest is quote
	if len(pairStr) >= 6 {
		pair.Base = pairStr[:3]
		pair.Quote = pairStr[3:]
		return nil
	}

	return fmt.Errorf("unable to parse pair: %s", pairStr)
}

// determineMarketType identifies the market type based on pair characteristics
func (n *Normalizer) determineMarketType(pair *NormalizedPair) string {
	if pair.IsFunding {
		return "funding"
	}
	if pair.ContractSize != "" {
		return "futures"
	}
	// Could add more sophisticated detection here
	return "spot"
}

// DenormalizePair converts internal format back to exchange-specific format
func (n *Normalizer) DenormalizePair(internal string, exchange string) (string, error) {
	parts := strings.Split(internal, "-")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid internal pair format: %s", internal)
	}

	base := parts[0]
	quote := parts[1]

	switch strings.ToLower(exchange) {
	case "bitfinex":
		// Bitfinex uses tBASEQUOTE format for trading
		return fmt.Sprintf("t%s%s", base, quote), nil
	case "binance":
		// Binance uses BASEQUOTE format
		return fmt.Sprintf("%s%s", base, quote), nil
	default:
		return fmt.Sprintf("%s%s", base, quote), nil
	}
}

// GetCurrencyFullName returns the full name for a currency symbol
func (n *Normalizer) GetCurrencyFullName(symbol string) string {
	if fullName, ok := n.currencyLabels[strings.ToUpper(symbol)]; ok {
		return fullName
	}
	return symbol
}

// LoadCurrencyLabelsFromMap loads currency labels from a map
func (n *Normalizer) LoadCurrencyLabelsFromMap(labels [][2]string) {
	n.currencyLabels = make(map[string]string)
	for _, pair := range labels {
		if len(pair) >= 2 {
			n.currencyLabels[pair[0]] = pair[1]
		}
	}
}