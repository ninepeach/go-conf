package conf

import (
 "errors"
 "os"
 "path/filepath"
 "strings"
 "strconv"
 "testing"
)

func TestConfigSizeLimit(t *testing.T) {
 _, err := Parse(strings.Repeat(" ", maxConfigBytes+1))
 if !errors.Is(err, ErrConfigTooLarge) { t.Fatalf("expected ErrConfigTooLarge, got %v", err) }
 p := filepath.Join(t.TempDir(), "large.conf")
 if err := os.WriteFile(p, []byte(strings.Repeat(" ", maxConfigBytes+1)), 0600); err != nil { t.Fatal(err) }
 _, err = ParseFile(p)
 if !errors.Is(err, ErrConfigTooLarge) { t.Fatalf("expected ErrConfigTooLarge, got %v", err) }
}

func TestIncludeDepthLimit(t *testing.T) {
 dir := t.TempDir()
 for i:=0; i<maxIncludeDepth+1; i++ {
  path := filepath.Join(dir, stringFileName(i))
  data := "value = 1\n"
  if i<maxIncludeDepth { data = "include '" + stringFileName(i+1) + "'\n" }
  if err := os.WriteFile(path, []byte(data), 0600); err != nil { t.Fatal(err) }
 }
 _, err := ParseFile(filepath.Join(dir, stringFileName(0)))
 if !errors.Is(err, ErrIncludeDepth) && (err == nil || !strings.Contains(err.Error(), ErrIncludeDepth.Error())) {
  t.Fatalf("expected depth limit, got %v", err)
 }
}

func stringFileName(i int) string { return strconv.Itoa(i) + ".conf" }

func FuzzParse(f *testing.F) {
 for _, s := range []string{"", "a = 1", "a = [1,2,3]", "a { b = true }", "{", "[", "\"", "a = $MISSING", "a = 9223372036854775807k"} { f.Add(s) }
 f.Fuzz(func(t *testing.T, data string) {
  if len(data)>maxConfigBytes { t.Skip() }
  _, _ = Parse(data)
  _, _ = ParseWithChecks(data)
 })
}
