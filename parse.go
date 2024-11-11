package conf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type parser struct {
	mapping  map[string]any
	lx       *lexer
	ctx      any
	ctxs     []any
	keys     []string
	ikeys    []item
	fp       string
	pedantic bool
}

func Parse(data string) (map[string]any, error) {
	p, err := parseData(data, "", false)
	if err != nil {
		return nil, err
	}
	return p.mapping, nil
}

func ParseWithChecks(data string) (map[string]any, error) {
	p, err := parseData(data, "", true)
	if err != nil {
		return nil, err
	}
	return p.mapping, nil
}

func ParseFile(fp string) (map[string]any, error) {
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %v", err)
	}
	p, err := parseData(string(data), fp, false)
	if err != nil {
		return nil, err
	}
	return p.mapping, nil
}

func ParseFileWithChecks(fp string) (map[string]any, error) {
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil, err
	}

	p, err := parseData(string(data), fp, true)
	if err != nil {
		return nil, err
	}

	return p.mapping, nil
}

func parseData(data, fp string, pedantic bool) (p *parser, err error) {
	p = &parser{
		mapping:  make(map[string]any),
		lx:       lex(data),
		ctxs:     []any{make(map[string]any)},
		keys:     make([]string, 0),
		ikeys:    make([]item, 0),
		fp:       filepath.Dir(fp),
		pedantic: pedantic,
	}

	p.pushContext(p.mapping)

	var prevItem item
	for {
		it := p.next()
		if it.typ == itemEOF && prevItem.typ == itemKey && prevItem.val != mapEndString {
			return nil, fmt.Errorf("config is invalid (%s:%d:%d)", fp, it.line, it.pos)
		}
		prevItem = it
		if err := p.processItem(it, fp); err != nil {
			return nil, err
		}
		if it.typ == itemEOF {
			break
		}
	}
	return p, nil
}

func (p *parser) next() item {
	return p.lx.nextItem()
}

func (p *parser) pushContext(ctx any) {
	p.ctxs = append(p.ctxs, ctx)
	p.ctx = ctx
}

func (p *parser) popContext() any {
	if len(p.ctxs) == 0 {
		panic("BUG: empty context stack")
	}
	last := p.ctxs[len(p.ctxs)-1]
	p.ctxs = p.ctxs[:len(p.ctxs)-1]
	p.ctx = p.ctxs[len(p.ctxs)-1]
	return last
}

func (p *parser) pushKey(key string) {
	p.keys = append(p.keys, key)
}

func (p *parser) popKey() string {
	if len(p.keys) == 0 {
		panic("BUG: empty keys stack")
	}
	last := p.keys[len(p.keys)-1]
	p.keys = p.keys[:len(p.keys)-1]
	return last
}

func (p *parser) pushItemKey(key item) {
	p.ikeys = append(p.ikeys, key)
}

func (p *parser) popItemKey() item {
	if len(p.ikeys) == 0 {
		panic("BUG: empty item keys stack")
	}
	last := p.ikeys[len(p.ikeys)-1]
	p.ikeys = p.ikeys[:len(p.ikeys)-1]
	return last
}

func (p *parser) processItem(it item, fp string) error {
	setValue := func(it item, v any) {
		if p.pedantic {
			p.setValue(&token{it, v, false, fp})
		} else {
			p.setValue(v)
		}
	}

	switch it.typ {
	case itemError:
		return fmt.Errorf("Parse error on line %d: '%s'", it.line, it.val)
	case itemKey:
		p.pushKey(it.val)
		if p.pedantic {
			p.pushItemKey(it)
		}
	case itemMapStart:
		newCtx := make(map[string]any)
		p.pushContext(newCtx)
	case itemMapEnd:
		setValue(it, p.popContext())
	case itemString:
		setValue(it, it.val)
	case itemInteger:
		num, err := parseInteger(it.val)
		if err != nil {
			return err
		}
		setValue(it, num)
	case itemFloat:
		num, err := strconv.ParseFloat(it.val, 64)
		if err != nil {
			return fmt.Errorf("expected float, but got '%s'", it.val)
		}
		setValue(it, num)
	case itemBool:
		setValue(it, parseBool(it.val))
	case itemDatetime:
		dt, err := time.Parse("2006-01-02T15:04:05Z", it.val)
		if err != nil {
			return fmt.Errorf("invalid DateTime: '%s'", it.val)
		}
		setValue(it, dt)
	case itemArrayStart:
		p.pushContext([]any{})
	case itemArrayEnd:
		setValue(it, p.popContext())
	case itemVariable:
		value, found, err := p.lookupVariable(it.val)
		if err != nil {
			return fmt.Errorf("variable reference for '%s' on line %d could not be parsed: %s",
				it.val, it.line, err)
		}
		if !found {
			return fmt.Errorf("variable reference for '%s' on line %d can not be found",
				it.val, it.line)
		}

		if p.pedantic {
			switch tk := value.(type) {
			case *token:
				// Mark the looked up variable as used, and make
				// the variable reference become handled as a token.
				tk.usedVariable = true
				p.setValue(&token{it, tk.Value(), false, fp})
			default:
				// Special case to add position context to bcrypt references.
				p.setValue(&token{it, value, false, fp})
			}
		} else {
			p.setValue(value)
		}
	case itemInclude:
		m, err := parseIncludeFile(p, it.val)
		if err != nil {
			return fmt.Errorf("error parsing include file '%s', %v", it.val, err)
		}
		for k, v := range m {
			p.pushKey(k)
			if p.pedantic {
				switch tk := v.(type) {
				case *token:
					p.pushItemKey(tk.item)
				}
			}
			p.setValue(v)
		}
	}

	return nil
}

// parseNumberSuffix extracts the numeric part and the suffix from a string like "100k" or "2.5g".
func parseNumberSuffix(val string) (string, string) {
	var suffix string
	for i := len(val) - 1; i >= 0; i-- {
		if !unicode.IsLetter(rune(val[i])) {
			return val[:i+1], val[i+1:]
		}
		suffix = string(val[i]) + suffix
	}
	lowerSuffix := strings.ToLower(suffix)
	return val, lowerSuffix
}

func parseInteger(val string) (any, error) {
	numStr, suffix := parseNumberSuffix(val)
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer '%s'", val)
	}
	return applySuffix(num, suffix), nil
}

func applySuffix(num int64, suffix string) any {
	suffix = strings.ToLower(suffix)

	switch suffix {
	case "k":
		return num * 1000
	case "m":
		return num * 1000 * 1000
	case "g":
		return num * 1000 * 1000 * 1000
	case "t":
		return num * 1000 * 1000 * 1000 * 1000
	case "kb", "ki", "kib":
		return num * 1024
	case "mb", "mi", "mib":
		return num * 1024 * 1024
	case "gb", "gi", "gib":
		return num * 1024 * 1024 * 1024
	case "tb", "ti", "tib":
		return num * 1024 * 1024 * 1024 * 1024
	case "p":
		return num * 1000 * 1000 * 1000 * 1000 * 1000
	case "pb", "pi", "pib":
		return num * 1024 * 1024 * 1024 * 1024 * 1024
	case "e":
		return num * 1000 * 1000 * 1000 * 1000 * 1000 * 1000
	case "eb", "ei", "eib":
		return num * 1024 * 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return num
	}
}

func parseBool(val string) bool {
	switch strings.ToLower(val) {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	}
	return false
}

// Used to map an environment value into a temporary map to pass to secondary Parse call.
const pkey = "pk"

// We special case raw strings here that are bcrypt'd. This allows us not to force quoting the strings
const bcryptPrefix = "2a$"

func (p *parser) lookupVariable(varReference string) (any, bool, error) {
	// Handle special cases like bcrypt, then check contexts and env vars.
	if strings.HasPrefix(varReference, bcryptPrefix) {
		return "$" + varReference, true, nil
	}
	for i := len(p.ctxs) - 1; i >= 0; i-- {
		ctx := p.ctxs[i]
		if m, ok := ctx.(map[string]any); ok {
			if v, ok := m[varReference]; ok {
				return v, ok, nil
			}
		}
	}
	if vStr, ok := os.LookupEnv(varReference); ok {
		if vmap, err := Parse(fmt.Sprintf("%s=%s", pkey, vStr)); err == nil {
			v, ok := vmap[pkey]
			return v, ok, nil
		} else {
			return nil, false, err
		}
	}
	return nil, false, nil
}

func parseIncludeFile(p *parser, fileName string) (map[string]any, error) {
	var m map[string]any
	var err error // Declare err outside the if block

	if p.pedantic {
		m, err = ParseFileWithChecks(filepath.Join(p.fp, fileName)) // Assign error to the variable
	} else {
		m, err = ParseFile(filepath.Join(p.fp, fileName)) // Assign error to the variable
	}

	// Return both the map and the error
	return m, err
}

func (p *parser) setValue(val any) {
	// Test to see if we are on an array or a map

	// Array processing
	if ctx, ok := p.ctx.([]any); ok {
		p.ctx = append(ctx, val)
		p.ctxs[len(p.ctxs)-1] = p.ctx
	}

	// Map processing
	if ctx, ok := p.ctx.(map[string]any); ok {
		key := p.popKey()

		if p.pedantic {
			// Change the position to the beginning of the key
			// since more useful when reporting errors.
			switch v := val.(type) {
			case *token:
				it := p.popItemKey()
				v.item.pos = it.pos
				v.item.line = it.line
				ctx[key] = v
			}
		} else {
			// FIXME(dlc), make sure to error if redefining same key?
			ctx[key] = val
		}
	}
}

type token struct {
	item         item
	value        any
	usedVariable bool
	sourceFile   string
}

func (t *token) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.value)
}

func (t *token) Value() any {
	return t.value
}

func (t *token) Line() int {
	return t.item.line
}

func (t *token) IsUsedVariable() bool {
	return t.usedVariable
}

func (t *token) SourceFile() string {
	return t.sourceFile
}

func (t *token) Position() int {
	return t.item.pos
}
