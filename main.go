package main

import (
	"fmt"
  "unicode"
  "unicode/utf8"
  "strings"
  "bufio"
)

func WrapTraceableErrorf(err error, format string, args ...interface{}) error {
  msg := fmt.Sprintf(format, args...)
  return fmt.Errorf("%s: %w", msg, err)
}

func ShellSplit(s string) ([]string, error) {
	return ShellSplitEx(s, unicode.IsSpace)
}

func ShellSplitEx(s string, splitFn func(rune) bool) ([]string, error) {
	ss := make([]string, 0)
	b := []byte(s)
	l := len(b)
	idx := 0 // next rune index
	// define convenient functions with closure
	skipSplitCh := func() error { // skip spaces
		for idx < l {
			r, s := utf8.DecodeRune(b[idx:])
			if r == utf8.RuneError { // invalid Unicode encoding
				return WrapTraceableErrorf(nil,
					"failed to skip spaces: invalid Unicode encoding char '%c' at index %d (%s)",
					b[idx], idx, string(b[:idx]))
			}
			if !splitFn(r) { // done, stop at the non-split rune
				break
			}
			idx += s
		}
		// end of string
		return nil
	}
	findEndQuote := func(q rune) error { // find the end matching quote
		prev := utf8.RuneError
		for idx < l {
			r, s := utf8.DecodeRune(b[idx:])
			if r == utf8.RuneError { // invalid Unicode encoding
				return WrapTraceableErrorf(nil,
					"failed to find end matching quote: invalid Unicode encoding char '%c' at index %d (%s)",
					b[idx], idx, string(b[:idx]))
			}
			idx += s
			if r == q {
				if prev != '\\' { // found it
					return nil
				}
				// escaped quote
			}
			prev = r
		}
		// end of string
		return WrapTraceableErrorf(nil, "no end matching quote (%c) found", q)
	}
	findSplitCh := func() error { // find the next space to split
		prev := utf8.RuneError
		for idx < l {
			r, s := utf8.DecodeRune(b[idx:])
			if r == utf8.RuneError { // invalid Unicode encoding
				return WrapTraceableErrorf(nil,
					"failed to find next space: invalid Unicode encoding char '%c' at index %d (%s)",
					b[idx], idx, string(b[:idx]))
			}
			if splitFn(r) { // found it
				return nil
			}
			idx += s
			if r == '"' || r == '\'' { // quote
				if prev == '\\' { // escaped quote
					prev = r
					continue
				}
				start := idx
				if err := findEndQuote(r); err != nil { // find the matching end quote
					return WrapTraceableErrorf(err, "failed to find the matching quote starting at index %d (%s)",
						start, string(b[:start]))
				}
			}
			prev = r
		}
		// end of string
		return nil
	}

	var start int
	for idx < l {
		if err := skipSplitCh(); err != nil {
			return nil, err
		}
		start = idx
		if err := findSplitCh(); err != nil {
			return nil, err
		}
		if start < idx {
			if b[start] == b[idx-1] && (b[start] == '"' || b[start] == '\'') {
				// remove the surrounding quotes
				ss = append(ss, string(b[start+1:idx-1]))
			} else {
				ss = append(ss, string(b[start:idx]))
			}
		}
	}
	if len(ss) == 0 {
		return nil, nil
	}
	return ss, nil
}

func ParseBootConfig(input string) ([]string, error) {
	// $ cat /proc/bootconfig
	// CabCmdBranches = "test\x20me", "here", "ok"
	// CabCmdDryRun = "1"
	// CabServer = "10.10.1.234"
	cmds := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()
		kv := strings.SplitN(line, "=", 2)
		if l := len(kv); l != 2 {
			return nil, WrapTraceableErrorf(nil, "failed to parse /proc/bootconfig output line %q: missing '='", line)
		}
		key := strings.TrimSpace(kv[0])
		fields, err := ShellSplitEx(kv[1], func(r rune) bool {
			return unicode.IsSpace(r) || r == ','
		})
		if err != nil {
			return nil, WrapTraceableErrorf(err, "failed to parse /proc/bootconfig output line %q after '='", line)
		}
		value := strings.Join(fields, ",")
		cmds = append(cmds, fmt.Sprintf("%s=%s", key, value))
	}
	if err := scanner.Err(); err != nil {
		return nil, WrapTraceableErrorf(err, "failed to parse /proc/bootconfig output")
	}
	return cmds, nil
}

func main() {
  cmd := `test me "here and there" ok`
  fields, err := ShellSplit(cmd)
  if err != nil {
    fmt.Printf("ERROR: Failed to shell-split %q: %+v\n", cmd, err)
  }
	fmt.Printf("Shell Split: %q\n", fields)
  
  cmd = ` "test\x20me", "here", "ok"`
  fields, err = ShellSplitEx(cmd, func(r rune) bool {
    return unicode.IsSpace(r) || r == ','
  })
  if err != nil {
    fmt.Printf("ERROR: Failed to shellSplitEx %q: %+v\n", cmd, err)
  }
	fmt.Printf("Shell Split Ex: %q\n", fields)

  cmd = `CabCmdBranches = "test me","here, \"quoted\", \'too\', there" "ok"`
  fields, err = ShellSplitEx(cmd, func(r rune) bool {
    return unicode.IsSpace(r) || r == ','
  })
  if err != nil {
    fmt.Printf("ERROR: Failed to shellSplitEx %q: %+v\n", cmd, err)
  }
	fmt.Printf("Shell Split Ex: %q\n", fields)

  fields, err = ParseBootConfig(bootcfg)
  if err != nil {
    fmt.Printf("ERROR: Failed to ParseBootConfig %q: %+v\n", cmd, err)
  }
	fmt.Printf("cmds: %q\n", fields)

}

const bootcfg = `kernel.CabCmdBranches = "test\x20me", "here", "ok"
kernel.CabCmdDryRun = "1"
kernel.CabIP = "10.10.1.234"
`