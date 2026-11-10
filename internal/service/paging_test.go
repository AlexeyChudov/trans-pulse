package service_test

import (
	"testing"

	"github.com/AlexeyChudov/trans-pulse/internal/service"
)

func TestNormalizePage(t *testing.T) {
	cases := []struct{ limit, offset, wantLimit, wantOffset int }{
		{0, 0, 50, 0},
		{10, 5, 10, 5},
		{1000, 0, 200, 0},
		{-3, -1, 50, 0},
	}
	for _, c := range cases {
		l, o := service.NormalizePage(c.limit, c.offset)
		if l != c.wantLimit || o != c.wantOffset {
			t.Errorf("NormalizePage(%d,%d) = (%d,%d), ожидалось (%d,%d)", c.limit, c.offset, l, o, c.wantLimit, c.wantOffset)
		}
	}
}
