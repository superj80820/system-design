package memoryandredis

import (
	"github.com/shopspring/decimal"
	"github.com/superj80820/system-design/domain"
)

type directionEnum domain.DirectionEnum

func (d directionEnum) compare(a, b interface{}) int {
	aPrice := a.(decimal.Decimal)
	bPrice := b.(decimal.Decimal)

	switch domain.DirectionEnum(d) {
	case domain.DirectionSell:
		return aPrice.Cmp(bPrice) // TODO: think
	case domain.DirectionBuy:
		return bPrice.Cmp(aPrice)
	case domain.DirectionUnknown:
		panic("unknown direction")
	default:
		panic("unknown direction")
	}
}
