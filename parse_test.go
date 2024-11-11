package conf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testParse(t *testing.T, data string, ex map[string]any) {
	t.Helper()
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Error: %v\n", err)
	}
	if !reflect.DeepEqual(m, ex) {
		t.Fatalf("Mismatch:\nReceived: '%+v'\nExpected: '%+v'\n", m, ex)
	}

	n, err := ParseWithChecks(data)
	if err != nil {
		t.Fatalf("Error: %v\n", err)
	}
	if !bytes.Equal(marshaled(m), marshaled(n)) {
		t.Fatalf("Mismatch after checks:\nReceived: '%+v'\nExpected: '%+v'\n", marshaled(m), marshaled(n))
	}
}

func marshaled(v any) []byte {
	result, _ := json.Marshal(v)
	return result
}

func TestSimpleTopLevel(t *testing.T) {
	ex := map[string]any{
		"foo": "1", "bar": 2.2, "baz": true, "boo": int64(22),
	}
	testParse(t, "foo='1'; bar=2.2; baz=true; boo=22", ex)
}

func TestVariableParsing(t *testing.T) {
	variables := []struct {
		data string
		ex   map[string]any
	}{
		{
			"index = 22; foo = $index",
			map[string]any{
				"index": int64(22), "foo": int64(22),
			},
		},
		{
			"index = 22; nest { index = 11; foo = $index }; bar = $index",
			map[string]any{
				"index": int64(22),
				"nest": map[string]any{
					"index": int64(11), "foo": int64(11),
				},
				"bar": int64(22),
			},
		},
	}

	for _, tt := range variables {
		testParse(t, tt.data, tt.ex)
	}
}

func TestMissingVariable(t *testing.T) {
	_, err := Parse("foo=$index")
	if err == nil || !strings.Contains(err.Error(), "variable reference") {
		t.Fatalf("Expected error for missing variable, got: %v", err)
	}
}

func TestEnvVariable(t *testing.T) {
	evar := "__UNIQ22__"
	os.Setenv(evar, "22")
	defer os.Unsetenv(evar)

	testParse(t, fmt.Sprintf("foo = $%s", evar), map[string]any{"foo": int64(22)})
}

func TestConvenientNumbers(t *testing.T) {
	ex := map[string]any{
		"k": int64(8 * 1000), "kb": int64(4 * 1024), "ki": int64(3 * 1024),
		"m": int64(1000 * 1000), "mb": int64(2 * 1024 * 1024), "mi": int64(2 * 1024 * 1024),
	}
	testParse(t, `k = 8k; kb = 4kb; ki = 3ki; m = 1m; mb = 2MB; mi = 2Mi`, ex)
}

func TestSample(t *testing.T) {
	sample := `
		foo {
			host { ip = '127.0.0.1'; port = 8080 }
			servers = [ "a.com", "b.com", "c.com" ]
		}
	`
	ex := map[string]any{
		"foo": map[string]any{
			"host":    map[string]any{"ip": "127.0.0.1", "port": int64(8080)},
			"servers": []any{"a.com", "b.com", "c.com"},
		},
	}
	testParse(t, sample, ex)
}

func TestSampleWithTime(t *testing.T) {
	dt, _ := time.Parse(time.RFC3339, "2016-05-04T18:53:41Z")
	ex := map[string]any{"now": dt, "gmt": false}
	testParse(t, "now = 2016-05-04T18:53:41Z; gmt = false", ex)
}

func TestIncludes(t *testing.T) {

	ex := map[string]any{
		"listen": "127.0.0.1:8080",
		"name":   "node0",
		"auth": map[string]any{
			"USER1_PASS": "WSGrnSowBu6QkU9",
			"USER2_PASS": "bo9V4j5B3VTLGns",
			"users": []any{
				map[string]any{
					"user":     "user1",
					"password": "WSGrnSowBu6QkU9"},
				map[string]any{
					"user":     "user2",
					"password": "bo9V4j5B3VTLGns"},
			},
			"timeout": float64(0.5),
		},
	}

	m, err := ParseFile("sample.conf")
	if err != nil {
		t.Fatalf("Error: %v\n", err)
	}
	if !reflect.DeepEqual(m, ex) {
		t.Fatalf("Mismatch:\nReceived: '%+v'\nExpected: '%+v'\n", m, ex)
	}
}

func TestParseErrorHandling(t *testing.T) {
	for _, conf := range []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"              aaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"     aaaaaaaaaaaaaaaaaaaaaaaaaaa         ",
		`# just comments with no values
		aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`,
		"    a,a,a,a,a,a,a,a,a,a,a",
	} {
		_, err := parseData(conf, "", true)
		if err == nil {
			t.Errorf("Expected error for config: %s", conf)
		}
	}
}

func TestJSONParseCompat(t *testing.T) {
	for _, test := range []struct {
		name     string
		input    string
		includes map[string]string
		expected map[string]any
	}{
		{
			"JSON empty object in one line",
			`{}`,
			nil,
			map[string]any{},
		},
		{
			"JSON empty object with line breaks",
			`
                        {
                        }
                        `,
			nil,
			map[string]any{},
		},
		{
			"JSON includes",
			`
                        accounts {
                          foo  { include 'foo.json'  }
                          bar  { include 'bar.json'  }
                          quux { include 'quux.json' }
                        }
                        `,
			map[string]string{
				"foo.json": `{ "users": [ {"user": "foo"} ] }`,
				"bar.json": `{
                                  "users": [ {"user": "bar"} ]
                                }`,
				"quux.json": `{}`,
			},
			map[string]any{
				"accounts": map[string]any{
					"foo": map[string]any{
						"users": []any{
							map[string]any{
								"user": "foo",
							},
						},
					},
					"bar": map[string]any{
						"users": []any{
							map[string]any{
								"user": "bar",
							},
						},
					},
					"quux": map[string]any{},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			sdir := t.TempDir()
			f, err := os.CreateTemp(sdir, "nats.conf-")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(f.Name(), []byte(test.input), 066); err != nil {
				t.Error(err)
			}
			if test.includes != nil {
				for includeFile, contents := range test.includes {
					inf, err := os.Create(filepath.Join(sdir, includeFile))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(inf.Name(), []byte(contents), 066); err != nil {
						t.Error(err)
					}
				}
			}
			m, err := ParseFile(f.Name())
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(m, test.expected) {
				t.Fatalf("Not Equal:\nReceived: '%+v'\nExpected: '%+v'\n", m, test.expected)
			}
		})
	}
}
