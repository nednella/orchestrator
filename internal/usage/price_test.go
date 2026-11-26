package usage

import "testing"

func TestPriceOfEachModel(t *testing.T) {
	tests := []struct {
		model string
		want  price
	}{
		{"claude-fable-5-2", price{8, 40, 0.2}},
		{"claude-fable-5-2-20261101", price{8, 40, 0.2}},
		{"claude-mythos-5-2", price{8, 40, 0.2}},
		{"claude-fable-5-1", price{10, 50, 0.25}},
		{"claude-mythos-5-1", price{10, 50, 0.25}},
		{"claude-fable-5", price{10, 50, 1}},
		{"claude-mythos-5", price{10, 50, 1}},
		{"claude-sonnet-5-5", price{2, 10, 0.1}},
		{"claude-sonnet-5", price{2, 10, 0.2}},
		{"claude-opus-5-5", price{4, 20, 0.2}},
		{Haiku, price{0.1, 0.5, 0.01}},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got, ok := priceOf(tt.model, 1000)
			if !ok || got != tt.want {
				t.Errorf("priceOf(%q) = %+v, %v; want %+v", tt.model, got, ok, tt.want)
			}
		})
	}
	if _, ok := priceOf("claude-unknown-9", 1000); ok {
		t.Error("an unknown model has a price")
	}
}
