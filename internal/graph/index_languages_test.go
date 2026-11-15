package graph

import (
	"path/filepath"
	"testing"
)

func TestIndexLanguagesExtractSymbols(t *testing.T) {
	cases := []struct {
		lang    string
		path    string
		content string
		wantFn  string
		wantCls string
		wantImp string
	}{
		{
			lang: "python", path: "app/main.py",
			content: "import os\nfrom pathlib import Path\n\nclass Worker:\n    def run(self):\n        return os.getcwd()\n\ndef helper():\n    return 1\n",
			wantFn:  "helper", wantCls: "Worker", wantImp: "os",
		},
		{
			lang: "java", path: "src/Hello.java",
			content: "package demo;\nimport java.util.List;\npublic class Hello {\n  public int add(int a, int b) { return a + b; }\n}\n",
			wantFn:  "add", wantCls: "Hello", wantImp: "java.util.List",
		},
		{
			lang: "kotlin", path: "src/Main.kt",
			content: "import kotlin.io.Console\ndata class User(val id: Int)\nfun greet(name: String): String { return name }\n",
			wantFn:  "greet", wantCls: "User", wantImp: "kotlin.io.Console",
		},
		{
			lang: "csharp", path: "src/Program.cs",
			content: "using System;\nnamespace App {\n  public class Program {\n    public static void Main() {}\n  }\n}\n",
			wantFn:  "Main", wantCls: "Program", wantImp: "System",
		},
		{
			lang: "cpp", path: "src/math.cpp",
			content: "#include <vector>\nclass Calc {\npublic:\n  int scale(int v);\n};\nint scale(int v) {\n  return v * 2;\n}\n",
			wantFn:  "scale", wantCls: "Calc", wantImp: "vector",
		},
		{
			lang: "rust", path: "src/lib.rs",
			content: "use std::fs;\npub struct Config;\npub fn load() -> Config { Config }\n",
			wantFn:  "load", wantCls: "Config", wantImp: "std::fs",
		},
		{
			lang: "php", path: "src/User.php",
			content: "<?php\nuse App\\Models\\Base;\nclass User {\n  public function save() {}\n}\nfunction boot() {}\n",
			wantFn:  "save", wantCls: "User", wantImp: "App\\Models\\Base",
		},
		{
			lang: "ruby", path: "lib/user.rb",
			content: "require 'json'\nclass User\n  def save\n  end\nend\ndef boot\nend\n",
			wantFn:  "save", wantCls: "User", wantImp: "json",
		},
		{
			lang: "swift", path: "Sources/App.swift",
			content: "import Foundation\nclass Service {\n  func start() {}\n}\nfunc boot() {}\n",
			wantFn:  "start", wantCls: "Service", wantImp: "Foundation",
		},
		{
			lang: "shell", path: "scripts/build.sh",
			content: "#!/bin/bash\nsource ./lib.sh\ngreet() {\n  echo hi\n}\n",
			wantFn:  "greet", wantImp: "./lib.sh",
		},
	}

	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			root := t.TempDir()
			mustWrite(t, filepath.Join(root, tc.path), tc.content)
			opts := DefaultBuildOptions()
			opts.Languages = []string{tc.lang}
			opts.Docs = []string{}
			idx, err := BuildFromRootOpts(root, &opts)
			if err != nil {
				t.Fatal(err)
			}
			if !hasNode(idx, NodeFunction, tc.wantFn) && tc.wantFn != "" {
				t.Fatalf("missing function %q nodes=%v", tc.wantFn, titles(idx, NodeFunction))
			}
			if tc.wantCls != "" && !hasNode(idx, NodeClass, tc.wantCls) {
				t.Fatalf("missing class %q nodes=%v", tc.wantCls, titles(idx, NodeClass))
			}
			if tc.wantImp != "" && !hasImport(idx, tc.wantImp) {
				t.Fatalf("missing import %q", tc.wantImp)
			}
		})
	}
}

func TestBuildOptionsFilterLanguages(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.py"), "def foo():\n    pass\n")
	mustWrite(t, filepath.Join(root, "a.go"), "package a\nfunc Bar() {}\n")
	opts := DefaultBuildOptions()
	opts.Languages = []string{"python"}
	opts.Docs = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNode(idx, NodeFunction, "foo") {
		t.Fatal("expected python foo")
	}
	if hasNode(idx, NodeFunction, "Bar") {
		t.Fatal("go should be filtered out")
	}
}

func TestSkipLargeFiles(t *testing.T) {
	root := t.TempDir()
	big := make([]byte, 2048)
	for i := range big {
		big[i] = 'a'
	}
	mustWrite(t, filepath.Join(root, "big.py"), "def keep():\n    pass\n"+string(big)+"\ndef hidden():\n    pass\n")
	opts := DefaultBuildOptions()
	opts.Languages = []string{"python"}
	opts.Docs = []string{}
	opts.MaxFileBytes = 100
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if hasNode(idx, NodeFunction, "keep") || hasNode(idx, NodeFunction, "hidden") {
		t.Fatal("large file should be skipped")
	}
}

func hasNode(idx *Index, typ NodeType, title string) bool {
	for _, n := range idx.Nodes {
		if n.Type == typ && n.Title == title {
			return true
		}
	}
	return false
}

func hasImport(idx *Index, title string) bool {
	for _, n := range idx.Nodes {
		if n.Type == NodePackage && n.Title == title {
			return true
		}
	}
	for _, e := range idx.Edges {
		if e.Type != EdgeImports {
			continue
		}
		for _, n := range idx.Nodes {
			if n.ID == e.TargetID && n.Title == title {
				return true
			}
		}
	}
	return false
}

func titles(idx *Index, typ NodeType) []string {
	var out []string
	for _, n := range idx.Nodes {
		if n.Type == typ {
			out = append(out, n.Title)
		}
	}
	return out
}
