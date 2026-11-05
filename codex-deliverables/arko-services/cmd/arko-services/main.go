package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	services "arko-services"
)

func main() {
	catalogPath := flag.String("catalog", "", "path to a versioned JSON service catalog")
	serviceType := flag.String("type", "", "filter by service type")
	serviceRef := flag.String("ref", "", "look up one service_ref")
	flag.Parse()

	if *catalogPath == "" {
		fail("-catalog is required")
	}
	file, err := os.Open(*catalogPath)
	if err != nil {
		fail(err.Error())
	}
	defer file.Close()

	registry := services.NewRegistry()
	if err := registry.ReloadJSON(file); err != nil {
		fail(err.Error())
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	switch {
	case *serviceRef != "":
		descriptor, err := registry.Lookup(*serviceRef)
		if err != nil {
			fail(err.Error())
		}
		if err := encoder.Encode(descriptor); err != nil {
			fail(err.Error())
		}
	case *serviceType != "":
		descriptors, err := registry.ListByType(services.ServiceType(*serviceType))
		if err != nil {
			fail(err.Error())
		}
		if err := encoder.Encode(descriptors); err != nil {
			fail(err.Error())
		}
	default:
		if err := encoder.Encode(registry.List()); err != nil {
			fail(err.Error())
		}
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
