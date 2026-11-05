package directory

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSearchFiltersAndPaginates(t *testing.T) {
	directory := NewDemonstrationDirectory()

	accounts, err := directory.SearchAccounts(SearchRequest{Query: "conta 02", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if accounts.Total != 1 || len(accounts.Items) != 1 || accounts.Items[0].CharacterCount != 1 {
		t.Fatalf("unexpected account results: %#v", accounts)
	}
	if accounts.Items[0].Label != DemonstrationLabel {
		t.Fatalf("account is missing synthetic-data label: %#v", accounts.Items[0])
	}

	characters, err := directory.SearchCharacters(SearchRequest{Query: "personagem", Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if characters.Total != 5 || len(characters.Items) != 2 || characters.Items[0].Name != "DEMONSTRAÇÃO | Personagem C" {
		t.Fatalf("unexpected character page: %#v", characters)
	}
	if characters.Items[0].AccountName != "DEMONSTRAÇÃO | Conta 02" || characters.Items[0].Label != DemonstrationLabel {
		t.Fatalf("unexpected character summary: %#v", characters.Items[0])
	}
}

func TestSearchRequestBounds(t *testing.T) {
	directory := NewDemonstrationDirectory()
	tests := []struct {
		name    string
		request SearchRequest
		wantErr error
	}{
		{name: "zero page", request: SearchRequest{Page: 0, PageSize: 1}, wantErr: ErrInvalidPage},
		{name: "page over limit", request: SearchRequest{Page: MaxPageNumber + 1, PageSize: 1}, wantErr: ErrInvalidPage},
		{name: "zero page size", request: SearchRequest{Page: 1, PageSize: 0}, wantErr: ErrInvalidPageSize},
		{name: "page size over limit", request: SearchRequest{Page: 1, PageSize: MaxPageSize + 1}, wantErr: ErrInvalidPageSize},
		{name: "long query", request: SearchRequest{Query: strings.Repeat("x", MaxQueryLength+1), Page: 1, PageSize: 1}, wantErr: ErrInvalidQuery},
		{name: "invalid UTF-8", request: SearchRequest{Query: string([]byte{0xff}), Page: 1, PageSize: 1}, wantErr: ErrInvalidQuery},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := directory.SearchAccounts(test.request)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("SearchAccounts error = %v, want %v", err, test.wantErr)
			}
			_, err = directory.SearchCharacters(test.request)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("SearchCharacters error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestDirectoryHasReadOnlyInterfaceAndDetachedResults(t *testing.T) {
	directory := NewDemonstrationDirectory()

	interfaceType := reflect.TypeOf((*Directory)(nil)).Elem()
	if interfaceType.NumMethod() != 2 {
		t.Fatalf("Directory exposes %d methods, want exactly two read methods", interfaceType.NumMethod())
	}
	for _, name := range []string{"SearchAccounts", "SearchCharacters"} {
		if _, ok := interfaceType.MethodByName(name); !ok {
			t.Fatalf("Directory is missing read method %q", name)
		}
	}
	if concreteMethods := reflect.TypeOf(directory).NumMethod(); concreteMethods != 2 {
		t.Fatalf("demonstration adapter exposes %d exported methods, want exactly two read methods", concreteMethods)
	}

	first, err := directory.SearchAccounts(SearchRequest{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	first.Items[0].Name = "caller-modified"
	second, err := directory.SearchAccounts(SearchRequest{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if second.Items[0].Name == "caller-modified" {
		t.Fatal("mutating a returned page changed the in-memory fixture")
	}
}
