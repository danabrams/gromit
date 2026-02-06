package rules_test

import (
	"fmt"
	"log"

	"github.com/danabrams/ralph-runner/internal/rules"
)

func ExampleLoad() {
	// Load a rules file
	r, err := rules.Load(".ralph/RULES.md")
	if err != nil {
		log.Fatal(err)
	}

	// Access sections
	for _, section := range r.Sections {
		fmt.Printf("Section: %s\n", section.Name)
		fmt.Printf("Rules: %d\n", len(section.Rules))
	}
}

func ExampleRules_AddRule() {
	r := &rules.Rules{}

	// Add rules to sections
	r.AddRule("Code Style", "Use idiomatic Go patterns")
	r.AddRule("Code Style", "Keep functions focused and small")
	r.AddRule("Testing", "Write comprehensive tests")

	// The rules are organized by section
	fmt.Printf("Sections: %d\n", len(r.Sections))
	// Output: Sections: 2
}

func ExampleRules_ModifyRule() {
	r := &rules.Rules{
		Sections: []rules.Section{
			{
				Name:  "Code Style",
				Rules: []string{"Old rule text"},
			},
		},
	}

	// Modify an existing rule
	err := r.ModifyRule("Code Style", "Old rule text", "New rule text")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(r.Sections[0].Rules[0])
	// Output: New rule text
}

func ExampleRules_GetSection() {
	r := &rules.Rules{
		Sections: []rules.Section{
			{Name: "Code Style", Rules: []string{"Rule 1", "Rule 2"}},
			{Name: "Testing", Rules: []string{"Rule 3"}},
		},
	}

	// Get a specific section
	section := r.GetSection("Testing")
	if section != nil {
		fmt.Printf("Found section '%s' with %d rules\n", section.Name, len(section.Rules))
	}
	// Output: Found section 'Testing' with 1 rules
}
