// +build !cvis

package mtree

import "unicode"

func unvis(src string) (string, error) {
	dst := []rune{}
	var s state
	for i, r := range src {
	again:
		err := unvisRune(&dst, r, &s, 0)
		switch err {
		case unvisValid:
			break
		case unvisValidPush:
			goto again
		case unvisNone:
			fallthrough
		case unvisNochar:
			break
		default:
			return "", err
		}
		if i == len(src)-1 {
			unvisRune(&dst, r, &s, unvisEnd)
		}
	}

	str := ""
	for _, ch := range dst {
		str += string(ch)
	}
	return str, nil
}

func unvisRune(dst *[]rune, r rune, s *state, flags VisFlag) error {
	if (flags & unvisEnd) != 0 {
		if *s == stateOctal2 || *s == stateOctal3 {
			*s = stateGround
			return unvisValid
		}
		if *s == stateGround {
			return unvisNochar
		}
		return unvisErrSynbad
	}

	switch *s & ^stateHTTP {
	case stateGround:
		if r == '\\' {
			*s = stateStart
			return unvisNone
		}
		if flags&VisHttpstyle != 0 && r == '%' {
			*s = stateStart | stateHTTP
			return unvisNone
		}
		*dst = append(*dst, r)
		return unvisValid
	case stateStart:
		if *s&stateHTTP != 0 && ishex(unicode.ToLower(r)) {
			if unicode.IsNumber(r) {
				*dst = append(*dst, r-'0')
			} else {
				*dst = append(*dst, unicode.ToLower(r)-'a')
			}
			*s = stateHex2
			return unvisNone
		}
		switch r {
		case '\\':
			*s = stateGround
			*dst = append(*dst, r)
			return unvisValid
		case '0':
			fallthrough
		case '1':
			fallthrough
		case '2':
			fallthrough
		case '3':
			fallthrough
		case '4':
			fallthrough
		case '5':
			fallthrough
		case '6':
			fallthrough
		case '7':
			*s = stateOctal2
			*dst = append(*dst, r-'0')
			return unvisNone
		case 'M':
			*s = stateMeta
			*dst = append(*dst, rune(0200))
			return unvisNone
		case '^':
			*s = stateCtrl
			return unvisNone
		case 'n':
			*s = stateGround
			*dst = append(*dst, '\n')
			return unvisValid
		case 'r':
			*s = stateGround
			*dst = append(*dst, '\r')
			return unvisValid
		case 'b':
			*s = stateGround
			*dst = append(*dst, '\b')
			return unvisValid
		case 'a':
			*s = stateGround
			*dst = append(*dst, '\007')
			return unvisValid
		case 'v':
			*s = stateGround
			*dst = append(*dst, '\v')
			return unvisValid
		case 't':
			*s = stateGround
			*dst = append(*dst, '\t')
			return unvisValid
		case 'f':
			*s = stateGround
			*dst = append(*dst, '\f')
			return unvisValid
		case 's':
			*s = stateGround
			*dst = append(*dst, ' ')
			return unvisValid
		case 'E':
			*s = stateGround
			*dst = append(*dst, '\033')
			return unvisValid
		case '\n':
			// hidden newline
			*s = stateGround
			return unvisNochar
		case '$':
			// hidden marker
			*s = stateGround
			return unvisNochar
		}
		*s = stateGround
		return unvisErrSynbad
	case stateMeta:
		if r == '-' {
			*s = stateMeta1
		} else if r == '^' {
			*s = stateCtrl
		} else {
			*s = stateGround
			return unvisErrSynbad
		}
		return unvisNone
	case stateMeta1:
		*s = stateGround
		dp := *dst
		dp[len(dp)-1] |= r
		return unvisValid
	case stateCtrl:
		dp := *dst
		if r == '?' {
			dp[len(dp)-1] |= rune(0177)
		} else {
			dp[len(dp)-1] |= r & 037
		}
		*s = stateGround
		return unvisValid
	case stateOctal2:
		if isoctal(r) {
			dp := *dst
			if len(dp) > 0 {
				last := dp[len(dp)-1]
				dp[len(dp)-1] = (last << 3) + (r - '0')
			} else {
				dp = append(dp, (0<<3)+(r-'0'))
			}
			*s = stateOctal3
			return unvisNone
		}
		*s = stateGround
		return unvisValidPush
	case stateOctal3:
		*s = stateGround
		if isoctal(r) {
			dp := *dst
			if len(dp) > 0 {
				last := dp[len(dp)-1]
				dp[len(dp)-1] = (last << 3) + (r - '0')
			} else {
				dp = append(dp, (0<<3)+(r-'0'))
			}
			return unvisValid
		}
		return unvisValidPush
	case stateHex2:
		if ishex(unicode.ToLower(r)) {
			last := rune(0)
			dp := *dst
			if len(dp) > 0 {
				last = dp[len(dp)-1]
			}
			if unicode.IsNumber(r) {
				dp = append(dp, (last<<4)+(r-'0'))
			} else {
				dp = append(dp, (last<<4)+(unicode.ToLower(r)-'a'+10))
			}
		}
		*s = stateGround
		return unvisValid
	}

	*s = stateGround
	return unvisErrSynbad
}

type state int

const (
	stateGround state = iota /* haven't seen escape char */
	stateStart               /* start decoding special sequence */
	stateMeta                /* metachar started (M) */
	stateMeta1               /* metachar more, regular char (-) */
	stateCtrl                /* control char started (^) */
	stateOctal2              /* octal digit 2 */
	stateOctal3              /* octal digit 3 */
	stateHex2                /* hex digit 2 */

	stateHTTP state = 0x080 /* %HEXHEX escape */
)
