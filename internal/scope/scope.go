package scope

import (
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
)

// IsAllowed evaluates if a given file path is permitted by the Intent.
// A path is allowed if it matches at least one Allow rule and does not match any Deny rule.
// If Allow is empty, everything is denied by default unless the Intent is completely empty (which also denies all).
func (i *Intent) IsAllowed(path string) (bool, error) {
	// Deny rules take precedence. If a path matches any deny rule, block it immediately.
	for _, denyPattern := range i.Deny {
		match, err := doublestar.Match(denyPattern, path)
		if err != nil {
			return false, fmt.Errorf("invalid deny pattern '%s': %w", denyPattern, err)
		}
		if match {
			return false, nil
		}
	}

	// In strict security contexts, default is deny. If no Allow rules exist, return false.
	if len(i.Allow) == 0 {
		return false, nil
	}

	// The path must match at least one Allow rule to be permitted.
	for _, allowPattern := range i.Allow {
		match, err := doublestar.Match(allowPattern, path)
		if err != nil {
			return false, fmt.Errorf("invalid allow pattern '%s': %w", allowPattern, err)
		}
		if match {
			return true, nil
		}
	}

	return false, nil
}
