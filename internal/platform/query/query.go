// Package query provides support for pagination and sorting.
package query

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	defaultLimit = 10
	maxLimit     = 100
)

type Page struct {
	Limit  int
	Offset int
}

// ParsePage parses ?page and ?limit into a Page.
func ParsePage(r *http.Request) (Page, error) {
	limit := defaultLimit

	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error

		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maxLimit {
			return Page{}, fmt.Errorf("limit must be an integer between 1 and %d", maxLimit)
		}
	}

	page := 1

	if raw := r.URL.Query().Get("page"); raw != "" {
		var err error

		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 {
			return Page{}, errors.New("page must be a positive integer")
		}
	}

	return Page{
		Limit:  limit,
		Offset: (page - 1) * limit,
	}, nil
}

type Order struct {
	Field string
	Desc  bool
}

type OrderBy []Order

// ParseOrder parses ?sort against the allowed whitelist.
func ParseOrder(r *http.Request, allowed map[string]string) (OrderBy, error) {
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		return nil, nil
	}

	var orders OrderBy

	for part := range strings.SplitSeq(sort, ",") {
		desc := strings.HasPrefix(part, "-")
		field := strings.TrimPrefix(part, "-")

		column, ok := allowed[field]
		if !ok {
			return nil, fmt.Errorf("unknown sort field %q", field)
		}

		orders = append(orders, Order{
			Field: column,
			Desc:  desc,
		})
	}

	return orders, nil
}

// SQL renders the ORDER BY clause with a tiebreaker for stable pagination.
func (ob OrderBy) SQL(tiebreaker string) string {
	parts := make([]string, 0, len(ob)+1)
	tied := false

	for _, o := range ob {
		dir := " ASC"
		if o.Desc {
			dir = " DESC"
		}

		parts = append(parts, o.Field+dir)

		if o.Field == tiebreaker {
			tied = true
		}
	}

	if !tied {
		parts = append(parts, tiebreaker+" ASC")
	}

	return strings.Join(parts, ", ")
}

type Result[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

func NewResult[T any](items []T, total int64, page Page) Result[T] {
	if items == nil {
		items = []T{}
	}

	return Result[T]{
		Items: items,
		Total: total,
		Page:  page.Offset/page.Limit + 1,
		Limit: page.Limit,
	}
}
