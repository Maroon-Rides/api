package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/MaroonRides/api/internal/db/sync"
)

func TestEverySyncableModelIsRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, table := range SyncTables {
		registered[reflect.TypeOf(table.Model).Elem().Name()] = true
	}

	for _, name := range syncableStructs(t) {
		if !registered[name] {
			t.Errorf("%s embeds sync.Syncable but is missing from SyncTables", name)
		}
	}
}

func syncableStructs(t *testing.T) []string {
	t.Helper()

	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", notATest, 0)
	if err != nil {
		t.Fatalf("parse model package: %v", err)
	}

	var names []string
	for _, pkg := range pkgs {
		ast.Inspect(pkg, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if str, ok := spec.Type.(*ast.StructType); ok && embedsSyncable(str) {
				names = append(names, spec.Name.Name)
			}
			return true
		})
	}
	if len(names) == 0 {
		t.Fatal("found no syncable models; the scan is looking in the wrong place")
	}
	return names
}

func embedsSyncable(str *ast.StructType) bool {
	for _, field := range str.Fields.List {
		sel, ok := field.Type.(*ast.SelectorExpr)
		if len(field.Names) == 0 && ok && sel.Sel.Name == "Syncable" {
			return true
		}
	}
	return false
}

func TestSyncStreamsHideNosyncColumns(t *testing.T) {
	var hidden int

	for _, table := range SyncTables {
		for _, field := range columnFields(reflect.TypeOf(table.Model).Elem()) {
			column, _, _ := strings.Cut(field.Tag.Get("bun"), ",")
			if !sync.Ignored(field) {
				if !slices.Contains(table.Synced, column) {
					t.Errorf("%s.%s carries no nosync tag but is missing from the stream", table.Name, column)
				}
				continue
			}
			hidden++

			if slices.Contains(table.Synced, column) {
				t.Errorf("%s publishes %q to clients", table.Name, column)
			}
			if strings.Contains(sync.TriggerDDL(table), column) {
				t.Errorf("%s bumps updateId when %q changes", table.Name, column)
			}
		}
	}

	if hidden == 0 {
		t.Fatal("no synced model tags a column nosync; the scan is looking in the wrong place")
	}
}

// columnFields walks the embedded structs bun folds into one table,
func columnFields(typ reflect.Type) []reflect.StructField {
	var out []reflect.StructField
	for f := range typ.Fields() {
		tag := f.Tag.Get("bun")
		switch {
		case f.Anonymous && f.Type.Kind() == reflect.Struct:
			out = append(out, columnFields(f.Type)...)
		case tag == "" || strings.HasPrefix(tag, "rel:"):
		default:
			out = append(out, f)
		}
	}
	return out
}

func notATest(info fs.FileInfo) bool {
	return !strings.HasSuffix(info.Name(), "_test.go")
}
