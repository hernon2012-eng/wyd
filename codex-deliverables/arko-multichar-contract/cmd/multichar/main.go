package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/arko/multichar-contract"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) == 0 {
		return usageError()
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	filename := flags.String("file", "-", "JSON array of account snapshots, or - for stdin")
	if err := flags.Parse(args[1:]); err != nil {
		return usageError()
	}
	remaining := flags.Args()
	switch command {
	case "validate", "list":
		if len(remaining) != 0 {
			return usageError()
		}
	case "get":
		if len(remaining) != 1 {
			return usageError()
		}
	case "character":
		if len(remaining) != 2 {
			return usageError()
		}
	default:
		return usageError()
	}

	input, closeInput, err := openInput(*filename)
	if err != nil {
		return err
	}
	defer closeInput()
	store, err := multichar.Load(input)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	switch command {
	case "validate":
		_, err = fmt.Fprintf(output, "valid: %d account snapshots\n", len(store.List()))
	case "list":
		err = encoder.Encode(store.List())
	case "get":
		snapshot, found := store.Lookup(remaining[0])
		if !found {
			return fmt.Errorf("account_ref %q not found", remaining[0])
		}
		err = encoder.Encode(snapshot)
	case "character":
		character, found := store.LookupCharacter(remaining[0], remaining[1])
		if !found {
			return fmt.Errorf("character_ref %q not found in account %q", remaining[1], remaining[0])
		}
		err = encoder.Encode(character)
	}
	return err
}

func openInput(filename string) (io.Reader, func(), error) {
	if filename == "-" {
		return os.Stdin, func() {}, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open input: %w", err)
	}
	return file, func() { _ = file.Close() }, nil
}

func usageError() error {
	return fmt.Errorf("usage: multichar {validate|list|get <account_ref>|character <account_ref> <character_ref>} [-file path|-]")
}
