package domain

type CurrencyType int

const (
	UnknownCurrencyType CurrencyType = iota
	BaseCurrencyType
	QuoteCurrencyType
)

type CurrencyProduct struct {
	ID             string `json:"id"`
	BaseCurrency   string `json:"baseCurrency"`
	QuoteCurrency  string `json:"quoteCurrency"`
	BaseMinSize    string `json:"baseMinSize"`            // TODO: implement
	BaseMaxSize    string `json:"baseMaxSize"`            // TODO: implement
	QuoteMinSize   string `json:"QuoteMinSize,omitempty"` // TODO: implement
	QuoteMaxSize   string `json:"QuoteMaxSize,omitempty"` // TODO: implement
	QuoteIncrement string `json:"quoteIncrement"`         // TODO: implement
	BaseScale      int    `json:"baseScale"`              // TODO: implement
	QuoteScale     int    `json:"quoteScale"`             // TODO: implement
}

type CurrencyUseCase interface {
	GetBaseCurrencyID() int
	GetQuoteCurrencyID() int
	GetCurrencyTypeByName(name string) (CurrencyType, error)
	GetCurrencyUpperNameByType(currencyType CurrencyType) (string, error)
	GetProductID() string
	GetProduct() *CurrencyProduct
}
