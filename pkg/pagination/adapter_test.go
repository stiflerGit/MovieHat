package pagination

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ascending builds a source of n items: 0, 1, ... n-1.
func ascending(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

// newAdapter serves source in fixed-size pages, mimicking a real remote source.
func newAdapter(source []int, sourcePageSize int) *Adapter[int] {
	return NewAdapter(sourcePageSize, func(_ context.Context, page int) ([]int, error) {
		start := (page - 1) * sourcePageSize
		if start < 0 || start >= len(source) {
			return nil, nil
		}
		return source[start:min(start+sourcePageSize, len(source))], nil
	})
}

func TestAdapter_Fetch(t *testing.T) {
	tests := []struct {
		name        string
		adapter     *Adapter[int]
		offset      int
		pageSize    int
		wantItems   []int
		wantHasMore bool
		wantErr     bool
	}{
		{
			name:        "window inside a single source page",
			adapter:     newAdapter(ascending(25), 10),
			offset:      3,
			pageSize:    3,
			wantItems:   []int{3, 4, 5},
			wantHasMore: true,
		},
		{
			name:        "window spans two source pages",
			adapter:     newAdapter(ascending(25), 10),
			offset:      0,
			pageSize:    15,
			wantItems:   []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
			wantHasMore: true,
		},
		{
			name:        "window spans two pages with both offsets",
			adapter:     newAdapter(ascending(50), 10),
			offset:      15,
			pageSize:    15,
			wantItems:   []int{15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29},
			wantHasMore: true,
		},
		{
			name:        "window aligned to a full source page",
			adapter:     newAdapter(ascending(25), 10),
			offset:      10,
			pageSize:    10,
			wantItems:   []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19},
			wantHasMore: true,
		},
		{
			name:        "window ends on the last short source page",
			adapter:     newAdapter(ascending(25), 10),
			offset:      20,
			pageSize:    10,
			wantItems:   []int{20, 21, 22, 23, 24},
			wantHasMore: false,
		},
		{
			name:        "window past the last source page",
			adapter:     newAdapter(ascending(5), 10),
			offset:      5,
			pageSize:    5,
			wantItems:   []int{},
			wantHasMore: false,
		},
		{
			name:        "requested page beyond available data",
			adapter:     newAdapter(ascending(25), 10),
			offset:      90,
			pageSize:    10,
			wantItems:   []int{},
			wantHasMore: false,
		},
		{
			name:        "short middle page ends the stream early",
			adapter:     newAdapter(ascending(15), 10),
			offset:      0,
			pageSize:    25,
			wantItems:   []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
			wantHasMore: false,
		},
		{
			name:        "window covers the whole short source",
			adapter:     newAdapter(ascending(25), 10),
			offset:      0,
			pageSize:    25,
			wantItems:   ascending(25),
			wantHasMore: false,
		},
		{
			name:        "window covers a source whose size is a multiple of the page size",
			adapter:     newAdapter(ascending(20), 10),
			offset:      0,
			pageSize:    20,
			wantItems:   ascending(20),
			wantHasMore: false,
		},
		{
			name:        "window ends on the last item of a full source page",
			adapter:     newAdapter(ascending(20), 10),
			offset:      10,
			pageSize:    10,
			wantItems:   []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19},
			wantHasMore: false,
		},
		{
			name:     "offset must be non-negative",
			adapter:  newAdapter(ascending(25), 10),
			offset:   -1,
			pageSize: 10,
			wantErr:  true,
		},
		{
			name:     "pageSize must be positive",
			adapter:  newAdapter(ascending(25), 10),
			offset:   0,
			pageSize: 0,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotItems, gotHasMore, err := tt.adapter.Fetch(t.Context(), tt.offset, tt.pageSize)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantItems, gotItems)
			assert.Equal(t, tt.wantHasMore, gotHasMore)
		})
	}
}
