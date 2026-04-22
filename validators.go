package graph

import (
	"fmt"
	"regexp"
)

// Built-in validator constructors. Each returns a Validator — a (metadata,
// function) pair you pass to UnifiedResolver.WithArgValidator. The metadata
// surfaces in FieldInfo.Args[i].Validators for dashboards; the function
// runs pre-resolve and aborts with the returned error.
//
//	r.WithArgValidator("title", graph.Required(), graph.StringLength(1, 100))
//	r.WithArgValidator("email", graph.StringMatch(`^\S+@\S+$`, "must be an email"))
//	r.WithArgValidator("role",  graph.OneOf("admin", "user", "guest"))

// Required rejects nil / missing values. Useful for "Int" args where the
// GraphQL type is nullable but the business rule isn't.
func Required() Validator {
	return Validator{
		Info: ValidatorInfo{Kind: "required", Message: "required"},
		Fn: func(v any) error {
			if v == nil {
				return fmt.Errorf("required")
			}
			if s, ok := v.(string); ok && s == "" {
				return fmt.Errorf("required")
			}
			return nil
		},
	}
}

// StringLength enforces min <= len(value) <= max for string args.
// Use -1 for min or max to skip that side of the check.
func StringLength(min, max int) Validator {
	msg := lengthMessage(min, max)
	return Validator{
		Info: ValidatorInfo{
			Kind:    "length",
			Message: msg,
			Details: map[string]any{"min": min, "max": max},
		},
		Fn: func(v any) error {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("not a string")
			}
			n := len(s)
			if min >= 0 && n < min {
				return fmt.Errorf("%s", msg)
			}
			if max >= 0 && n > max {
				return fmt.Errorf("%s", msg)
			}
			return nil
		},
	}
}

func lengthMessage(min, max int) string {
	switch {
	case min >= 0 && max >= 0:
		return fmt.Sprintf("length must be between %d and %d", min, max)
	case min >= 0:
		return fmt.Sprintf("length must be at least %d", min)
	case max >= 0:
		return fmt.Sprintf("length must be at most %d", max)
	default:
		return "length check"
	}
}

// StringMatch returns a validator that accepts the string arg iff it matches
// the regex. The optional message replaces the default "invalid format" text.
// Panics at build time if the regex is invalid — that's a programmer bug.
func StringMatch(pattern string, message ...string) Validator {
	re := regexp.MustCompile(pattern)
	msg := "invalid format"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	return Validator{
		Info: ValidatorInfo{
			Kind:    "regex",
			Message: msg,
			Details: map[string]any{"pattern": pattern},
		},
		Fn: func(v any) error {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("not a string")
			}
			if !re.MatchString(s) {
				return fmt.Errorf("%s", msg)
			}
			return nil
		},
	}
}

// OneOf restricts the value to the given allowed set. Works for strings and
// ints (most GraphQL scalars that survive JSON decoding).
func OneOf(allowed ...any) Validator {
	set := make(map[any]struct{}, len(allowed))
	for _, a := range allowed {
		set[a] = struct{}{}
	}
	return Validator{
		Info: ValidatorInfo{
			Kind:    "oneOf",
			Message: fmt.Sprintf("must be one of %v", allowed),
			Details: map[string]any{"allowed": allowed},
		},
		Fn: func(v any) error {
			if _, ok := set[v]; !ok {
				return fmt.Errorf("must be one of %v", allowed)
			}
			return nil
		},
	}
}

// IntRange enforces min <= value <= max for Int args. Use math.MinInt /
// math.MaxInt to skip a bound.
func IntRange(min, max int) Validator {
	return Validator{
		Info: ValidatorInfo{
			Kind:    "range",
			Message: fmt.Sprintf("must be between %d and %d", min, max),
			Details: map[string]any{"min": min, "max": max},
		},
		Fn: func(v any) error {
			n, ok := toInt(v)
			if !ok {
				return fmt.Errorf("not an integer")
			}
			if n < min || n > max {
				return fmt.Errorf("must be between %d and %d", min, max)
			}
			return nil
		},
	}
}

// Custom wraps an arbitrary validator function with a displayable name.
// The kind is always "custom"; use the message/details fields of
// ValidatorInfo for anything you want the dashboard to show.
//
//	r.WithArgValidator("slug", graph.Custom("kebab-case", "lowercase letters and hyphens only",
//	    func(v any) error { ... }))
func Custom(name, message string, fn ArgValidator) Validator {
	return Validator{
		Info: ValidatorInfo{
			Kind:    "custom",
			Message: message,
			Details: map[string]any{"name": name},
		},
		Fn: fn,
	}
}

// toInt converts any numeric JSON-decoded value to an int. JSON numbers
// arrive as float64 from graphql-go for untyped input.
func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float32:
		return int(x), true
	case float64:
		return int(x), true
	}
	return 0, false
}