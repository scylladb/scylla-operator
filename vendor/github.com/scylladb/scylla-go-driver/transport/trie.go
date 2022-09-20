package transport

import (
	"github.com/scylladb/scylla-go-driver/frame"
)

type trie struct {
	next map[frame.UUID]*trie
	path []*Node
}

func trieRoot() trie {
	return trie{
		next: make(map[frame.UUID]*trie),
	}
}

func newTrie(node *Node, parent *trie) *trie {
	return &trie{
		next: make(map[frame.UUID]*trie),
		path: append(parent.path, node),
	}
}

func (t *trie) Next(node *Node) *trie {
	n, ok := t.next[node.hostID]
	if ok {
		return n
	}

	n = newTrie(node, t)
	t.next[node.hostID] = n
	return n
}

func (t *trie) Path() []*Node {
	return t.path
}
