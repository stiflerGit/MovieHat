package pagination

import (
	"context"
	"fmt"
)

// Adapter maps a logical page/perPage window onto a source with a fixed page
// size, fetching and trimming the source pages that cover it.
type Adapter[T any] struct {
	// sourcePageSize is the fixed number of items each source page holds.
	sourcePageSize int
	// fetch returns the items of a 1-indexed source page.
	fetch func(ctx context.Context, page int) ([]T, error)
}

// NewAdapter returns an Adapter that reads 1-indexed pages of up to sourcePageSize
// items from fetch. The final page may be shorter.
func NewAdapter[T any](
	sourcePageSize int,
	fetch func(ctx context.Context, page int) ([]T, error),
) *Adapter[T] {
	return &Adapter[T]{
		sourcePageSize: sourcePageSize,
		fetch:          fetch,
	}
}

// Fetch returns up to perPage items from the 1-indexed logical page page. The
// window may span several source pages.
//
// hasMore reports whether items may exist past the window. It is false once the
// source is exhausted
func (a *Adapter[T]) Fetch(ctx context.Context, offset, pageSize int) (items []T, hasMore bool, err error) {
	if offset < 0 {
		return nil, false, fmt.Errorf("offset must be >= 0, got: %d", offset)
	}
	if pageSize < 1 {
		return nil, false, fmt.Errorf("pageSize must be >= 1, got pageSize=%d", pageSize)
	}

	firstItem := offset
	lastItem := offset + pageSize - 1

	startSrcPage := firstItem/a.sourcePageSize + 1
	endSrcPage := lastItem/a.sourcePageSize + 1

	firstIndexInPage := firstItem % a.sourcePageSize
	lastIndexInPage := lastItem % a.sourcePageSize

	lastSrcPage := 0
	lastSrcPageLen := a.sourcePageSize

	items = make([]T, 0, pageSize)
	for srcPage := startSrcPage; srcPage <= endSrcPage && lastSrcPageLen == a.sourcePageSize; srcPage++ {
		elems, err := a.fetch(ctx, srcPage)
		if err != nil {
			return items, false, fmt.Errorf("getting page %d: %w", srcPage, err)
		}
		lastSrcPageLen = len(elems)

		if srcPage == endSrcPage {
			elems = elems[:min(len(elems), lastIndexInPage+1)]
		}
		if srcPage == startSrcPage {
			elems = elems[min(len(elems), firstIndexInPage):]
		}

		items = append(items, elems...)
		lastSrcPage = srcPage
	}

	hasMore, err = a.hasMoreItems(ctx, offset, pageSize, lastSrcPage, lastSrcPageLen)
	if err != nil {
		return items, hasMore, fmt.Errorf("a.hasMoreItems: %w", err)
	}

	return items, hasMore, nil
}

func (a *Adapter[T]) hasMoreItems(ctx context.Context, offset, pageSize, lastSrcPage, lastSrcPageLen int) (bool, error) {
	lastItem := offset + pageSize - 1
	endSrcPage := lastItem/a.sourcePageSize + 1
	lastFetchedItem := (lastSrcPage-1)*a.sourcePageSize + lastSrcPageLen - 1

	// we already have the nextItem -> return true
	if lastItem+1 <= lastFetchedItem {
		return true, nil
	}

	// the only case in which we need to fetch the next page is when
	// lastItem is exactly equal to the last fetched item
	if lastItem == lastFetchedItem {
		elems, err := a.fetch(ctx, endSrcPage+1)
		if err != nil {
			return false, fmt.Errorf("getting page %d: %w", endSrcPage+1, err)
		}

		return len(elems) > 0, nil
	}

	return false, nil
}
