package matching

import "github.com/superj80820/system-design/domain"

type directionEnum domain.DirectionEnum

func (d directionEnum) compare(a, b *orderKey) int {
	switch domain.DirectionEnum(d) { //TODO: think performance
	case domain.DirectionBuy:
		cmp := b.price.Cmp(a.price)
		if cmp == 0 {
			if a.sequenceId > b.sequenceId {
				return 1
			} else if a.sequenceId < b.sequenceId {
				return -1
			} else {
				return 0
			}
		}
		return cmp
	case domain.DirectionSell:
		cmp := a.price.Cmp(b.price)
		if cmp == 0 {
			if a.sequenceId > b.sequenceId {
				return 1
			} else if a.sequenceId < b.sequenceId {
				return -1
			} else {
				return 0
			}
		}
		return cmp
	case domain.DirectionUnknown:
		panic("unknown direction")
	default:
		panic("unknown direction")
	}
}
