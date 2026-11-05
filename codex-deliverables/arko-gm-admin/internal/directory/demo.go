package directory

type demonstrationDirectory struct {
	accounts   []AccountSummary
	characters []CharacterSummary
}

// NewDemonstrationDirectory returns a synthetic, in-memory directory.
func NewDemonstrationDirectory() Directory {
	return &demonstrationDirectory{
		accounts: []AccountSummary{
			{Reference: "demo-account-01", Name: "DEMONSTRAÇÃO | Conta 01", CharacterCount: 2, Label: DemonstrationLabel},
			{Reference: "demo-account-02", Name: "DEMONSTRAÇÃO | Conta 02", CharacterCount: 1, Label: DemonstrationLabel},
			{Reference: "demo-account-03", Name: "DEMONSTRAÇÃO | Conta 03", CharacterCount: 2, Label: DemonstrationLabel},
		},
		characters: []CharacterSummary{
			{Reference: "demo-character-a", AccountReference: "demo-account-01", Name: "DEMONSTRAÇÃO | Personagem A", AccountName: "DEMONSTRAÇÃO | Conta 01", Label: DemonstrationLabel},
			{Reference: "demo-character-b", AccountReference: "demo-account-01", Name: "DEMONSTRAÇÃO | Personagem B", AccountName: "DEMONSTRAÇÃO | Conta 01", Label: DemonstrationLabel},
			{Reference: "demo-character-c", AccountReference: "demo-account-02", Name: "DEMONSTRAÇÃO | Personagem C", AccountName: "DEMONSTRAÇÃO | Conta 02", Label: DemonstrationLabel},
			{Reference: "demo-character-d", AccountReference: "demo-account-03", Name: "DEMONSTRAÇÃO | Personagem D", AccountName: "DEMONSTRAÇÃO | Conta 03", Label: DemonstrationLabel},
			{Reference: "demo-character-e", AccountReference: "demo-account-03", Name: "DEMONSTRAÇÃO | Personagem E", AccountName: "DEMONSTRAÇÃO | Conta 03", Label: DemonstrationLabel},
		},
	}
}

func (d *demonstrationDirectory) SearchAccounts(request SearchRequest) (Page[AccountSummary], error) {
	if err := validateRequest(request); err != nil {
		return Page[AccountSummary]{}, err
	}

	matches := make([]AccountSummary, 0, len(d.accounts))
	for _, account := range d.accounts {
		if queryMatches(request.Query, account.Name) {
			matches = append(matches, account)
		}
	}
	return pageItems(matches, request), nil
}

func (d *demonstrationDirectory) SearchCharacters(request SearchRequest) (Page[CharacterSummary], error) {
	if err := validateRequest(request); err != nil {
		return Page[CharacterSummary]{}, err
	}

	matches := make([]CharacterSummary, 0, len(d.characters))
	for _, character := range d.characters {
		if queryMatches(request.Query, character.Name) || queryMatches(request.Query, character.AccountName) {
			matches = append(matches, character)
		}
	}
	return pageItems(matches, request), nil
}
