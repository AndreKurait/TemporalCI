package eval

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
)

var varPattern = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)

// Evaluate evaluates a conditional expression against an environment map.
// Variables are referenced as $VAR_NAME. Supported operators: ==, !=, contains, startsWith, &&, ||, !.
func Evaluate(expression string, env map[string]string) (bool, error) {
	if expression == "" {
		return true, nil
	}

	// Replace $VAR references with env lookups
	resolved := varPattern.ReplaceAllStringFunc(expression, func(match string) string {
		name := match[1:]
		if v, ok := env[name]; ok {
			return fmt.Sprintf("%q", v)
		}
		return `""`
	})

	// Convert our syntax to expr-lang compatible syntax
	resolved = strings.ReplaceAll(resolved, " contains ", " in ")
	// "x" in "y" isn't right for expr — use contains() function
	// Actually expr-lang uses `"substr" in "string"` or we can use contains()
	// Let's use a custom approach: replace `A contains B` with `containsFn(A, B)`
	resolved = expression // reset and do proper transform

	resolved = varPattern.ReplaceAllStringFunc(resolved, func(match string) string {
		name := match[1:]
		if v, ok := env[name]; ok {
			return fmt.Sprintf("%q", v)
		}
		return `""`
	})

	// Build expr environment with helper functions
	envMap := map[string]interface{}{
		"containsFn":    func(haystack, needle string) bool { return strings.Contains(haystack, needle) },
		"startsWithFn":  func(s, prefix string) bool { return strings.HasPrefix(s, prefix) },
	}

	// Also inject raw env vars as identifiers for `event == 'push'` style
	for k, v := range env {
		envMap[k] = v
	}

	// Transform operators
	resolved = transformContains(resolved)
	resolved = transformStartsWith(resolved)
	// Convert single-quoted strings to double-quoted for expr
	resolved = strings.ReplaceAll(resolved, "'", "\"")

	program, err := expr.Compile(resolved, expr.Env(envMap), expr.AsBool())
	if err != nil {
		return false, fmt.Errorf("compile expression %q: %w", expression, err)
	}

	result, err := expr.Run(program, envMap)
	if err != nil {
		return false, fmt.Errorf("evaluate expression %q: %w", expression, err)
	}

	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("expression %q did not return bool", expression)
	}
	return b, nil
}

// transformContains converts `A contains B` to `containsFn(A, B)`.
func transformContains(s string) string {
	re := regexp.MustCompile(`(\S+)\s+contains\s+(\S+)`)
	return re.ReplaceAllString(s, "containsFn($1, $2)")
}

// transformStartsWith converts `A startsWith B` to `startsWithFn(A, B)`.
func transformStartsWith(s string) string {
	re := regexp.MustCompile(`(\S+)\s+startsWith\s+(\S+)`)
	return re.ReplaceAllString(s, "startsWithFn($1, $2)")
}
