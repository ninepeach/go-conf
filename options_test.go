package conf

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestStrictDuplicateKeys(t *testing.T) {
 for _, input := range []string{"a=1; a=2", "nested { a=1; a=2 }"} {
  if _, err := ParseWithOptions(input, Options{RejectDuplicateKeys:true}); err == nil || !strings.Contains(err.Error(), "duplicate key") { t.Fatalf("expected duplicate error for %q: %v", input, err) }
 }
 if _, err := Parse("a=1; a=2"); err != nil { t.Fatalf("legacy override failed: %v", err) }
}

func TestEnvironmentAsString(t *testing.T) {
 t.Setenv("GO_CONF_OPTION_TEST", "123")
 m, err := ParseWithOptions("value=$GO_CONF_OPTION_TEST", Options{EnvAsString:true})
 if err != nil { t.Fatal(err) }
 if got := m["value"]; got != "123" { t.Fatalf("want literal string, got %#v", got) }
 legacy, err := Parse("value=$GO_CONF_OPTION_TEST")
 if err != nil { t.Fatal(err) }
 if legacy["value"] != int64(123) { t.Fatalf("legacy typed parsing changed: %#v", legacy["value"]) }
}

func TestIncludeRoot(t *testing.T) {
 dir := t.TempDir()
 inside := filepath.Join(dir, "inside")
 if err := os.Mkdir(inside, 0700); err != nil { t.Fatal(err) }
 child := filepath.Join(inside, "child.conf")
 outside := filepath.Join(dir, "outside.conf")
 root := filepath.Join(inside, "root.conf")
 if err := os.WriteFile(child, []byte("x=1"), 0600); err != nil { t.Fatal(err) }
 if err := os.WriteFile(outside, []byte("x=2"), 0600); err != nil { t.Fatal(err) }
 if err := os.WriteFile(root, []byte("include 'child.conf'"), 0600); err != nil { t.Fatal(err) }
 if _, err := ParseFileWithOptions(root, Options{IncludeRoot:inside}); err != nil { t.Fatal(err) }
 if err := os.WriteFile(root, []byte("include '../outside.conf'"), 0600); err != nil { t.Fatal(err) }
 if _, err := ParseFileWithOptions(root, Options{IncludeRoot:inside}); err == nil { t.Fatal("expected path escape rejection") }
 if _, err := ParseFileWithOptions(outside, Options{IncludeRoot:inside}); err == nil { t.Fatal("expected initial file rejection") }
}
