package currency

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type currencyUseCase struct {
	currencyProduct *domain.CurrencyProduct
}

func CreateCurrencyUseCase(currencyProduct *domain.CurrencyProduct) domain.CurrencyUseCase {
	return &currencyUseCase{
		currencyProduct: currencyProduct,
	}
}

func (c *currencyUseCase) GetCurrencyTypeByName(name string) (domain.CurrencyType, error) {
	upperName := strings.ToUpper(name)
	if upperName == c.currencyProduct.BaseCurrency {
		return domain.BaseCurrencyType, nil
	} else if upperName == c.currencyProduct.QuoteCurrency {
		return domain.QuoteCurrencyType, nil
	}
	return domain.UnknownCurrencyType, domain.ErrNoData
}

func (c *currencyUseCase) GetCurrencyUpperNameByType(currencyType domain.CurrencyType) (string, error) {
	switch currencyType {
	case domain.BaseCurrencyType:
		return c.currencyProduct.BaseCurrency, nil
	case domain.QuoteCurrencyType:
		return c.currencyProduct.QuoteCurrency, nil
	default:
		return "", errors.Wrap(domain.ErrNoData, "unknown currency type")
	}
}

func (c *currencyUseCase) GetProductID() string {
	return c.currencyProduct.ID
}

func (c *currencyUseCase) GetProduct() *domain.CurrencyProduct {
	return c.currencyProduct
}

func (c *currencyUseCase) GetBaseCurrencyID() int {
	return int(domain.BaseCurrencyType)
}

func (*currencyUseCase) GetQuoteCurrencyID() int {
	return int(domain.QuoteCurrencyType)
}
