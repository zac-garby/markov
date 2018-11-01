package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/awalterschulze/gographviz"
)

var (
	order    = flag.Int("order", 5, "the 'look-behind memory' of the Markov chain")
	filename = flag.String("file", "in.txt", "the file to create a Markov chain from")
	kind     = flag.String("kind", "word", "the size of a single token/entity. allowed values: word, character, line")
	seed     = flag.String("seed", "", "the text to seed the generator with. tokens separated by whitespace")
	amount   = flag.Int("amount", 8, "the amount of output to generate (e.g. -amount 8 will output 8 words if -kind=word)")
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

	countingTree := &countingTree{
		children: make(map[state]*countingNode),
	}

	fmt.Println("training...")
	countingTree.learnNgrams(states, *order)
	fmt.Println("done")

	tree := countingTree.makeProbabilityTree()

	var (
		previous   = make([]state, 0)
		seedTokens = make([]string, 0)
	)

	switch *kind {
	case "word":
		seedTokens = strings.Fields(*seed)

	case "character":
		seedTokens = strings.Split(input, "")

	case "line":
		seedTokens = strings.Split(input, "\n")

	default:
		log.Fatalf("argument -kind should be 'word', 'character', or 'line'")
	}

	if len(seedTokens) == 0 {
		log.Fatalf("argument -seed should be non-empty")
	}

	for _, tok := range seedTokens {
		previous = append(previous, state(tok))
	}

	fmt.Print(*seed)
	switch *kind {
	case "word":
		fmt.Print(" ")
	case "line":
		fmt.Print("\n")
	}

	for i := 0; i < *amount; i++ {
		next := tree.predict(previous)

		fmt.Print(next)
		switch *kind {
		case "word":
			fmt.Print(" ")
		case "line":
			fmt.Print("\n")
		}

		previous = append(previous[1:], next)
	}
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

type stateProbabilityPair struct {
	state       state
	probability float64
}

type byProbability []stateProbabilityPair

func (b byProbability) Len() int           { return len(b) }
func (b byProbability) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byProbability) Less(i, j int) bool { return b[i].probability < b[j].probability }

func (t *tree) predict(previous []state) state {
	for {
		prediction, ok := t.predictExact(previous)
		if ok {
			return prediction
		}

		if len(previous) > 1 {
			previous = previous[1:]
		} else {
			return nil
		}
	}
}

func (t *tree) predictExact(previous []state) (prediction state, ok bool) {
	if len(previous) == 0 {
		// Make a choice
		return randomChoice(probPairs(t.children)), true
	}

	child, ok := t.children[previous[0]]
	if !ok {
		return nil, false
	}

	if len(previous) > 1 {
		return child.predictExact(previous[1:])
	}

	return child.predictExact(make([]state, 0))
}

func probPairs(nodes map[state]*node) []stateProbabilityPair {
	choices := make([]stateProbabilityPair, 0, len(nodes))

	for state, child := range nodes {
		choices = append(choices, stateProbabilityPair{
			state:       state,
			probability: child.probability,
		})
	}

	return choices
}

func randomChoice(choices []stateProbabilityPair) state {
	rand := rand.Float64()

	sort.Sort(byProbability(choices))

	for _, pair := range choices {
		if rand < pair.probability {
			return pair.state
		}

		rand -= pair.probability
	}

	return nil
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
