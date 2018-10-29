package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"strings"

	"github.com/awalterschulze/gographviz"
)

var (
	order    = flag.Int("order", 5, "the 'look-behind memory' of the Markov chain")
	filename = flag.String("file", "in.txt", "the file to create a Markov chain from")
	kind     = flag.String("kind", "word", "the size of a single token/entity. allowed values: word, character, line")
)

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

	idGen struct {
		next int
	}

	tree = node

	countingTree = countingNode
)

func main() {
	flag.Parse()

	bytes, err := ioutil.ReadFile(*filename)
	if err != nil {
		log.Fatalf("could not open file %s: %v", *filename, err)
	}

	var (
		input        = string(bytes)
		stringTokens = make([]string, 0, 64)
	)

	switch *kind {
	case "word":
		stringTokens = strings.Fields(input)

	case "character":
		stringTokens = strings.Split(input, "")

	case "line":
		stringTokens = strings.Split(input, "\n")

	default:
		log.Fatalf("argument -kind should be 'word', 'character', or 'line'")
	}

	states := make([]state, len(stringTokens))

	for i, tok := range stringTokens {
		states[i] = state(tok)
	}

	tree := &countingTree{
		children: make(map[state]*countingNode),
	}

	tree.learnNgrams(states, *order)

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

func (t *node) graphviz(graph *gographviz.Graph, idGen *idGen, name string) {
	curID := idGen.gen()

	graph.AddNode("G", curID, map[string]string{
		"label": fmt.Sprintf(`"%s"`, strings.Replace(name, `"`, `\"`, -1)),
	})

	for state, node := range t.children {
		childID := idGen.peek()

		node.graphviz(graph, idGen, fmt.Sprintf("%v", state))
		graph.AddEdge(curID, childID, true, map[string]string{
			"label": fmt.Sprintf("%v", math.Floor(node.probability*100)/100),
		})
	}
}

func (i *idGen) gen() string {
	val := fmt.Sprintf("%d", i.next)
	i.next++
	return val
}

func (i *idGen) peek() string {
	return fmt.Sprintf("%d", i.next)
}

func (t *countingNode) String() string {
	return fmt.Sprintf("(n=%d, children=%v)", t.n, t.children)
}

func (t *node) String() string {
	return fmt.Sprintf("(p=%v, children=%v)", t.probability, t.children)
}

func probabilityTreeGraph(tree *tree) {
	graphAst, _ := gographviz.ParseString("digraph G {}")
	graph := gographviz.NewGraph()
	if err := gographviz.Analyse(graphAst, graph); err != nil {
		fmt.Println("error generating graphviz graph:", err)
	}

	tree.graphviz(graph, new(idGen), "start")

	fmt.Println(graph.String())
}
