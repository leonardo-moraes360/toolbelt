package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

// Node states, in the order they are decided (docs/specs/agent-hierarchy.md R12).
const (
	StateRevoked   = "revoked"
	StateCompleted = "completed"
	StateReleased  = "released"
	StateClaimed   = "claimed"
	StateExpired   = "expired"
	StateAssigned  = "assigned"
)

// DefaultTreeLimit is the number of roots shown when the caller passes none.
const DefaultTreeLimit = 50

// TreeNode is one thread of a delegation tree. It carries no payload, result
// content, token or Agent Card session: it is safe to print.
type TreeNode struct {
	Thread       string `json:"thread"`
	Parent       string `json:"parent,omitempty"`
	Depth        int    `json:"depth"`
	State        string `json:"state"`
	Owner        string `json:"owner,omitempty"`
	Epoch        int64  `json:"epoch,omitempty"`
	AssignedBy   string `json:"assigned_by"`
	AssignID     string `json:"assign_id"`
	OpenChildren int    `json:"open_children,omitempty"`
}

// TreeResult lists nodes depth-first: each root, then its children in
// assignment order.
type TreeResult struct {
	Channel string     `json:"channel"`
	Nodes   []TreeNode `json:"nodes"`
}

// treeReader is what the tree loaders need from a read transaction.
type treeReader interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// connReader runs the loaders on one connection inside a manual transaction.
type connReader struct {
	ctx context.Context
	c   *sql.Conn
}

func (r connReader) Query(q string, a ...any) (*sql.Rows, error) {
	return r.c.QueryContext(r.ctx, q, a...)
}
func (r connReader) QueryRow(q string, a ...any) *sql.Row { return r.c.QueryRowContext(r.ctx, q, a...) }

type assignInfo struct {
	seq    int64
	by, id string
	live   bool // some task.assign on the thread has not expired by ttl
}

// Tree builds the delegation trees of a channel. root limits the result to
// that thread's subtree; all also lists roots without children; limit caps the
// roots, newest first (0 = DefaultTreeLimit). It only reads, so it works on an
// expired channel.
func (s *Store) Tree(channel, root string, all bool, limit int) (TreeResult, error) {
	if limit <= 0 {
		limit = DefaultTreeLimit
	}
	// The store opens with _txlock=immediate, which would take the write lock
	// and fail on a read-only bus.db. A deferred transaction on a dedicated
	// connection is a plain WAL read snapshot.
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return TreeResult{}, err
	}
	defer conn.Close() //nolint:errcheck // read-only
	if _, err := conn.ExecContext(ctx, "BEGIN DEFERRED"); err != nil {
		return TreeResult{}, err
	}
	defer conn.ExecContext(ctx, "ROLLBACK") //nolint:errcheck // read-only
	tx := connReader{ctx: ctx, c: conn}
	var one int
	if err := tx.QueryRow(`SELECT 1 FROM channels WHERE name = ?`, channel).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TreeResult{}, fmt.Errorf("channel %q not found", channel)
		}
		return TreeResult{}, err
	}
	g, err := loadGraph(tx, channel, s.nowNS())
	if err != nil {
		return TreeResult{}, err
	}
	roots, err := g.pickRoots(root, all, limit)
	if err != nil {
		return TreeResult{}, err
	}
	res := TreeResult{Channel: channel, Nodes: []TreeNode{}}
	var walk func(th string)
	walk = func(th string) {
		res.Nodes = append(res.Nodes, *g.nodes[th])
		for _, k := range g.children[th] {
			walk(k)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	return res, nil
}

// treeGraph is the channel's delegation state, read in one snapshot.
type treeGraph struct {
	assigns  map[string]*assignInfo
	children map[string][]string
	nodes    map[string]*TreeNode
}

func loadGraph(tx treeReader, channel string, now int64) (*treeGraph, error) {
	assigns, err := loadAssigns(tx, channel, now)
	if err != nil {
		return nil, err
	}
	links, children, err := loadLinks(tx, channel, assigns)
	if err != nil {
		return nil, err
	}
	claims, err := loadClaims(tx, channel)
	if err != nil {
		return nil, err
	}
	g := &treeGraph{assigns: assigns, children: children, nodes: map[string]*TreeNode{}}
	for th, a := range assigns {
		n := &TreeNode{Thread: th, Parent: links[th].parent, Depth: links[th].depth, AssignedBy: a.by, AssignID: a.id}
		n.State, n.Owner, n.Epoch = nodeState(claims[th], a.live)
		g.nodes[th] = n
	}
	for th, kids := range children {
		for _, k := range kids {
			if isOpenState(g.nodes[k].State) {
				g.nodes[th].OpenChildren++
			}
		}
	}
	return g, nil
}

// pickRoots chooses the trees to print: the one rooted at root, or the roots
// with children (all roots with all), newest first, at most limit.
func (g *treeGraph) pickRoots(root string, all bool, limit int) ([]string, error) {
	if root != "" {
		if _, ok := g.nodes[root]; !ok {
			return nil, fmt.Errorf("thread %q has no task.assign in the channel", root)
		}
		return []string{root}, nil
	}
	var roots []string
	for th, n := range g.nodes {
		if n.Parent == "" && (all || len(g.children[th]) > 0) {
			roots = append(roots, th)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return g.assigns[roots[i]].seq > g.assigns[roots[j]].seq })
	if len(roots) > limit {
		roots = roots[:limit]
	}
	return roots, nil
}

// isOpenState mirrors selectOpenChildren: only closed work is not open.
func isOpenState(state string) bool {
	return state != StateRevoked && state != StateCompleted && state != StateExpired
}

func nodeState(c *Claim, live bool) (state, owner string, epoch int64) {
	if c == nil {
		if live {
			return StateAssigned, "", 0
		}
		return StateExpired, "", 0
	}
	switch {
	case c.RevokedAt != nil:
		state = StateRevoked
	case c.CompletedAt != nil:
		state = StateCompleted
	case c.ReleasedAt != nil:
		state = StateReleased
	default:
		state = StateClaimed
	}
	return state, c.Owner, c.Epoch
}

// loadAssigns keeps the first task.assign of each thread (who assigned it and
// its ID) and whether any assign is still within its ttl.
func loadAssigns(tx treeReader, channel string, now int64) (map[string]*assignInfo, error) {
	rows, err := tx.Query(`SELECT seq, id, sender, thread, expires_at FROM messages
		WHERE channel = ? AND type = 'task.assign' AND thread IS NOT NULL ORDER BY seq`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*assignInfo{}
	for rows.Next() {
		var seq int64
		var id, by, thread string
		var exp sql.NullInt64
		if err := rows.Scan(&seq, &id, &by, &thread, &exp); err != nil {
			return nil, err
		}
		a := out[thread]
		if a == nil {
			a = &assignInfo{seq: seq, by: by, id: id}
			out[thread] = a
		}
		if !exp.Valid || exp.Int64 > now {
			a.live = true
		}
	}
	return out, rows.Err()
}

type linkInfo struct {
	parent string
	depth  int
}

// loadLinks returns each linked thread's link and the children by parent in
// assignment order.
func loadLinks(tx treeReader, channel string, assigns map[string]*assignInfo) (links map[string]linkInfo, children map[string][]string, err error) {
	rows, err := tx.Query(`SELECT thread, parent, depth FROM task_links WHERE channel = ?`, channel)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	links, children = map[string]linkInfo{}, map[string][]string{}
	for rows.Next() {
		var th string
		var l linkInfo
		if err := rows.Scan(&th, &l.parent, &l.depth); err != nil {
			return nil, nil, err
		}
		if assigns[th] == nil || assigns[l.parent] == nil {
			continue // a link is only ever written next to its assign; skip a torn row
		}
		links[th] = l
		children[l.parent] = append(children[l.parent], th)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	for _, kids := range children {
		sort.Slice(kids, func(i, j int) bool { return assigns[kids[i]].seq < assigns[kids[j]].seq })
	}
	return links, children, nil
}

func loadClaims(tx treeReader, channel string) (map[string]*Claim, error) {
	rows, err := tx.Query(`SELECT `+claimColumns+` FROM claims WHERE channel = ?`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*Claim{}
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		out[c.Thread] = &c
	}
	return out, rows.Err()
}
