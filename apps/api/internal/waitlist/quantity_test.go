package waitlist

import (
	"errors"
	"testing"
)

func withQuantity(product string, q float64) Entry {
	in := entry("p@example.com")
	in.Product = product
	in.Quantity = &q
	return in
}

func TestValidateQuantity(t *testing.T) {
	refused := map[string]Entry{
		"on scribe": withQuantity("scribe", 1),
		"zero":      withQuantity("pastille", 0),
		"eleven":    withQuantity("pastille", 11),
		"fraction":  withQuantity("pastille", 2.5),
	}
	for name, in := range refused {
		if _, err := Validate(in); !errors.Is(err, ErrQuantity) {
			t.Errorf("%s: got %v, want ErrQuantity", name, err)
		}
	}
	for _, q := range []float64{1, 10} {
		if _, err := Validate(withQuantity("pastille", q)); err != nil {
			t.Errorf("quantity %v refused: %v", q, err)
		}
	}
	if _, err := Validate(entry("p@example.com")); err != nil {
		t.Errorf("no quantity refused: %v", err)
	}
}
