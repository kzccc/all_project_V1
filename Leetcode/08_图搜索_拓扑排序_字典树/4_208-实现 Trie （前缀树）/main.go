package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	in := bufio.NewReader(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	var q int
	if _, err := fmt.Fscan(in, &q); err != nil {
		return
	}
	trie := ConstructorReference()
	for i := 0; i < q; i++ {
		var op, word string
		fmt.Fscan(in, &op, &word)
		if op == "insert" {
			trie.Insert(word)
		} else if op == "search" {
			fmt.Fprintln(out, trie.Search(word))
		} else if op == "startsWith" {
			fmt.Fprintln(out, trie.StartsWith(word))
		}
	}
}

type TrieReference struct {
	children [26]*TrieReference
	isWord   bool
}

func ConstructorReference() TrieReference {
	return TrieReference{}
}

func (t *TrieReference) Insert(word string) {
	node := t
	for i := 0; i < len(word); i++ {
		idx := word[i] - 'a'
		if node.children[idx] == nil {
			node.children[idx] = &TrieReference{}
		}
		node = node.children[idx]
	}
	node.isWord = true
}

func (t *TrieReference) Search(word string) bool {
	node := t.findPrefixReference(word)
	return node != nil && node.isWord
}

func (t *TrieReference) StartsWith(prefix string) bool {
	return t.findPrefixReference(prefix) != nil
}

func (t *TrieReference) findPrefixReference(prefix string) *TrieReference {
	node := t
	for i := 0; i < len(prefix); i++ {
		idx := prefix[i] - 'a'
		if node.children[idx] == nil {
			return nil
		}
		node = node.children[idx]
	}
	return node
}
