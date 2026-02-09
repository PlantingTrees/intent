package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/PlantingTrees/intent/auth"
	engine "github.com/PlantingTrees/intent/intentEngine"
)

func main() {
	fmt.Println("=== Intent Engine Initalizing ===")

	fmt.Println("Authenticating with Gmail...\n")
	// 1. Authenticate with Gmail
	c, err := auth.Authenticate()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Logout()

	// 2. Create parser and executor
	parser := engine.NewParser()
	executor := engine.NewExecutor(c) // Pass the IMAP client

	fmt.Println("\n=== Ready! ===")
	fmt.Println("\nExample commands:")
	for i, example := range engine.ParseExamples() {
		fmt.Printf("  %d. %s\n", i+1, example)
	}
	fmt.Println("\nType 'help' for more examples, 'quit' to exit")

	// 3. Interactive loop

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\nIntent > ")

		// Read user input
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v", err)
			continue
		}

		input = strings.TrimSpace(input)

		// Handle special commands
		if input == "" {
			continue
		}

		if input == "quit" || input == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		if input == "help" || input == "examples" {
			fmt.Println("\nExample commands:")
			for i, example := range engine.ParseExamples() {
				fmt.Printf("  %d. %s\n", i+1, example)
			}
			continue
		}

		// Parse the intent
		intent, err := parser.Parse(input)
		if err != nil {
			fmt.Println("\nExpected format:")
			fmt.Println(`  SEARCH for "keywords" from "sender" [date_range]`)
			fmt.Println(`  LISTEN from "sender"`)
			continue
		}

		fmt.Println("✓ Parsed successfully!")

		// Validate the intent
		if err := executor.Validate(intent); err != nil {
			fmt.Printf("Validation error: %v\n", err)
			continue
		}

		fmt.Println("✓ Validated successfully!")

		// Execute the intent
		_, err = executor.Execute(intent)
		if err != nil {
			fmt.Printf("Execution error: %v\n", err)
			continue
		}
	}
}
