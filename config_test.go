package main

import (
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/packer"
)

func TestRequiredParameters(t *testing.T) {
	var c Config
	_, _, err := c.Prepare(&c, make(map[string]interface{}))
	if err == nil {
		t.Fatal("Expected empty configuration to fail")
	}
	errs, ok := err.(*packer.MultiError)
	if !ok {
		t.Fatal("Expected errors to be packer.MultiError")
	}
	required := []string{"name", "ssh_username", "ssh_password"}
	for _, param := range required {
		found := false
		for _, err := range errs.Errors {
			if strings.Contains(err.Error(), param) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error about missing parameter %q", required)
		}
	}
}
