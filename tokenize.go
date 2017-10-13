// Copyright (c) 2017, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"fmt"
	"go/scanner"
	"go/token"
	"strings"
)

const (
	_ token.Token = -iota
	tokWild
	tokWildAny
	tokAggressive
)

type fullToken struct {
	pos token.Position
	tok token.Token
	lit string
}

func tokenize(src string) ([]fullToken, error) {
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	var err error
	onError := func(pos token.Position, msg string) {
		switch msg { // allow certain extra chars
		case `illegal character U+0024 '$'`:
		case `illegal character U+007E '~'`:
		default:
			err = fmt.Errorf("%v: %s", pos, msg)
		}
	}

	// we will modify the input source under the scanner's nose to
	// enable some features such as regexes.
	scanSrc := []byte(src)
	s.Init(file, scanSrc, onError, scanner.ScanComments)

	next := func() fullToken {
		pos, tok, lit := s.Scan()
		return fullToken{fset.Position(pos), tok, lit}
	}

	var toks []fullToken
	for t := next(); t.tok != token.EOF; t = next() {
		switch t.lit {
		case "$": // continues below
		case "~":
			toks = append(toks, fullToken{t.pos, tokAggressive, ""})
			continue
		default: // regular Go code
			toks = append(toks, t)
			continue
		}
		wt := fullToken{t.pos, tokWild, ""}
		t = next()
		paren := false
		if paren = t.tok == token.LPAREN; paren {
			t = next()
		}
		if t.tok == token.MUL {
			wt.tok = tokWildAny
			t = next()
		}
		if t.tok != token.IDENT {
			err = fmt.Errorf("%v: $ must be followed by ident, got %v",
				t.pos, t.tok)
			break
		}
		wt.lit = t.lit
		if paren {
			t = next()
			if t.tok == token.QUO {
				start := t.pos.Offset + 1
				rxStr := string(src[start:])
				end := strings.Index(rxStr, "/")
				if end < 0 {
					err = fmt.Errorf("%v: expected / to terminate regex",
						t.pos)
					break
				}
				rxStr = rxStr[:end]
				for i := start; i < start+end; i++ {
					scanSrc[i] = ' '
				}
				t = next() // skip opening /
				if t.tok != token.QUO {
					// skip any following token, as
					// go/scanner retains one char
					// for its next token.
					t = next()
				}
				t = next() // skip closing /
				// TODO: pass rxStr on and use it
			}
			if t.tok != token.RPAREN {
				err = fmt.Errorf("%v: expected ) to close $(",
					t.pos)
				break
			}
		}
		toks = append(toks, wt)
	}
	return toks, err
}
