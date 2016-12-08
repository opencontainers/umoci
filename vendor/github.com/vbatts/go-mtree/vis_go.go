// +build !cvis

package mtree

import (
	"fmt"
	"unicode"
)

func vis(src string, flags VisFlag) (string, error) {
	var ret string
	for _, r := range src {
		vStr, err := visRune(r, flags)
		if err != nil {
			return "", err
		}
		ret = ret + vStr
	}
	return ret, nil
}

func visRune(r rune, flags VisFlag) (string, error) {
	if flags&VisHttpstyle != 0 {
		// Described in RFC 1808
		if !isalnum(r) ||
			/* safe */
			r == '$' || r == '-' || r == '_' || r == '.' || r == '+' ||
			/* extra */
			r == '!' || r == '*' || r == '\'' || r == '(' ||
			r == ')' || r == ',' {
			if r < 16 {
				return fmt.Sprintf("%%0%X", r), nil
			}
			return fmt.Sprintf("%%%X", r), nil
		}
	}

	if (flags&VisGlob) != 0 && (r == '*' || r == '?' || r == '[' || r == '#') {
		// ... ?
	} else if isgraph(r) ||
		((flags&VisSp) == 0 && r == ' ') ||
		((flags&VisTab) == 0 && r == '\t') ||
		((flags&VisNl) == 0 && r == '\n') ||
		((flags&VisSafe) != 0 && (r == '\b' || r == '\007' || r == '\r')) {
		if r == '\\' && (flags&VisNoSlash) == 0 {
			return fmt.Sprintf("%s\\", string(r)), nil
		}
		return string(r), nil
	}

	if (flags & VisCstyle) != 0 {
		switch r {
		case '\n':
			return "\\n", nil
		case '\r':
			return "\\r", nil
		case '\b':
			return "\\b", nil
		case '\a':
			return "\\a", nil
		case '\v':
			return "\\v", nil
		case '\t':
			return "\\t", nil
		case '\f':
			return "\\f", nil
		case ' ':
			return "\\s", nil
		case rune(0x0):
			return "\\0", nil
			/*
				if isoctal(nextr) {
					dst = append(dst, '0')
					dst = append(dst, '0')
				}
			*/
		}
	}
	if ((r & 0177) == ' ') || isgraph(r) || (flags&VisOctal) != 0 {
		dst := make([]rune, 4)
		dst[0] = '\\'
		dst[1] = (r >> 6 & 07) + '0'
		dst[2] = (r >> 3 & 07) + '0'
		dst[3] = (r & 07) + '0'
		return string(dst), nil
	}
	var dst []rune
	if (flags & VisNoSlash) == 0 {
		dst = append(dst, '\\')
	}
	if (r & 0200) != 0 {
		r &= 0177
		dst = append(dst, 'M')
	}
	if unicode.IsControl(r) {
		dst = append(dst, '^')
		if r == 0177 {
			dst = append(dst, '?')
		} else {
			dst = append(dst, r+'@')
		}
	} else {
		dst = append(dst, '-')
		dst = append(dst, r)
	}
	return string(dst), nil
}
