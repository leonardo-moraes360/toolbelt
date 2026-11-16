package graph

import (
	"path/filepath"
	"testing"
)

func TestTypeScriptReactCallersImpactFixture(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "components", "Layout.tsx"), `
import React from 'react';

export function Layout({ children }: { children: React.ReactNode }) {
  return <div className="layout">{children}</div>;
}
`)
	mustWrite(t, filepath.Join(root, "lib", "utils.ts"), `
export function formatTitle(s: string): string {
  return s.trim();
}

export const slugify = (s: string) => s.toLowerCase().replace(/\s+/g, '-');
`)
	mustWrite(t, filepath.Join(root, "pages", "index.tsx"), `
import React from 'react';
import { Layout } from '../components/Layout';
import { formatTitle, slugify } from '../lib/utils';

export default function Home() {
  const title = formatTitle('Hello');
  return (
    <Layout>
      <h1>{slugify(title)}</h1>
    </Layout>
  );
}

export async function getStaticProps() {
  return { props: { title: formatTitle('Home') } };
}
`)
	mustWrite(t, filepath.Join(root, "pages", "api", "health.ts"), `
import { formatTitle } from '../../lib/utils';

export default function handler() {
  return formatTitle('ok');
}
`)

	opts := DefaultBuildOptions()
	opts.Languages = []string{"javascript"}
	opts.Frameworks = []string{"react", "routes"}
	opts.Docs = []string{}
	idx, err := BuildFromRootOpts(root, &opts)
	if err != nil {
		t.Fatal(err)
	}

	if !hasNode(idx, NodeFunction, "Layout") {
		t.Fatalf("expected Layout function; funcs=%v comps=%v", titles(idx, NodeFunction), titles(idx, NodeComponent))
	}
	if !hasNode(idx, NodeFunction, "formatTitle") || !hasNode(idx, NodeFunction, "slugify") {
		t.Fatalf("expected utils functions; funcs=%v", titles(idx, NodeFunction))
	}
	if !hasNode(idx, NodeFunction, "getStaticProps") || !hasNode(idx, NodeFunction, "handler") {
		t.Fatalf("expected getStaticProps/handler; funcs=%v", titles(idx, NodeFunction))
	}

	store := NewStore(filepath.Join(t.TempDir(), "graph.json"))
	store.index = idx

	layoutCallers := store.Callers("Layout", 1)
	if !callerTitlesContain(layoutCallers, "Home") {
		t.Fatalf("callers(Layout) missing Home; got %#v calls=%v", layoutCallers, callEdges(idx))
	}

	formatCallers := store.Callers("formatTitle", 1)
	if !callerTitlesContain(formatCallers, "Home") || !callerTitlesContain(formatCallers, "getStaticProps") {
		t.Fatalf("callers(formatTitle) missing Home/getStaticProps; got %#v calls=%v", formatCallers, callEdges(idx))
	}
	if !callerTitlesContain(formatCallers, "handler") {
		t.Fatalf("callers(formatTitle) missing API handler; got %#v", formatCallers)
	}

	layoutImpact := store.Impact("components/Layout.tsx", 2)
	if len(layoutImpact) == 0 || !callerTitlesContain(layoutImpact, "Home") {
		t.Fatalf("impact(Layout.tsx) empty/missing Home; got %#v calls=%v", layoutImpact, callEdges(idx))
	}
	utilsImpact := store.Impact("lib/utils.ts", 2)
	if len(utilsImpact) == 0 {
		t.Fatalf("impact(utils.ts) empty; calls=%v", callEdges(idx))
	}

	if !hasResolvedCallTarget(idx, "Layout", "components") {
		t.Fatalf("expected Layout call resolved to components.Layout, not pages.Layout; calls=%v", callEdges(idx))
	}
}

func callerTitlesContain(nodes []Node, title string) bool {
	for _, n := range nodes {
		if n.Title == title {
			return true
		}
	}
	return false
}

func hasResolvedCallTarget(idx *Index, title, wantPkg string) bool {
	byID := map[string]Node{}
	for _, n := range idx.Nodes {
		byID[n.ID] = n
	}
	for _, e := range idx.Edges {
		if e.Type != EdgeCalls {
			continue
		}
		tgt, ok := byID[e.TargetID]
		if !ok || tgt.Title != title {
			continue
		}
		if tgt.Package == wantPkg && e.Confidence >= MinCallsConfidence {
			return true
		}
	}
	return false
}
