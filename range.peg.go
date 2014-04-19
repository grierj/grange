package grange

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const end_symbol rune = 4

/* The rule types inferred from the grammar are below. */
type Rule uint8

const (
	RuleUnknown Rule = iota
	Ruleexpression
	Rulerangeexpr
	Rulecombinators
	Ruleintersect
	Ruleexclude
	Ruleunion
	Rulebraces
	Rulegroupq
	Rulecluster
	Rulegroup
	Rulekey
	Rulelocalkey
	Rulefunction
	Rulefuncargs
	Ruleregex
	Ruleliteral
	Rulevalue
	Rulespace
	Ruleq
	RuleAction0
	Rulenull
	RuleAction1
	RuleAction2
	RuleAction3
	RuleAction4
	RuleAction5
	RuleAction6
	RuleAction7
	RuleAction8
	RuleAction9
	RuleAction10
	RuleAction11
	RuleAction12
	RulePegText
	RuleAction13
	RuleAction14
	RuleAction15

	RulePre_
	Rule_In_
	Rule_Suf
)

var Rul3s = [...]string{
	"Unknown",
	"expression",
	"rangeexpr",
	"combinators",
	"intersect",
	"exclude",
	"union",
	"braces",
	"groupq",
	"cluster",
	"group",
	"key",
	"localkey",
	"function",
	"funcargs",
	"regex",
	"literal",
	"value",
	"space",
	"q",
	"Action0",
	"null",
	"Action1",
	"Action2",
	"Action3",
	"Action4",
	"Action5",
	"Action6",
	"Action7",
	"Action8",
	"Action9",
	"Action10",
	"Action11",
	"Action12",
	"PegText",
	"Action13",
	"Action14",
	"Action15",

	"Pre_",
	"_In_",
	"_Suf",
}

type tokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule Rule, begin, end, next, depth int)
	Expand(index int) tokenTree
	Tokens() <-chan token32
	AST() *Node32
	Error() []token32
	trim(length int)
}

type Node32 struct {
	token32
	up, next *Node32
}

func (node *Node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", Rul3s[node.Rule], strconv.Quote(buffer[node.begin:node.end]))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (ast *Node32) Print(buffer string) {
	ast.print(0, buffer)
}

type element struct {
	node *Node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token16 struct {
	Rule
	begin, end, next int16
}

func (t *token16) isZero() bool {
	return t.Rule == RuleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token16) isParentOf(u token16) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token16) GetToken32() token32 {
	return token32{Rule: t.Rule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token16) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", Rul3s[t.Rule], t.begin, t.end, t.next)
}

type tokens16 struct {
	tree    []token16
	ordered [][]token16
}

func (t *tokens16) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens16) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens16) Order() [][]token16 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int16, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.Rule == RuleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token16, len(depths)), make([]token16, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int16(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type State16 struct {
	token16
	depths []int16
	leaf   bool
}

func (t *tokens16) AST() *Node32 {
	tokens := t.Tokens()
	stack := &element{node: &Node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &Node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens16) PreOrder() (<-chan State16, [][]token16) {
	s, ordered := make(chan State16, 6), t.Order()
	go func() {
		var states [8]State16
		for i, _ := range states {
			states[i].depths = make([]int16, len(ordered))
		}
		depths, state, depth := make([]int16, len(ordered)), 0, 1
		write := func(t token16, leaf bool) {
			S := states[state]
			state, S.Rule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.Rule, t.begin, t.end, int16(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token16 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token16{Rule: Rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token16{Rule: RulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.Rule != RuleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.Rule != RuleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token16{Rule: Rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens16) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", Rul3s[token.Rule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", Rul3s[token.Rule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", Rul3s[token.Rule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens16) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", Rul3s[token.Rule], strconv.Quote(buffer[token.begin:token.end]))
	}
}

func (t *tokens16) Add(rule Rule, begin, end, depth, index int) {
	t.tree[index] = token16{Rule: rule, begin: int16(begin), end: int16(end), next: int16(depth)}
}

func (t *tokens16) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.GetToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens16) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].GetToken32()
		}
	}
	return tokens
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	Rule
	begin, end, next int32
}

func (t *token32) isZero() bool {
	return t.Rule == RuleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) GetToken32() token32 {
	return token32{Rule: t.Rule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", Rul3s[t.Rule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.Rule == RuleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type State32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *Node32 {
	tokens := t.Tokens()
	stack := &element{node: &Node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &Node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan State32, [][]token32) {
	s, ordered := make(chan State32, 6), t.Order()
	go func() {
		var states [8]State32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.Rule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.Rule, t.begin, t.end, int32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{Rule: Rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{Rule: RulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.Rule != RuleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.Rule != RuleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{Rule: Rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", Rul3s[token.Rule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", Rul3s[token.Rule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", Rul3s[token.Rule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", Rul3s[token.Rule], strconv.Quote(buffer[token.begin:token.end]))
	}
}

func (t *tokens32) Add(rule Rule, begin, end, depth, index int) {
	t.tree[index] = token32{Rule: rule, begin: int32(begin), end: int32(end), next: int32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.GetToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].GetToken32()
		}
	}
	return tokens
}

func (t *tokens16) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		for i, v := range tree {
			expanded[i] = v.GetToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}

func (t *tokens32) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type rangeQuery struct {
	currentLiteral string
	nodeStack      []parserNode

	Buffer string
	buffer []rune
	rules  [38]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	tokenTree
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer string, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer[0:] {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p *rangeQuery
}

func (e *parseError) Error() string {
	tokens, error := e.p.tokenTree.Error(), "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.Buffer, positions)
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf("parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n",
			Rul3s[token.Rule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			/*strconv.Quote(*/ e.p.Buffer[begin:end] /*)*/)
	}

	return error
}

func (p *rangeQuery) PrintSyntaxTree() {
	p.tokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *rangeQuery) Highlighter() {
	p.tokenTree.PrintSyntax()
}

func (p *rangeQuery) Execute() {
	buffer, begin, end := p.Buffer, 0, 0
	for token := range p.tokenTree.Tokens() {
		switch token.Rule {
		case RulePegText:
			begin, end = int(token.begin), int(token.end)
		case RuleAction0:
			p.addBraceStart()
		case RuleAction1:
			p.addOperator(operatorIntersect)
		case RuleAction2:
			p.addOperator(operatorSubtract)
		case RuleAction3:
			p.addOperator(operatorUnion)
		case RuleAction4:
			p.addBraces()
		case RuleAction5:
			p.addGroupQuery()
		case RuleAction6:
			p.addClusterLookup()
		case RuleAction7:
			p.addGroupLookup()
		case RuleAction8:
			p.addKeyLookup()
		case RuleAction9:
			p.addLocalClusterLookup(buffer[begin:end])
		case RuleAction10:
			p.addFunction(buffer[begin:end])
		case RuleAction11:
			p.addFuncArg()
		case RuleAction12:
			p.addFuncArg()
		case RuleAction13:
			p.addRegex(buffer[begin:end])
		case RuleAction14:
			p.addValue(buffer[begin:end])
		case RuleAction15:
			p.addConstant(buffer[begin:end])

		}
	}
}

func (p *rangeQuery) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != end_symbol {
		p.buffer = append(p.buffer, end_symbol)
	}

	var tree tokenTree = &tokens16{tree: make([]token16, math.MaxInt16)}
	position, depth, tokenIndex, buffer, rules := 0, 0, 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokenTree = tree
		if matches {
			p.tokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule Rule, begin int) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
	}

	matchDot := func() bool {
		if buffer[position] != end_symbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	rules = [...]func() bool{
		nil,
		/* 0 expression <- <(rangeexpr combinators? !.)> */
		func() bool {
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
				if !rules[Rulerangeexpr]() {
					goto l0
				}
				{
					position2, tokenIndex2, depth2 := position, tokenIndex, depth
					if !rules[Rulecombinators]() {
						goto l2
					}
					goto l3
				l2:
					position, tokenIndex, depth = position2, tokenIndex2, depth2
				}
			l3:
				{
					position4, tokenIndex4, depth4 := position, tokenIndex, depth
					if !matchDot() {
						goto l4
					}
					goto l0
				l4:
					position, tokenIndex, depth = position4, tokenIndex4, depth4
				}
				depth--
				add(Ruleexpression, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 rangeexpr <- <(space (q / function / cluster / group / groupq / localkey / regex / value / (Action0 braces) / null))> */
		func() bool {
			position5, tokenIndex5, depth5 := position, tokenIndex, depth
			{
				position6 := position
				depth++
				if !rules[Rulespace]() {
					goto l5
				}
				{
					position7, tokenIndex7, depth7 := position, tokenIndex, depth
					if !rules[Ruleq]() {
						goto l8
					}
					goto l7
				l8:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[Rulefunction]() {
						goto l9
					}
					goto l7
				l9:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[Rulecluster]() {
						goto l10
					}
					goto l7
				l10:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[Rulegroup]() {
						goto l11
					}
					goto l7
				l11:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[Rulegroupq]() {
						goto l12
					}
					goto l7
				l12:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[Rulelocalkey]() {
						goto l13
					}
					goto l7
				l13:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[Ruleregex]() {
						goto l14
					}
					goto l7
				l14:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[Rulevalue]() {
						goto l15
					}
					goto l7
				l15:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[RuleAction0]() {
						goto l16
					}
					if !rules[Rulebraces]() {
						goto l16
					}
					goto l7
				l16:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[Rulenull]() {
						goto l5
					}
				}
			l7:
				depth--
				add(Rulerangeexpr, position6)
			}
			return true
		l5:
			position, tokenIndex, depth = position5, tokenIndex5, depth5
			return false
		},
		/* 2 combinators <- <(space (union / intersect / exclude / braces))> */
		func() bool {
			position17, tokenIndex17, depth17 := position, tokenIndex, depth
			{
				position18 := position
				depth++
				if !rules[Rulespace]() {
					goto l17
				}
				{
					position19, tokenIndex19, depth19 := position, tokenIndex, depth
					if !rules[Ruleunion]() {
						goto l20
					}
					goto l19
				l20:
					position, tokenIndex, depth = position19, tokenIndex19, depth19
					if !rules[Ruleintersect]() {
						goto l21
					}
					goto l19
				l21:
					position, tokenIndex, depth = position19, tokenIndex19, depth19
					if !rules[Ruleexclude]() {
						goto l22
					}
					goto l19
				l22:
					position, tokenIndex, depth = position19, tokenIndex19, depth19
					if !rules[Rulebraces]() {
						goto l17
					}
				}
			l19:
				depth--
				add(Rulecombinators, position18)
			}
			return true
		l17:
			position, tokenIndex, depth = position17, tokenIndex17, depth17
			return false
		},
		/* 3 intersect <- <('&' rangeexpr Action1 combinators?)> */
		func() bool {
			position23, tokenIndex23, depth23 := position, tokenIndex, depth
			{
				position24 := position
				depth++
				if buffer[position] != rune('&') {
					goto l23
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l23
				}
				if !rules[RuleAction1]() {
					goto l23
				}
				{
					position25, tokenIndex25, depth25 := position, tokenIndex, depth
					if !rules[Rulecombinators]() {
						goto l25
					}
					goto l26
				l25:
					position, tokenIndex, depth = position25, tokenIndex25, depth25
				}
			l26:
				depth--
				add(Ruleintersect, position24)
			}
			return true
		l23:
			position, tokenIndex, depth = position23, tokenIndex23, depth23
			return false
		},
		/* 4 exclude <- <('-' rangeexpr Action2 combinators?)> */
		func() bool {
			position27, tokenIndex27, depth27 := position, tokenIndex, depth
			{
				position28 := position
				depth++
				if buffer[position] != rune('-') {
					goto l27
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l27
				}
				if !rules[RuleAction2]() {
					goto l27
				}
				{
					position29, tokenIndex29, depth29 := position, tokenIndex, depth
					if !rules[Rulecombinators]() {
						goto l29
					}
					goto l30
				l29:
					position, tokenIndex, depth = position29, tokenIndex29, depth29
				}
			l30:
				depth--
				add(Ruleexclude, position28)
			}
			return true
		l27:
			position, tokenIndex, depth = position27, tokenIndex27, depth27
			return false
		},
		/* 5 union <- <(',' rangeexpr Action3 combinators?)> */
		func() bool {
			position31, tokenIndex31, depth31 := position, tokenIndex, depth
			{
				position32 := position
				depth++
				if buffer[position] != rune(',') {
					goto l31
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l31
				}
				if !rules[RuleAction3]() {
					goto l31
				}
				{
					position33, tokenIndex33, depth33 := position, tokenIndex, depth
					if !rules[Rulecombinators]() {
						goto l33
					}
					goto l34
				l33:
					position, tokenIndex, depth = position33, tokenIndex33, depth33
				}
			l34:
				depth--
				add(Ruleunion, position32)
			}
			return true
		l31:
			position, tokenIndex, depth = position31, tokenIndex31, depth31
			return false
		},
		/* 6 braces <- <('{' rangeexpr combinators? '}' rangeexpr? Action4)> */
		func() bool {
			position35, tokenIndex35, depth35 := position, tokenIndex, depth
			{
				position36 := position
				depth++
				if buffer[position] != rune('{') {
					goto l35
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l35
				}
				{
					position37, tokenIndex37, depth37 := position, tokenIndex, depth
					if !rules[Rulecombinators]() {
						goto l37
					}
					goto l38
				l37:
					position, tokenIndex, depth = position37, tokenIndex37, depth37
				}
			l38:
				if buffer[position] != rune('}') {
					goto l35
				}
				position++
				{
					position39, tokenIndex39, depth39 := position, tokenIndex, depth
					if !rules[Rulerangeexpr]() {
						goto l39
					}
					goto l40
				l39:
					position, tokenIndex, depth = position39, tokenIndex39, depth39
				}
			l40:
				if !rules[RuleAction4]() {
					goto l35
				}
				depth--
				add(Rulebraces, position36)
			}
			return true
		l35:
			position, tokenIndex, depth = position35, tokenIndex35, depth35
			return false
		},
		/* 7 groupq <- <('?' rangeexpr Action5)> */
		func() bool {
			position41, tokenIndex41, depth41 := position, tokenIndex, depth
			{
				position42 := position
				depth++
				if buffer[position] != rune('?') {
					goto l41
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l41
				}
				if !rules[RuleAction5]() {
					goto l41
				}
				depth--
				add(Rulegroupq, position42)
			}
			return true
		l41:
			position, tokenIndex, depth = position41, tokenIndex41, depth41
			return false
		},
		/* 8 cluster <- <('%' rangeexpr Action6 key?)> */
		func() bool {
			position43, tokenIndex43, depth43 := position, tokenIndex, depth
			{
				position44 := position
				depth++
				if buffer[position] != rune('%') {
					goto l43
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l43
				}
				if !rules[RuleAction6]() {
					goto l43
				}
				{
					position45, tokenIndex45, depth45 := position, tokenIndex, depth
					if !rules[Rulekey]() {
						goto l45
					}
					goto l46
				l45:
					position, tokenIndex, depth = position45, tokenIndex45, depth45
				}
			l46:
				depth--
				add(Rulecluster, position44)
			}
			return true
		l43:
			position, tokenIndex, depth = position43, tokenIndex43, depth43
			return false
		},
		/* 9 group <- <('@' rangeexpr Action7)> */
		func() bool {
			position47, tokenIndex47, depth47 := position, tokenIndex, depth
			{
				position48 := position
				depth++
				if buffer[position] != rune('@') {
					goto l47
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l47
				}
				if !rules[RuleAction7]() {
					goto l47
				}
				depth--
				add(Rulegroup, position48)
			}
			return true
		l47:
			position, tokenIndex, depth = position47, tokenIndex47, depth47
			return false
		},
		/* 10 key <- <(':' rangeexpr Action8)> */
		func() bool {
			position49, tokenIndex49, depth49 := position, tokenIndex, depth
			{
				position50 := position
				depth++
				if buffer[position] != rune(':') {
					goto l49
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l49
				}
				if !rules[RuleAction8]() {
					goto l49
				}
				depth--
				add(Rulekey, position50)
			}
			return true
		l49:
			position, tokenIndex, depth = position49, tokenIndex49, depth49
			return false
		},
		/* 11 localkey <- <('$' literal Action9)> */
		func() bool {
			position51, tokenIndex51, depth51 := position, tokenIndex, depth
			{
				position52 := position
				depth++
				if buffer[position] != rune('$') {
					goto l51
				}
				position++
				if !rules[Ruleliteral]() {
					goto l51
				}
				if !rules[RuleAction9]() {
					goto l51
				}
				depth--
				add(Rulelocalkey, position52)
			}
			return true
		l51:
			position, tokenIndex, depth = position51, tokenIndex51, depth51
			return false
		},
		/* 12 function <- <(literal Action10 '(' funcargs ')')> */
		func() bool {
			position53, tokenIndex53, depth53 := position, tokenIndex, depth
			{
				position54 := position
				depth++
				if !rules[Ruleliteral]() {
					goto l53
				}
				if !rules[RuleAction10]() {
					goto l53
				}
				if buffer[position] != rune('(') {
					goto l53
				}
				position++
				if !rules[Rulefuncargs]() {
					goto l53
				}
				if buffer[position] != rune(')') {
					goto l53
				}
				position++
				depth--
				add(Rulefunction, position54)
			}
			return true
		l53:
			position, tokenIndex, depth = position53, tokenIndex53, depth53
			return false
		},
		/* 13 funcargs <- <((rangeexpr Action11 ';' funcargs) / (rangeexpr Action12))> */
		func() bool {
			position55, tokenIndex55, depth55 := position, tokenIndex, depth
			{
				position56 := position
				depth++
				{
					position57, tokenIndex57, depth57 := position, tokenIndex, depth
					if !rules[Rulerangeexpr]() {
						goto l58
					}
					if !rules[RuleAction11]() {
						goto l58
					}
					if buffer[position] != rune(';') {
						goto l58
					}
					position++
					if !rules[Rulefuncargs]() {
						goto l58
					}
					goto l57
				l58:
					position, tokenIndex, depth = position57, tokenIndex57, depth57
					if !rules[Rulerangeexpr]() {
						goto l55
					}
					if !rules[RuleAction12]() {
						goto l55
					}
				}
			l57:
				depth--
				add(Rulefuncargs, position56)
			}
			return true
		l55:
			position, tokenIndex, depth = position55, tokenIndex55, depth55
			return false
		},
		/* 14 regex <- <('/' <(!'/' .)*> '/' Action13)> */
		func() bool {
			position59, tokenIndex59, depth59 := position, tokenIndex, depth
			{
				position60 := position
				depth++
				if buffer[position] != rune('/') {
					goto l59
				}
				position++
				{
					position61 := position
					depth++
				l62:
					{
						position63, tokenIndex63, depth63 := position, tokenIndex, depth
						{
							position64, tokenIndex64, depth64 := position, tokenIndex, depth
							if buffer[position] != rune('/') {
								goto l64
							}
							position++
							goto l63
						l64:
							position, tokenIndex, depth = position64, tokenIndex64, depth64
						}
						if !matchDot() {
							goto l63
						}
						goto l62
					l63:
						position, tokenIndex, depth = position63, tokenIndex63, depth63
					}
					depth--
					add(RulePegText, position61)
				}
				if buffer[position] != rune('/') {
					goto l59
				}
				position++
				if !rules[RuleAction13]() {
					goto l59
				}
				depth--
				add(Ruleregex, position60)
			}
			return true
		l59:
			position, tokenIndex, depth = position59, tokenIndex59, depth59
			return false
		},
		/* 15 literal <- <<([a-z] / [A-Z] / ([0-9] / [0-9]) / '-' / '_')+>> */
		func() bool {
			position65, tokenIndex65, depth65 := position, tokenIndex, depth
			{
				position66 := position
				depth++
				{
					position67 := position
					depth++
					{
						position70, tokenIndex70, depth70 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l71
						}
						position++
						goto l70
					l71:
						position, tokenIndex, depth = position70, tokenIndex70, depth70
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l72
						}
						position++
						goto l70
					l72:
						position, tokenIndex, depth = position70, tokenIndex70, depth70
						{
							position74, tokenIndex74, depth74 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l75
							}
							position++
							goto l74
						l75:
							position, tokenIndex, depth = position74, tokenIndex74, depth74
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l73
							}
							position++
						}
					l74:
						goto l70
					l73:
						position, tokenIndex, depth = position70, tokenIndex70, depth70
						if buffer[position] != rune('-') {
							goto l76
						}
						position++
						goto l70
					l76:
						position, tokenIndex, depth = position70, tokenIndex70, depth70
						if buffer[position] != rune('_') {
							goto l65
						}
						position++
					}
				l70:
				l68:
					{
						position69, tokenIndex69, depth69 := position, tokenIndex, depth
						{
							position77, tokenIndex77, depth77 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l78
							}
							position++
							goto l77
						l78:
							position, tokenIndex, depth = position77, tokenIndex77, depth77
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l79
							}
							position++
							goto l77
						l79:
							position, tokenIndex, depth = position77, tokenIndex77, depth77
							{
								position81, tokenIndex81, depth81 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l82
								}
								position++
								goto l81
							l82:
								position, tokenIndex, depth = position81, tokenIndex81, depth81
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l80
								}
								position++
							}
						l81:
							goto l77
						l80:
							position, tokenIndex, depth = position77, tokenIndex77, depth77
							if buffer[position] != rune('-') {
								goto l83
							}
							position++
							goto l77
						l83:
							position, tokenIndex, depth = position77, tokenIndex77, depth77
							if buffer[position] != rune('_') {
								goto l69
							}
							position++
						}
					l77:
						goto l68
					l69:
						position, tokenIndex, depth = position69, tokenIndex69, depth69
					}
					depth--
					add(RulePegText, position67)
				}
				depth--
				add(Ruleliteral, position66)
			}
			return true
		l65:
			position, tokenIndex, depth = position65, tokenIndex65, depth65
			return false
		},
		/* 16 value <- <(<([a-z] / [A-Z] / ([0-9] / [0-9]) / '-' / '_' / '.')+> Action14)> */
		func() bool {
			position84, tokenIndex84, depth84 := position, tokenIndex, depth
			{
				position85 := position
				depth++
				{
					position86 := position
					depth++
					{
						position89, tokenIndex89, depth89 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l90
						}
						position++
						goto l89
					l90:
						position, tokenIndex, depth = position89, tokenIndex89, depth89
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l91
						}
						position++
						goto l89
					l91:
						position, tokenIndex, depth = position89, tokenIndex89, depth89
						{
							position93, tokenIndex93, depth93 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l94
							}
							position++
							goto l93
						l94:
							position, tokenIndex, depth = position93, tokenIndex93, depth93
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l92
							}
							position++
						}
					l93:
						goto l89
					l92:
						position, tokenIndex, depth = position89, tokenIndex89, depth89
						if buffer[position] != rune('-') {
							goto l95
						}
						position++
						goto l89
					l95:
						position, tokenIndex, depth = position89, tokenIndex89, depth89
						if buffer[position] != rune('_') {
							goto l96
						}
						position++
						goto l89
					l96:
						position, tokenIndex, depth = position89, tokenIndex89, depth89
						if buffer[position] != rune('.') {
							goto l84
						}
						position++
					}
				l89:
				l87:
					{
						position88, tokenIndex88, depth88 := position, tokenIndex, depth
						{
							position97, tokenIndex97, depth97 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l98
							}
							position++
							goto l97
						l98:
							position, tokenIndex, depth = position97, tokenIndex97, depth97
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l99
							}
							position++
							goto l97
						l99:
							position, tokenIndex, depth = position97, tokenIndex97, depth97
							{
								position101, tokenIndex101, depth101 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l102
								}
								position++
								goto l101
							l102:
								position, tokenIndex, depth = position101, tokenIndex101, depth101
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l100
								}
								position++
							}
						l101:
							goto l97
						l100:
							position, tokenIndex, depth = position97, tokenIndex97, depth97
							if buffer[position] != rune('-') {
								goto l103
							}
							position++
							goto l97
						l103:
							position, tokenIndex, depth = position97, tokenIndex97, depth97
							if buffer[position] != rune('_') {
								goto l104
							}
							position++
							goto l97
						l104:
							position, tokenIndex, depth = position97, tokenIndex97, depth97
							if buffer[position] != rune('.') {
								goto l88
							}
							position++
						}
					l97:
						goto l87
					l88:
						position, tokenIndex, depth = position88, tokenIndex88, depth88
					}
					depth--
					add(RulePegText, position86)
				}
				if !rules[RuleAction14]() {
					goto l84
				}
				depth--
				add(Rulevalue, position85)
			}
			return true
		l84:
			position, tokenIndex, depth = position84, tokenIndex84, depth84
			return false
		},
		/* 17 space <- <' '*> */
		func() bool {
			{
				position106 := position
				depth++
			l107:
				{
					position108, tokenIndex108, depth108 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l108
					}
					position++
					goto l107
				l108:
					position, tokenIndex, depth = position108, tokenIndex108, depth108
				}
				depth--
				add(Rulespace, position106)
			}
			return true
		},
		/* 18 q <- <('q' '(' <(!')' .)*> ')' Action15)> */
		func() bool {
			position109, tokenIndex109, depth109 := position, tokenIndex, depth
			{
				position110 := position
				depth++
				if buffer[position] != rune('q') {
					goto l109
				}
				position++
				if buffer[position] != rune('(') {
					goto l109
				}
				position++
				{
					position111 := position
					depth++
				l112:
					{
						position113, tokenIndex113, depth113 := position, tokenIndex, depth
						{
							position114, tokenIndex114, depth114 := position, tokenIndex, depth
							if buffer[position] != rune(')') {
								goto l114
							}
							position++
							goto l113
						l114:
							position, tokenIndex, depth = position114, tokenIndex114, depth114
						}
						if !matchDot() {
							goto l113
						}
						goto l112
					l113:
						position, tokenIndex, depth = position113, tokenIndex113, depth113
					}
					depth--
					add(RulePegText, position111)
				}
				if buffer[position] != rune(')') {
					goto l109
				}
				position++
				if !rules[RuleAction15]() {
					goto l109
				}
				depth--
				add(Ruleq, position110)
			}
			return true
		l109:
			position, tokenIndex, depth = position109, tokenIndex109, depth109
			return false
		},
		/* 20 Action0 <- <{ p.addBraceStart() }> */
		func() bool {
			{
				add(RuleAction0, position)
			}
			return true
		},
		/* 21 null <- <> */
		func() bool {
			{
				position118 := position
				depth++
				depth--
				add(Rulenull, position118)
			}
			return true
		},
		/* 22 Action1 <- <{ p.addOperator(operatorIntersect) }> */
		func() bool {
			{
				add(RuleAction1, position)
			}
			return true
		},
		/* 23 Action2 <- <{ p.addOperator(operatorSubtract) }> */
		func() bool {
			{
				add(RuleAction2, position)
			}
			return true
		},
		/* 24 Action3 <- <{ p.addOperator(operatorUnion) }> */
		func() bool {
			{
				add(RuleAction3, position)
			}
			return true
		},
		/* 25 Action4 <- <{ p.addBraces() }> */
		func() bool {
			{
				add(RuleAction4, position)
			}
			return true
		},
		/* 26 Action5 <- <{ p.addGroupQuery() }> */
		func() bool {
			{
				add(RuleAction5, position)
			}
			return true
		},
		/* 27 Action6 <- <{ p.addClusterLookup() }> */
		func() bool {
			{
				add(RuleAction6, position)
			}
			return true
		},
		/* 28 Action7 <- <{ p.addGroupLookup() }> */
		func() bool {
			{
				add(RuleAction7, position)
			}
			return true
		},
		/* 29 Action8 <- <{ p.addKeyLookup() }> */
		func() bool {
			{
				add(RuleAction8, position)
			}
			return true
		},
		/* 30 Action9 <- <{ p.addLocalClusterLookup(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction9, position)
			}
			return true
		},
		/* 31 Action10 <- <{ p.addFunction(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction10, position)
			}
			return true
		},
		/* 32 Action11 <- <{ p.addFuncArg() }> */
		func() bool {
			{
				add(RuleAction11, position)
			}
			return true
		},
		/* 33 Action12 <- <{ p.addFuncArg() }> */
		func() bool {
			{
				add(RuleAction12, position)
			}
			return true
		},
		nil,
		/* 35 Action13 <- <{ p.addRegex(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction13, position)
			}
			return true
		},
		/* 36 Action14 <- <{ p.addValue(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction14, position)
			}
			return true
		},
		/* 37 Action15 <- <{ p.addConstant(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction15, position)
			}
			return true
		},
	}
	p.rules = rules
}
