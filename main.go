package main

import (
	"fmt"
	"math"

	"github.com/awalterschulze/gographviz"
)

var order = 4

type (
	state interface{}

	node struct {
		probability float64
		children    map[state]*node
	}

	countingNode struct {
		n        int
		children map[state]*countingNode
	}

	tree = node

	countingTree = countingNode
)

func main() {
	tree := &countingTree{
		children: make(map[state]*countingNode),
	}

	tree.learnNgrams(
		[]state{"any", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog", "because", "the", "quick", "orange", "dog", "did", "not", "jump", "over", "any", "animals"},
		8,
	)

	probabilityTreeGraph(tree.makeProbabilityTree())
}

func (t *countingNode) learn(seq []state) {
	if len(seq) == 0 {
		return
	}

	firstState := seq[0]

	nextChild, alreadyExists := t.children[firstState]
	if alreadyExists {
		// Update existing branch
		nextChild.n++
	} else {
		// Make new branch, set nextChild to the new branch
		nextChild = &countingNode{
			n:        1,
			children: make(map[state]*countingNode),
		}
		t.children[firstState] = nextChild
	}

	if len(seq) > 1 {
		nextChild.learn(seq[1:])
	}
}

func (t *countingNode) learnMany(seqs [][]state) {
	for _, seq := range seqs {
		t.learn(seq)
	}
}

func (t *countingNode) learnNgrams(seq []state, n int) {
	buf := make([]state, 0, n)

	for _, s := range seq {
		if len(buf) == n {
			buf = append(buf[1:], s)
		} else {
			buf = append(buf, s)
		}

		t.learn(buf)
	}

	for i := 1; i < n; i++ {
		buf = buf[1:]

		t.learn(buf)
	}
}

func (t *countingNode) makeProbabilityTree() *node {
	totalCount := 0
	for _, c := range t.children {
		totalCount += c.n
	}

	probabilityChildren := make(map[state]*node)

	for state, c := range t.children {
		childTree := c.makeProbabilityTree()
		childTree.probability = float64(c.n) / float64(totalCount)

		probabilityChildren[state] = childTree
	}

	return &node{
		children: probabilityChildren,
	}
}

func (t *countingNode) graphviz(graph *gographviz.Graph, id, name string) {
	graph.AddNode("G", id, map[string]string{
		"label": name,
	})

	for state, node := range t.children {
		childID := fmt.Sprintf("%s_%v", id, state)
		node.graphviz(graph, childID, fmt.Sprintf("%v", state))
		graph.AddEdge(id, childID, true, map[string]string{
			"label": fmt.Sprintf("%v", node.n),
		})
	}
}

func (t *node) graphviz(graph *gographviz.Graph, id, name string) {
	graph.AddNode("G", id, map[string]string{
		"label": name,
	})

	for state, node := range t.children {
		childID := fmt.Sprintf("%s_%v", id, state)
		node.graphviz(graph, childID, fmt.Sprintf("%v", state))
		graph.AddEdge(id, childID, true, map[string]string{
			"label": fmt.Sprintf("%v", math.Floor(node.probability*100)/100),
		})
	}
}

func (t *countingNode) String() string {
	return fmt.Sprintf("(n=%d, children=%v)", t.n, t.children)
}

func (t *node) String() string {
	return fmt.Sprintf("(p=%v, children=%v)", t.probability, t.children)
}

func countingTreeGraph(tree *countingTree) {
	graphAst, _ := gographviz.ParseString("digraph G {}")
	graph := gographviz.NewGraph()
	if err := gographviz.Analyse(graphAst, graph); err != nil {
		fmt.Println("error generating graphviz graph:", err)
	}

	tree.graphviz(graph, "start", "start")

	fmt.Println(graph.String())
}

func probabilityTreeGraph(tree *tree) {
	graphAst, _ := gographviz.ParseString("digraph G {}")
	graph := gographviz.NewGraph()
	if err := gographviz.Analyse(graphAst, graph); err != nil {
		fmt.Println("error generating graphviz graph:", err)
	}

	tree.graphviz(graph, "start", "start")

	fmt.Println(graph.String())
}
