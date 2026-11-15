package waitlist

import (
	"errors"
	"math"
)

// ErrQuantity is a quantity outside 1 to MaxQuantity, not a whole number, or
// sent for a product that takes none.
var ErrQuantity = errors.New("waitlist: invalid quantity")

// MaxQuantity is the most charms one Pastille pre-order may ask for.
const MaxQuantity = 10

// quantityProduct is the only list whose entries may carry a quantity.
const quantityProduct = "pastille"

// validQuantity reports whether q is absent, or a whole number from 1 to
// MaxQuantity on the one product that takes a quantity.
func validQuantity(q *float64, product string) bool {
	if q == nil {
		return true
	}
	return product == quantityProduct && *q == math.Trunc(*q) && *q >= 1 && *q <= MaxQuantity
}

// storedQuantity converts a validated quantity to its column type.
func storedQuantity(q *float64) *int16 {
	if q == nil {
		return nil
	}
	n := int16(*q)
	return &n
}
