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

	var capacity, q int
	if _, err := fmt.Fscan(in, &capacity, &q); err != nil {
		return
	}
	cache := ConstructorReference(capacity)
	for i := 0; i < q; i++ {
		var op string
		fmt.Fscan(in, &op)
		if op == "put" {
			var key, value int
			fmt.Fscan(in, &key, &value)
			cache.Put(key, value)
		} else if op == "get" {
			var key int
			fmt.Fscan(in, &key)
			fmt.Fprintln(out, cache.Get(key))
		}
	}
}

type entryNode struct {
	key, value  int
	prev, next *entryNode
}

type LRUCacheReference struct {
	capacity int
	nodes    map[int]*entryNode
	head     *entryNode
	tail     *entryNode
}

func ConstructorReference(capacity int) LRUCacheReference {
	head := &entryNode{}
	tail := &entryNode{}
	head.next = tail
	tail.prev = head
	return LRUCacheReference{
		capacity: capacity,
		nodes:    make(map[int]*entryNode),
		head:     head,
		tail:     tail,
	}
}

func (c *LRUCacheReference) Get(key int) int {
	node, ok := c.nodes[key]
	if !ok {
		return -1
	}
	c.moveToFrontReference(node)
	return node.value
}

func (c *LRUCacheReference) Put(key int, value int) {
	if node, ok := c.nodes[key]; ok {
		node.value = value
		c.moveToFrontReference(node)
		return
	}
	node := &entryNode{key: key, value: value}
	c.nodes[key] = node
	c.addAfterHeadReference(node)
	if len(c.nodes) > c.capacity {
		removed := c.tail.prev
		c.removeReference(removed)
		delete(c.nodes, removed.key)
	}
}

func (c *LRUCacheReference) moveToFrontReference(node *entryNode) {
	c.removeReference(node)
	c.addAfterHeadReference(node)
}

func (c *LRUCacheReference) addAfterHeadReference(node *entryNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

func (c *LRUCacheReference) removeReference(node *entryNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}
