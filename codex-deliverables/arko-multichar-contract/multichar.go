package multichar

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const ContractVersion = "1.0.0"

type Progression string

const (
	ProgressionArch        Progression = "Arch"
	ProgressionMortal      Progression = "Mortal"
	ProgressionCelestial   Progression = "Celestial"
	ProgressionCelestialCS Progression = "CelestialCS"
	ProgressionSCelestial  Progression = "SCelestial"
)

var progressions = map[Progression]struct{}{
	ProgressionArch:        {},
	ProgressionMortal:      {},
	ProgressionCelestial:   {},
	ProgressionCelestialCS: {},
	ProgressionSCelestial:  {},
}

// AccountSnapshot is a read-only contract snapshot. Revision is json.Number so
// non-negative integer values are not artificially limited to machine width.
type AccountSnapshot struct {
	ContractVersion string      `json:"contract_version"`
	AccountRef      string      `json:"account_ref"`
	Characters      []Character `json:"characters"`
	ActiveSessions  int         `json:"active_sessions"`
}

type Character struct {
	CharacterRef string      `json:"character_ref"`
	Progression  Progression `json:"progression"`
	Revision     json.Number `json:"revision"`
}

// Validate checks the proposal's JSON Schema constraints. It does not interpret
// slots, sessions, progression behavior, or server-side policy.
func (snapshot AccountSnapshot) Validate() error {
	if snapshot.ContractVersion != ContractVersion {
		return fmt.Errorf("contract_version must be %q, got %q", ContractVersion, snapshot.ContractVersion)
	}
	if !validRef(snapshot.AccountRef) {
		return errors.New("account_ref must contain 1 to 128 characters")
	}
	if snapshot.Characters == nil {
		return errors.New("characters must be an array")
	}
	if len(snapshot.Characters) > 8 {
		return errors.New("characters must contain no more than 8 items")
	}
	if snapshot.ActiveSessions < 0 || snapshot.ActiveSessions > 2 {
		return errors.New("active_sessions must be between 0 and 2")
	}
	seen := make(map[string]struct{}, len(snapshot.Characters))
	for i, character := range snapshot.Characters {
		if !validRef(character.CharacterRef) {
			return fmt.Errorf("characters[%d].character_ref must contain 1 to 128 characters", i)
		}
		if _, ok := progressions[character.Progression]; !ok {
			return fmt.Errorf("characters[%d].progression is not a supported category", i)
		}
		if !nonNegativeInteger(character.Revision) {
			return fmt.Errorf("characters[%d].revision must be a non-negative integer", i)
		}
		if _, exists := seen[character.CharacterRef]; exists {
			return fmt.Errorf("duplicate character_ref %q in account %q", character.CharacterRef, snapshot.AccountRef)
		}
		seen[character.CharacterRef] = struct{}{}
	}
	return nil
}

func validRef(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) >= 1 && utf8.RuneCountInString(value) <= 128
}

// nonNegativeInteger follows JSON Schema's numeric "integer" semantics,
// including integral decimal and exponent forms such as 2.0 and 2e0.
func nonNegativeInteger(number json.Number) bool {
	raw := number.String()
	if raw == "" || !json.Valid([]byte(raw)) {
		return false
	}

	negative := strings.HasPrefix(raw, "-")
	if negative {
		raw = raw[1:]
	}
	exponent := new(big.Int)
	if at := strings.IndexAny(raw, "eE"); at >= 0 {
		if _, ok := exponent.SetString(raw[at+1:], 10); !ok {
			return false
		}
		raw = raw[:at]
	}

	integerPart, fractionalPart, _ := strings.Cut(raw, ".")
	digits := strings.TrimLeft(integerPart+fractionalPart, "0")
	if digits == "" {
		return true
	}
	if negative {
		return false
	}

	scale := new(big.Int).Sub(big.NewInt(int64(len(fractionalPart))), exponent)
	if scale.Sign() <= 0 {
		return true
	}
	if !scale.IsInt64() || scale.Int64() > int64(len(digits)) {
		return false
	}
	trailingZeros := len(digits) - len(strings.TrimRight(digits, "0"))
	return trailingZeros >= int(scale.Int64())
}

type accountWire struct {
	ContractVersion *string          `json:"contract_version"`
	AccountRef      *string          `json:"account_ref"`
	Characters      *[]characterWire `json:"characters"`
	ActiveSessions  *json.Number     `json:"active_sessions"`
}

type characterWire struct {
	CharacterRef *string      `json:"character_ref"`
	Progression  *Progression `json:"progression"`
	Revision     *json.Number `json:"revision"`
}

// DecodeSnapshots decodes the utility's JSON array of account snapshots. Each
// array item is validated against the v1 contract, including required fields
// and rejection of unknown properties.
func DecodeSnapshots(reader io.Reader) ([]AccountSnapshot, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	decoder.UseNumber()

	var wires []accountWire
	if err := decoder.Decode(&wires); err != nil {
		return nil, fmt.Errorf("decode account snapshots: %w", err)
	}
	if wires == nil {
		return nil, errors.New("account snapshots must be a JSON array")
	}
	if err := ensureEOF(decoder); err != nil {
		return nil, err
	}

	snapshots := make([]AccountSnapshot, len(wires))
	for i, wire := range wires {
		if wire.ContractVersion == nil || wire.AccountRef == nil || wire.Characters == nil || wire.ActiveSessions == nil {
			return nil, fmt.Errorf("account snapshots[%d] is missing a required field or uses null", i)
		}
		activeSessions, ok := smallInteger(*wire.ActiveSessions, 2)
		if !ok {
			return nil, fmt.Errorf("account snapshots[%d].active_sessions must be an integer from 0 to 2", i)
		}
		snapshot := AccountSnapshot{
			ContractVersion: *wire.ContractVersion,
			AccountRef:      *wire.AccountRef,
			ActiveSessions:  activeSessions,
			Characters:      make([]Character, len(*wire.Characters)),
		}
		for characterIndex, character := range *wire.Characters {
			if character.CharacterRef == nil || character.Progression == nil || character.Revision == nil {
				return nil, fmt.Errorf("account snapshots[%d].characters[%d] is missing a required field or uses null", i, characterIndex)
			}
			snapshot.Characters[characterIndex] = Character{
				CharacterRef: *character.CharacterRef,
				Progression:  *character.Progression,
				Revision:     *character.Revision,
			}
		}
		if err := snapshot.Validate(); err != nil {
			return nil, fmt.Errorf("account snapshots[%d]: %w", i, err)
		}
		snapshots[i] = snapshot
	}
	return snapshots, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return fmt.Errorf("read trailing data: %w", err)
	}
	return nil
}

func smallInteger(number json.Number, maximum int) (int, bool) {
	if !nonNegativeInteger(number) {
		return 0, false
	}
	raw := strings.TrimSpace(number.String())
	if strings.HasPrefix(raw, "-") {
		raw = raw[1:]
	}
	exponent := new(big.Int)
	if at := strings.IndexAny(raw, "eE"); at >= 0 {
		if _, ok := exponent.SetString(raw[at+1:], 10); !ok {
			return 0, false
		}
		raw = raw[:at]
	}
	integerPart, fractionalPart, _ := strings.Cut(raw, ".")
	digits := strings.TrimLeft(integerPart+fractionalPart, "0")
	if digits == "" {
		return 0, true
	}
	scale := new(big.Int).Sub(big.NewInt(int64(len(fractionalPart))), exponent)
	if scale.Sign() > 0 {
		if !scale.IsInt64() || scale.Int64() > int64(len(digits)) {
			return 0, false
		}
		places := int(scale.Int64())
		trailingZeros := len(digits) - len(strings.TrimRight(digits, "0"))
		if trailingZeros < places {
			return 0, false
		}
		digits = digits[:len(digits)-places]
	} else if scale.Sign() < 0 {
		zeroes := new(big.Int).Neg(scale)
		if !zeroes.IsInt64() || zeroes.Int64() > 1 {
			return 0, false
		}
		digits += strings.Repeat("0", int(zeroes.Int64()))
	}
	if len(digits) > len(strconv.Itoa(maximum)) {
		return 0, false
	}
	result, err := strconv.Atoi(digits)
	if err != nil || result > maximum {
		return 0, false
	}
	return result, true
}

func cloneSnapshot(snapshot AccountSnapshot) AccountSnapshot {
	snapshot.Characters = append([]Character(nil), snapshot.Characters...)
	return snapshot
}

// Store provides atomic replacement and read-only indexed lookups.
type Store struct {
	mu       sync.RWMutex
	accounts map[string]AccountSnapshot
}

func NewStore(snapshots []AccountSnapshot) (*Store, error) {
	store := &Store{}
	if err := store.Replace(snapshots); err != nil {
		return nil, err
	}
	return store, nil
}

// Replace validates and builds a complete replacement before publishing it.
// Duplicate account_ref values are rejected. Character refs are required to
// be unique within their account; the contract does not define global identity.
func (store *Store) Replace(snapshots []AccountSnapshot) error {
	next := make(map[string]AccountSnapshot, len(snapshots))
	for i, snapshot := range snapshots {
		if err := snapshot.Validate(); err != nil {
			return fmt.Errorf("snapshots[%d]: %w", i, err)
		}
		if _, exists := next[snapshot.AccountRef]; exists {
			return fmt.Errorf("duplicate account_ref %q", snapshot.AccountRef)
		}
		copyOfSnapshot := cloneSnapshot(snapshot)
		sort.Slice(copyOfSnapshot.Characters, func(i, j int) bool {
			return copyOfSnapshot.Characters[i].CharacterRef < copyOfSnapshot.Characters[j].CharacterRef
		})
		next[snapshot.AccountRef] = copyOfSnapshot
	}

	store.mu.Lock()
	store.accounts = next
	store.mu.Unlock()
	return nil
}

// ReplaceJSON decodes and validates the whole input before changing the store.
func (store *Store) ReplaceJSON(reader io.Reader) error {
	snapshots, err := DecodeSnapshots(reader)
	if err != nil {
		return err
	}
	return store.Replace(snapshots)
}

func (store *Store) Lookup(accountRef string) (AccountSnapshot, bool) {
	store.mu.RLock()
	snapshot, ok := store.accounts[accountRef]
	if ok {
		snapshot = cloneSnapshot(snapshot)
	}
	store.mu.RUnlock()
	return snapshot, ok
}

func (store *Store) LookupCharacter(accountRef, characterRef string) (Character, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	snapshot, ok := store.accounts[accountRef]
	if !ok {
		return Character{}, false
	}
	index := sort.Search(len(snapshot.Characters), func(i int) bool {
		return snapshot.Characters[i].CharacterRef >= characterRef
	})
	if index == len(snapshot.Characters) || snapshot.Characters[index].CharacterRef != characterRef {
		return Character{}, false
	}
	return snapshot.Characters[index], true
}

// List returns account snapshots sorted by account_ref, with each character
// list sorted by character_ref. Returned slices do not alias store state.
func (store *Store) List() []AccountSnapshot {
	store.mu.RLock()
	defer store.mu.RUnlock()
	accountRefs := make([]string, 0, len(store.accounts))
	for accountRef := range store.accounts {
		accountRefs = append(accountRefs, accountRef)
	}
	sort.Strings(accountRefs)
	snapshots := make([]AccountSnapshot, 0, len(accountRefs))
	for _, accountRef := range accountRefs {
		snapshots = append(snapshots, cloneSnapshot(store.accounts[accountRef]))
	}
	return snapshots
}

// Load constructs a store from a JSON array of account snapshots.
func Load(reader io.Reader) (*Store, error) {
	snapshots, err := DecodeSnapshots(reader)
	if err != nil {
		return nil, err
	}
	return NewStore(snapshots)
}

// MarshalList encodes the store's deterministic sorted representation.
func (store *Store) MarshalList() ([]byte, error) {
	var output bytes.Buffer
	if err := json.NewEncoder(&output).Encode(store.List()); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
