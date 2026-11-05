package multichar

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func sample(accountRef string) AccountSnapshot {
	return AccountSnapshot{
		ContractVersion: ContractVersion,
		AccountRef:      accountRef,
		Characters: []Character{
			{CharacterRef: "char-b", Progression: ProgressionMortal, Revision: json.Number("3")},
			{CharacterRef: "char-a", Progression: ProgressionArch, Revision: json.Number("0")},
		},
		ActiveSessions: 1,
	}
}

func TestDecodeAndSortedReadOperations(t *testing.T) {
	store, err := Load(strings.NewReader(`[
		{"contract_version":"1.0.0","account_ref":"z-account","characters":[{"character_ref":"char-b","progression":"Mortal","revision":3},{"character_ref":"char-a","progression":"Arch","revision":0}],"active_sessions":1},
		{"contract_version":"1.0.0","account_ref":"a-account","characters":[],"active_sessions":0}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	list := store.List()
	if len(list) != 2 || list[0].AccountRef != "a-account" || list[1].AccountRef != "z-account" {
		t.Fatalf("accounts are not sorted: %#v", list)
	}
	if got := list[1].Characters[0].CharacterRef; got != "char-a" {
		t.Fatalf("characters are not sorted: got %q", got)
	}
	if character, ok := store.LookupCharacter("z-account", "char-b"); !ok || character.Progression != ProgressionMortal {
		t.Fatalf("character lookup failed: %#v, %v", character, ok)
	}
	if _, ok := store.Lookup("unknown"); ok {
		t.Fatal("unexpected lookup result")
	}
}

func TestDecodeRejectsMalformedAndOutOfSchemaInputs(t *testing.T) {
	tests := map[string]string{
		"malformed":           `[`,
		"root-not-array":      `{}`,
		"trailing-value":      `[] []`,
		"missing-required":    `[{"contract_version":"1.0.0","account_ref":"a","characters":[]}]`,
		"null-array":          `[{"contract_version":"1.0.0","account_ref":"a","characters":null,"active_sessions":0}]`,
		"unknown-property":    `[{"contract_version":"1.0.0","account_ref":"a","characters":[],"active_sessions":0,"extra":true}]`,
		"unknown-category":    `[{"contract_version":"1.0.0","account_ref":"a","characters":[{"character_ref":"c","progression":"Other","revision":1}],"active_sessions":0}]`,
		"fractional-revision": `[{"contract_version":"1.0.0","account_ref":"a","characters":[{"character_ref":"c","progression":"Arch","revision":1.5}],"active_sessions":0}]`,
		"too-many-characters": `[{"contract_version":"1.0.0","account_ref":"a","characters":[{"character_ref":"1","progression":"Arch","revision":0},{"character_ref":"2","progression":"Arch","revision":0},{"character_ref":"3","progression":"Arch","revision":0},{"character_ref":"4","progression":"Arch","revision":0},{"character_ref":"5","progression":"Arch","revision":0},{"character_ref":"6","progression":"Arch","revision":0},{"character_ref":"7","progression":"Arch","revision":0},{"character_ref":"8","progression":"Arch","revision":0},{"character_ref":"9","progression":"Arch","revision":0}],"active_sessions":0}]`,
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeSnapshots(strings.NewReader(input)); err == nil {
				t.Fatal("expected input to be rejected")
			}
		})
	}
}

func TestIntegerSemantics(t *testing.T) {
	for _, accepted := range []string{"0", "-0", "2.0", "20e-1", "200e-2", "1e100000"} {
		if !nonNegativeInteger(json.Number(accepted)) {
			t.Errorf("expected %q to be an integer", accepted)
		}
		for _, accepted := range []string{"0", "1", "2", "2.0", "20e-1"} {
			if got, ok := smallInteger(json.Number(accepted), 2); !ok || got > 2 {
				t.Errorf("expected active_sessions %q to be in range: got %d, %v", accepted, got, ok)
			}
		}
		for _, rejected := range []string{"3", "3.0", "1.5", "1e100000"} {
			if _, ok := smallInteger(json.Number(rejected), 2); ok {
				t.Errorf("expected active_sessions %q to be out of range", rejected)
			}
		}
	}
	for _, rejected := range []string{"-1", "1.2", "1e-1", "not-a-number", ""} {
		if nonNegativeInteger(json.Number(rejected)) {
			t.Errorf("expected %q to be rejected", rejected)
		}
	}
}

func TestDuplicateReferencesRejected(t *testing.T) {
	store, err := NewStore([]AccountSnapshot{sample("existing")})
	if err != nil {
		t.Fatal(err)
	}
	duplicateAccount := []AccountSnapshot{sample("same"), sample("same")}
	if err := store.Replace(duplicateAccount); err == nil {
		t.Fatal("expected duplicate account_ref error")
	}
	duplicateCharacter := sample("same-account")
	duplicateCharacter.Characters[1].CharacterRef = duplicateCharacter.Characters[0].CharacterRef
	if err := store.Replace([]AccountSnapshot{duplicateCharacter}); err == nil {
		t.Fatal("expected duplicate character_ref error")
	}
	if _, ok := store.Lookup("existing"); !ok {
		t.Fatal("failed replacement changed existing store")
	}
}

func TestFailedJSONReplacementIsAtomic(t *testing.T) {
	store, err := NewStore([]AccountSnapshot{sample("preserved")})
	if err != nil {
		t.Fatal(err)
	}
	inputs := []string{
		`[`,
		`[{"contract_version":"1.0.0","account_ref":"bad","characters":[],"active_sessions":3}]`,
		`[{"contract_version":"1.0.0","account_ref":"dup","characters":[],"active_sessions":0},{"contract_version":"1.0.0","account_ref":"dup","characters":[],"active_sessions":0}]`,
	}
	for _, input := range inputs {
		if err := store.ReplaceJSON(strings.NewReader(input)); err == nil {
			t.Fatalf("expected replacement failure for %s", input)
		}
		if _, ok := store.Lookup("preserved"); !ok || len(store.List()) != 1 {
			t.Fatal("failed replacement partially changed store")
		}
	}
}

func TestDefensiveCopies(t *testing.T) {
	input := sample("copy-test")
	store, err := NewStore([]AccountSnapshot{input})
	if err != nil {
		t.Fatal(err)
	}
	input.Characters[0].CharacterRef = "mutated-input"
	stored, ok := store.Lookup("copy-test")
	if !ok || stored.Characters[0].CharacterRef == "mutated-input" {
		t.Fatal("store retained input slice")
	}
	stored.Characters[0].CharacterRef = "mutated-lookup"
	listed := store.List()
	listed[0].Characters[0].CharacterRef = "mutated-list"
	again, _ := store.Lookup("copy-test")
	if again.Characters[0].CharacterRef != "char-a" {
		t.Fatal("returned values alias store state")
	}
}

func TestConcurrentReplaceAndReads(t *testing.T) {
	store, err := NewStore([]AccountSnapshot{sample("account")})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 8
	const iterations = 100
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for i := 0; i < iterations; i++ {
				if worker%2 == 0 {
					next := sample("account")
					next.Characters[0].Revision = json.Number("100")
					if err := store.Replace([]AccountSnapshot{next}); err != nil {
						t.Errorf("replace: %v", err)
						return
					}
				} else {
					snapshot, ok := store.Lookup("account")
					if !ok || len(snapshot.Characters) != 2 {
						t.Errorf("lookup returned incomplete data: %#v, %v", snapshot, ok)
						return
					}
					_ = store.List()
					_, _ = store.LookupCharacter("account", "char-a")
				}
			}
		}(worker)
	}
	wait.Wait()
}

func TestMarshalListIsJSONAndSorted(t *testing.T) {
	store, err := NewStore([]AccountSnapshot{sample("b"), sample("a")})
	if err != nil {
		t.Fatal(err)
	}
	data, err := store.MarshalList()
	if err != nil {
		t.Fatal(err)
	}
	var decoded []AccountSnapshot
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded[0].AccountRef != "a" || decoded[1].AccountRef != "b" {
		t.Fatalf("unexpected order: %#v", decoded)
	}
}
