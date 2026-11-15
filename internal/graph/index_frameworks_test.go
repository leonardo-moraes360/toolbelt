package graph

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReactIndexerComponentsHooksRenders(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src", "Widget.tsx"), `
import React from 'react';
import { Button } from './Button';

export function Widget() {
  return <Button />;
}

export const useWidgetData = () => 1;

const Banner: React.FC = () => <div />;
`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"javascript"}
	opts.Frameworks = []string{"react"}
	opts.Docs = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNode(idx, NodeComponent, "Widget") {
		t.Fatalf("missing Widget component: %v", titles(idx, NodeComponent))
	}
	if !hasNode(idx, NodeHook, "useWidgetData") {
		t.Fatalf("missing hook: %v", titles(idx, NodeHook))
	}
	if !hasEdgeType(idx, EdgeRenders) {
		t.Fatal("expected renders edge")
	}
}

func TestVueIndexer(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "components", "Hello.vue"), `
<script setup>
import FooBar from './FooBar.vue'
</script>
<template>
  <FooBar />
  <foo-baz />
</template>
`)
	opts := frameworkOnly("vue")
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNode(idx, NodeComponent, "Hello") {
		t.Fatalf("missing Hello: %v", titles(idx, NodeComponent))
	}
	if !hasImport(idx, "./FooBar.vue") {
		t.Fatal("missing vue import")
	}
	if !hasEdgeType(idx, EdgeRenders) {
		t.Fatal("expected renders from template")
	}
}

func TestSvelteIndexer(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "lib", "Card.svelte"), `
<script>
  import Badge from './Badge.svelte';
</script>
<Badge />
`)
	opts := frameworkOnly("svelte")
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNode(idx, NodeComponent, "Card") {
		t.Fatal("missing Card")
	}
	if !hasImport(idx, "./Badge.svelte") {
		t.Fatal("missing svelte import")
	}
	if !hasEdgeType(idx, EdgeRenders) {
		t.Fatal("expected renders")
	}
}

func TestAstroIndexer(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "components", "Hero.astro"), `---
import CTA from './CTA.astro';
---
<section><CTA /></section>
`)
	opts := frameworkOnly("astro")
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNode(idx, NodeComponent, "Hero") {
		t.Fatal("missing Hero")
	}
	if !hasImport(idx, "./CTA.astro") {
		t.Fatal("missing astro import")
	}
	if !hasEdgeType(idx, EdgeRenders) {
		t.Fatal("expected renders")
	}
}

func TestAngularIndexer(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "app", "hello.component.ts"), `
import { Component } from '@angular/core';

@Component({
  selector: 'app-hello',
  templateUrl: './hello.component.html'
})
export class HelloComponent {}
`)
	mustWrite(t, filepath.Join(root, "app", "hello.component.html"), `<app-child></app-child>`)
	opts := frameworkOnly("angular")
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNode(idx, NodeComponent, "HelloComponent") {
		t.Fatalf("missing HelloComponent: %v", titles(idx, NodeComponent))
	}
	if !hasNode(idx, NodeComponent, "app-hello") {
		t.Fatal("missing selector node")
	}
	if !hasEdgeType(idx, EdgeTemplate) {
		t.Fatal("expected template edge")
	}
}

func TestRoutesIndexerNextAndSvelteKit(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "app", "blog", "[slug]", "page.tsx"), `export default function Page(){ return null }`)
	mustWrite(t, filepath.Join(root, "src", "routes", "docs", "[id]", "+page.svelte"), `<h1>doc</h1>`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{}
	opts.Docs = []string{}
	opts.Frameworks = []string{"routes"}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	routes := titles(idx, NodeRoute)
	if !containsString(routes, "/blog/:slug") {
		t.Fatalf("expected next route, got %v", routes)
	}
	if !containsString(routes, "/docs/:id") {
		t.Fatalf("expected sveltekit route, got %v", routes)
	}
}

func TestMDXIndexerDocWithComponents(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "content", "intro.mdx"), `import { Callout } from './Callout'

# Intro

<Callout />
`)
	opts := frameworkOnly("mdx")
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNode(idx, NodeSection, "Intro") {
		t.Fatal("missing MDX heading")
	}
	if !hasImport(idx, "./Callout") {
		t.Fatal("missing MDX import")
	}
	if !hasEdgeType(idx, EdgeRenders) {
		t.Fatal("expected component render edge")
	}
}

func TestFrameworksFilter(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "A.tsx"), `export function A(){ return <B/> }`)
	mustWrite(t, filepath.Join(root, "B.vue"), `<template><div/></template>`)
	opts := DefaultBuildOptions()
	opts.Languages = []string{"javascript"}
	opts.Docs = []string{}
	opts.Frameworks = []string{"react"}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNode(idx, NodeComponent, "A") {
		t.Fatal("react should index")
	}
	for _, n := range idx.Nodes {
		if strings.HasSuffix(n.Path, ".vue") {
			t.Fatal("vue should be filtered out")
		}
	}
}

func frameworkOnly(name string) BuildOptions {
	opts := DefaultBuildOptions()
	opts.Languages = []string{"go"}
	opts.Docs = []string{}
	opts.Frameworks = []string{name}
	return opts
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
