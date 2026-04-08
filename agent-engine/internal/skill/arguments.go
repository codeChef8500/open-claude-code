package skill

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
)

// ParseArguments splits an arguments string into individual arguments using
// shell-style quoting rules. Quoted strings are preserved as single arguments.
//
// Examples:
//
//	"foo bar baz"            → ["foo", "bar", "baz"]
//	`foo "hello world" baz`  → ["foo", "hello world", "baz"]
func ParseArguments(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}
	words, err := shellquote.Split(args)
	if err != nil {
		// Fall back to simple whitespace split if parsing fails.
		return strings.Fields(args)
	}
	return words
}

// ParseArgumentNames parses the frontmatter 'arguments' field into a list of
// valid argument names. Numeric-only names are filtered out to avoid conflict
// with the $0/$1 shorthand.
func ParseArgumentNames(raw []string) []string {
	var names []string
	numericRe := regexp.MustCompile(`^\d+$`)
	for _, name := range raw {
		name = strings.TrimSpace(name)
		if name == "" || numericRe.MatchString(name) {
			continue
		}
		names = append(names, name)
	}
	return names
}

// GenerateProgressiveArgumentHint returns a hint for remaining unfilled args.
// e.g., argNames=["file","flags"], typedArgs=["main.go"] → "[flags]"
func GenerateProgressiveArgumentHint(argNames []string, typedArgs []string) string {
	if len(typedArgs) >= len(argNames) {
		return ""
	}
	remaining := argNames[len(typedArgs):]
	parts := make([]string, len(remaining))
	for i, n := range remaining {
		parts[i] = fmt.Sprintf("[%s]", n)
	}
	return strings.Join(parts, " ")
}

// namedArgRegexp matches $name where name is NOT followed by [ or word chars.
// Go's RE2 doesn't support lookaheads, so we capture a trailing boundary char
// and restore it during replacement.
func namedArgRegexp(name string) *regexp.Regexp {
	// Match $name at end of string OR followed by a non-word, non-[ character.
	return regexp.MustCompile(`\$` + regexp.QuoteMeta(name) + `([^\w\[]|$)`)
}

var (
	indexedArgRe = regexp.MustCompile(`\$ARGUMENTS\[(\d+)\]`)
	// Match $N followed by non-word char or end of string.
	shorthandArgRe = regexp.MustCompile(`\$(\d+)([^\w]|$)`)
)

// SubstituteArguments replaces $ARGUMENTS placeholders in content with actual
// argument values, matching claude-code-main's substituteArguments behavior.
//
// Substitutions (applied in this order):
//  1. Named arguments:        $foo → parsedArgs[i] (where argNames[i]=="foo")
//  2. Indexed arguments:      $ARGUMENTS[0] → parsedArgs[0]
//  3. Shorthand indexed:      $0 → parsedArgs[0]
//  4. Full arguments string:  $ARGUMENTS → args
//
// If no placeholder was found and appendIfNoPlaceholder is true, the raw args
// are appended as "\n\nARGUMENTS: <args>".
func SubstituteArguments(content, args string, appendIfNoPlaceholder bool, argNames []string) string {
	if args == "" {
		return content
	}

	parsedArgs := ParseArguments(args)
	original := content

	// 1. Named arguments
	for i, name := range argNames {
		if name == "" {
			continue
		}
		re := namedArgRegexp(name)
		val := ""
		if i < len(parsedArgs) {
			val = parsedArgs[i]
		}
		content = re.ReplaceAllStringFunc(content, func(match string) string {
			sub := re.FindStringSubmatch(match)
			trailing := ""
			if len(sub) > 1 {
				trailing = sub[1]
			}
			return val + trailing
		})
	}

	// 2. Indexed arguments: $ARGUMENTS[N]
	content = indexedArgRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := indexedArgRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		idx, _ := strconv.Atoi(sub[1])
		if idx < len(parsedArgs) {
			return parsedArgs[idx]
		}
		return ""
	})

	// 3. Shorthand indexed: $N
	content = shorthandArgRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := shorthandArgRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		idx, _ := strconv.Atoi(sub[1])
		trailing := sub[2] // boundary character captured
		if idx < len(parsedArgs) {
			return parsedArgs[idx] + trailing
		}
		return trailing
	})

	// 4. Full arguments string
	content = strings.ReplaceAll(content, "$ARGUMENTS", args)

	// Append if no placeholder was found
	if content == original && appendIfNoPlaceholder && args != "" {
		content = content + "\n\nARGUMENTS: " + args
	}

	return content
}
