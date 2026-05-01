package web

import (
	"context"
	"fmt"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
)

const (
	webMaxPageNumber    = 1000
	webPageWindowRadius = 2
)

func clampWebPageNumber(page int) int {
	if page < 1 {
		return 1
	}
	if page > webMaxPageNumber {
		return webMaxPageNumber
	}
	return page
}

func buildPaginationData(page int, hasMore bool, totalPages ...int) *PaginationData {
	page = clampWebPageNumber(page)
	pagination := &PaginationData{Page: page}
	if len(totalPages) > 0 && totalPages[0] > 0 {
		maxPage := totalPages[0]
		if maxPage > webMaxPageNumber {
			maxPage = webMaxPageNumber
		}
		if page > maxPage {
			page = maxPage
			pagination.Page = page
		}
		if page > 1 {
			pagination.PrevPage = page - 1
		}
		if page < maxPage {
			pagination.NextPage = page + 1
		}
		pagination.Pages = make([]int, 0, maxPage)
		for current := 1; current <= maxPage; current++ {
			pagination.Pages = append(pagination.Pages, current)
		}
		pagination.TotalPages = maxPage
		return pagination
	}
	if page > 1 {
		pagination.PrevPage = page - 1
	}
	if hasMore && page < webMaxPageNumber {
		pagination.NextPage = page + 1
	}
	start := page - webPageWindowRadius
	if start < 1 {
		start = 1
	}
	end := page
	if hasMore {
		end = page + webPageWindowRadius
		if end > webMaxPageNumber {
			end = webMaxPageNumber
		}
	}
	pagination.Pages = make([]int, 0, end-start+1)
	for current := start; current <= end; current++ {
		pagination.Pages = append(pagination.Pages, current)
	}
	return pagination
}

func totalPagesFromCount(totalCount, limit int) int {
	if totalCount <= 0 || limit <= 0 {
		return 0
	}
	totalPages := (totalCount + limit - 1) / limit
	if totalPages > webMaxPageNumber {
		return webMaxPageNumber
	}
	return totalPages
}

func totalPagesFromPostList(list *model.PostList) int {
	if list == nil {
		return 0
	}
	return totalPagesFromCount(list.TotalCount, list.Limit)
}

func listPageURL(data any, page int) string {
	page = clampWebPageNumber(page)
	kind := ""
	baseURL := ""
	limit := 0
	sortValue := ""
	windowValue := ""
	query := ""
	statusValue := ""
	switch value := data.(type) {
	case PageData:
		kind = value.Kind
		baseURL = value.ListBaseURL
		limit = value.ListLimit
		sortValue = value.SortValue
		windowValue = value.WindowValue
		query = value.Query
		statusValue = value.StatusValue
	case map[string]any:
		kind, _ = value["Kind"].(string)
		baseURL, _ = value["ListBaseURL"].(string)
		limit = intFromAny(value["ListLimit"])
		sortValue, _ = value["SortValue"].(string)
		windowValue, _ = value["WindowValue"].(string)
		query, _ = value["Query"].(string)
		statusValue, _ = value["StatusValue"].(string)
	default:
		return "/"
	}
	params := []any{"limit", limit, "page", page}
	switch kind {
	case "feed":
		params = append(params,
			"sort", sortValue,
			"window", windowValue,
		)
	case "search":
		params = append(params,
			"q", query,
			"sort", sortValue,
			"window", windowValue,
		)
	case "admin-reports":
		params = append(params, "status", statusValue)
	}
	return pageURL(baseURL, params...)
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		return int(typed)
	case uint64:
		return int(typed)
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return 0
		}
		var value int
		if _, err := fmt.Sscanf(typed, "%d", &value); err == nil {
			return value
		}
	}
	return 0
}

func loadSequentialPage[T any, Token any](
	ctx context.Context,
	page int,
	initial Token,
	fetch func(context.Context, Token) (T, Token, bool, error),
) (T, int, bool, error) {
	var zero T
	targetPage := clampWebPageNumber(page)
	token := initial
	var current T
	var hasMore bool
	for currentPage := 1; currentPage <= targetPage; currentPage++ {
		value, nextToken, more, err := fetch(ctx, token)
		if err != nil {
			return zero, 0, false, err
		}
		current = value
		hasMore = more
		if currentPage == targetPage || !more {
			return current, currentPage, more, nil
		}
		token = nextToken
	}
	return current, targetPage, hasMore, nil
}
