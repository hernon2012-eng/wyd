// Package directory provides read-only account and character summaries.
package directory

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	// DemonstrationLabel marks every record as synthetic and non-official.
	DemonstrationLabel = "DEMONSTRAÇÃO — dados sintéticos, não oficiais"

	MaxQueryLength = 100
	MaxPageSize    = 100
	MaxPageNumber  = 1_000_000
)

var (
	ErrInvalidQuery    = errors.New("directory: query must be valid UTF-8 and at most 100 characters")
	ErrInvalidPage     = errors.New("directory: page must be between 1 and 1000000")
	ErrInvalidPageSize = errors.New("directory: page size must be between 1 and 100")
)

// AccountSummary contains display-only account information.
type AccountSummary struct {
	Reference      string
	Name           string
	CharacterCount int
	Label          string
}

// CharacterSummary contains display-only character and owning-account names.
type CharacterSummary struct {
	Reference        string
	AccountReference string
	Name             string
	AccountName      string
	Label            string
}

// SearchRequest bounds a case-insensitive substring search and its result page.
type SearchRequest struct {
	Query    string
	Page     int
	PageSize int
}

// Page contains one page of read-only search results.
type Page[T any] struct {
	Items    []T
	Page     int
	PageSize int
	Total    int
}

// Directory exposes search operations only; it has no mutation capabilities.
type Directory interface {
	SearchAccounts(SearchRequest) (Page[AccountSummary], error)
	SearchCharacters(SearchRequest) (Page[CharacterSummary], error)
}

func validateRequest(request SearchRequest) error {
	if !utf8.ValidString(request.Query) || len(request.Query) > MaxQueryLength*4 ||
		utf8.RuneCountInString(request.Query) > MaxQueryLength {
		return ErrInvalidQuery
	}
	if request.Page < 1 || request.Page > MaxPageNumber {
		return ErrInvalidPage
	}
	if request.PageSize < 1 || request.PageSize > MaxPageSize {
		return ErrInvalidPageSize
	}
	return nil
}

func queryMatches(query, value string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	return query == "" || strings.Contains(strings.ToLower(value), query)
}

func pageItems[T any](items []T, request SearchRequest) Page[T] {
	start := (request.Page - 1) * request.PageSize
	if start >= len(items) {
		return Page[T]{
			Items:    []T{},
			Page:     request.Page,
			PageSize: request.PageSize,
			Total:    len(items),
		}
	}

	end := start + request.PageSize
	if end > len(items) {
		end = len(items)
	}
	return Page[T]{
		Items:    append([]T(nil), items[start:end]...),
		Page:     request.Page,
		PageSize: request.PageSize,
		Total:    len(items),
	}
}
