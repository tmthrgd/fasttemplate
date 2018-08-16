// Package fasttemplate implements simple and fast template library.
//
// Fasttemplate is faster than text/template, strings.Replace
// and strings.Replacer.
//
// Fasttemplate ideally fits for fast and simple placeholders' substitutions.
package fasttemplate

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Template implements simple template engine, which can be used for fast
// tags' (aka placeholders) substitution.
type Template struct {
	template []byte
	texts    [][]byte
	tags     []string
}

// New parses the given template using the given startTag and endTag
// as tag start and tag end.
//
// The returned template can be executed by concurrently running goroutines
// using Execute* methods.
//
// New panics if the given template cannot be parsed. Use NewTemplate instead
// if template may contain errors.
func New(template, startTag, endTag string) *Template {
	t, err := NewTemplate(template, startTag, endTag)
	if err != nil {
		panic(err)
	}
	return t
}

// NewTemplate parses the given template using the given startTag and endTag
// as tag start and tag end.
//
// The returned template can be executed by concurrently running goroutines
// using Execute* methods.
func NewTemplate(template, startTag, endTag string) (*Template, error) {
	if len(startTag) == 0 {
		panic("fasttemplate: startTag cannot be empty")
	}
	if len(endTag) == 0 {
		panic("fasttemplate: endTag cannot be empty")
	}

	s := []byte(template)
	st := template

	var t Template

	tagsCount := strings.Count(template, startTag)
	if tagsCount == 0 {
		t.template = s
		return &t, nil
	}

	t.texts = make([][]byte, 0, tagsCount+1)
	t.tags = make([]string, 0, tagsCount)

	for {
		n := strings.Index(st, startTag)
		if n < 0 {
			t.texts = append(t.texts, s)
			break
		}
		t.texts = append(t.texts, s[:n])

		s = s[n+len(startTag):]
		st = st[n+len(startTag):]

		n = strings.Index(st, endTag)
		if n < 0 {
			return nil, fmt.Errorf("fasttemplate: missing end tag=%q in template=%q starting from %q", endTag, template, s)
		}

		t.tags = append(t.tags, st[:n])

		s = s[n+len(endTag):]
		st = st[n+len(endTag):]
	}

	return &t, nil
}

// TagFunc can be used as a substitution value in the map passed to Execute*.
// Execute* functions pass tag (placeholder) name in 'tag' argument.
//
// TagFunc must write contents to w and be safe to call from concurrently
// running goroutines.
type TagFunc func(w io.Writer, tag string) error

// ExecuteFunc calls f on each template tag (placeholder) occurrence.
func (t *Template) ExecuteFunc(w io.Writer, f TagFunc) error {
	n := len(t.texts) - 1
	if n == -1 {
		_, err := w.Write(t.template)
		return err
	}

	for i := 0; i < n; i++ {
		if _, err := w.Write(t.texts[i]); err != nil {
			return err
		}

		if err := f(w, t.tags[i]); err != nil {
			return err
		}
	}

	_, err := w.Write(t.texts[n])
	return err
}

// Execute substitutes template tags (placeholders) with the corresponding
// values from the map m and writes the result to the given writer w.
//
// Substitution map m may contain values with the following types:
//   * []byte - the fastest value type
//   * string - convenient value type
//   * TagFunc - flexible value type
func (t *Template) Execute(w io.Writer, m map[string]interface{}) error {
	return t.ExecuteFunc(w, func(w io.Writer, tag string) error {
		return stdTagFunc(w, tag, m)
	})
}

// ExecuteFuncBytes calls f on each template tag (placeholder) occurrence
// and substitutes it with the data written to TagFunc's w.
//
// Returns the resulting byte slice.
func (t *Template) ExecuteFuncBytes(f TagFunc) []byte {
	var buf bytes.Buffer
	buf.Grow(len(t.template))
	if err := t.ExecuteFunc(&buf, f); err != nil {
		panic(fmt.Sprintf("fasttemplate: unexpected error: %s", err))
	}
	return buf.Bytes()
}

// ExecuteBytes substitutes template tags (placeholders) with the corresponding
// values from the map m and returns the result.
//
// Substitution map m may contain values with the following types:
//   * []byte - the fastest value type
//   * string - convenient value type
//   * TagFunc - flexible value type
func (t *Template) ExecuteBytes(m map[string]interface{}) []byte {
	return t.ExecuteFuncBytes(func(w io.Writer, tag string) error {
		return stdTagFunc(w, tag, m)
	})
}

// ExecuteFuncString calls f on each template tag (placeholder) occurrence
// and substitutes it with the data written to TagFunc's w.
//
// Returns the resulting string.
func (t *Template) ExecuteFuncString(f TagFunc) string {
	var sb strings.Builder
	sb.Grow(len(t.template))
	if err := t.ExecuteFunc(&sb, f); err != nil {
		panic(fmt.Sprintf("fasttemplate: unexpected error: %s", err))
	}
	return sb.String()
}

// ExecuteString substitutes template tags (placeholders) with the corresponding
// values from the map m and returns the result.
//
// Substitution map m may contain values with the following types:
//   * []byte - the fastest value type
//   * string - convenient value type
//   * TagFunc - flexible value type
func (t *Template) ExecuteString(m map[string]interface{}) string {
	return t.ExecuteFuncString(func(w io.Writer, tag string) error {
		return stdTagFunc(w, tag, m)
	})
}

func stdTagFunc(w io.Writer, tag string, m map[string]interface{}) error {
	v := m[tag]
	if v == nil {
		return nil
	}
	switch value := v.(type) {
	case []byte:
		_, err := w.Write(value)
		return err
	case string:
		_, err := io.WriteString(w, value)
		return err
	case TagFunc:
		return value(w, tag)
	default:
		panic(fmt.Sprintf("fasttemplate: tag=%q contains unexpected value type=%#v", tag, v))
	}
}
