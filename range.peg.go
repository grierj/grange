package grange

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const END_SYMBOL rune = 4

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

type TokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule Rule, begin, end, next, depth int)
	Expand(index int) TokenTree
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

func (t *tokens16) Expand(index int) TokenTree {
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

func (t *tokens32) Expand(index int) TokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type RangeQuery struct {
	currentLiteral string
	nodeStack      []Node

	Buffer string
	buffer []rune
	rules  [38]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	TokenTree
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
	p *RangeQuery
}

func (e *parseError) Error() string {
	tokens, error := e.p.TokenTree.Error(), "\n"
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

func (p *RangeQuery) PrintSyntaxTree() {
	p.TokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *RangeQuery) Highlighter() {
	p.TokenTree.PrintSyntax()
}

func (p *RangeQuery) Execute() {
	buffer, begin, end := p.Buffer, 0, 0
	for token := range p.TokenTree.Tokens() {
		switch token.Rule {
		case RulePegText:
			begin, end = int(token.begin), int(token.end)
		case RuleAction0:
			p.AddNull()
		case RuleAction1:
			p.AddOperator(operatorIntersect)
		case RuleAction2:
			p.AddOperator(operatorSubtract)
		case RuleAction3:
			p.AddOperator(operatorUnion)
		case RuleAction4:
			p.AddBraces()
		case RuleAction5:
			p.AddGroupQuery()
		case RuleAction6:
			p.AddClusterLookup()
		case RuleAction7:
			p.AddGroupLookup()
		case RuleAction8:
			p.AddKeyLookup(buffer[begin:end])
		case RuleAction9:
			p.AddLocalClusterLookup(buffer[begin:end])
		case RuleAction10:
			p.AddFunction(buffer[begin:end])
		case RuleAction11:
			p.AddFuncArg()
		case RuleAction12:
			p.AddFuncArg()
		case RuleAction13:
			p.AddRegex(buffer[begin:end])
		case RuleAction14:
			p.AddValue(buffer[begin:end])
		case RuleAction15:
			p.AddConstant(buffer[begin:end])

		}
	}
}

func (p *RangeQuery) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != END_SYMBOL {
		p.buffer = append(p.buffer, END_SYMBOL)
	}

	var tree TokenTree = &tokens16{tree: make([]token16, math.MaxInt16)}
	position, depth, tokenIndex, buffer, rules := 0, 0, 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.TokenTree = tree
		if matches {
			p.TokenTree.trim(tokenIndex)
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
		if buffer[position] != END_SYMBOL {
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
		/* 3 intersect <- <('&' rangeexpr Action1)> */
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
				depth--
				add(Ruleintersect, position24)
			}
			return true
		l23:
			position, tokenIndex, depth = position23, tokenIndex23, depth23
			return false
		},
		/* 4 exclude <- <('-' rangeexpr Action2)> */
		func() bool {
			position25, tokenIndex25, depth25 := position, tokenIndex, depth
			{
				position26 := position
				depth++
				if buffer[position] != rune('-') {
					goto l25
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l25
				}
				if !rules[RuleAction2]() {
					goto l25
				}
				depth--
				add(Ruleexclude, position26)
			}
			return true
		l25:
			position, tokenIndex, depth = position25, tokenIndex25, depth25
			return false
		},
		/* 5 union <- <(',' rangeexpr Action3)> */
		func() bool {
			position27, tokenIndex27, depth27 := position, tokenIndex, depth
			{
				position28 := position
				depth++
				if buffer[position] != rune(',') {
					goto l27
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l27
				}
				if !rules[RuleAction3]() {
					goto l27
				}
				depth--
				add(Ruleunion, position28)
			}
			return true
		l27:
			position, tokenIndex, depth = position27, tokenIndex27, depth27
			return false
		},
		/* 6 braces <- <('{' rangeexpr combinators? '}' rangeexpr? Action4)> */
		func() bool {
			position29, tokenIndex29, depth29 := position, tokenIndex, depth
			{
				position30 := position
				depth++
				if buffer[position] != rune('{') {
					goto l29
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l29
				}
				{
					position31, tokenIndex31, depth31 := position, tokenIndex, depth
					if !rules[Rulecombinators]() {
						goto l31
					}
					goto l32
				l31:
					position, tokenIndex, depth = position31, tokenIndex31, depth31
				}
			l32:
				if buffer[position] != rune('}') {
					goto l29
				}
				position++
				{
					position33, tokenIndex33, depth33 := position, tokenIndex, depth
					if !rules[Rulerangeexpr]() {
						goto l33
					}
					goto l34
				l33:
					position, tokenIndex, depth = position33, tokenIndex33, depth33
				}
			l34:
				if !rules[RuleAction4]() {
					goto l29
				}
				depth--
				add(Rulebraces, position30)
			}
			return true
		l29:
			position, tokenIndex, depth = position29, tokenIndex29, depth29
			return false
		},
		/* 7 groupq <- <('?' rangeexpr Action5)> */
		func() bool {
			position35, tokenIndex35, depth35 := position, tokenIndex, depth
			{
				position36 := position
				depth++
				if buffer[position] != rune('?') {
					goto l35
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l35
				}
				if !rules[RuleAction5]() {
					goto l35
				}
				depth--
				add(Rulegroupq, position36)
			}
			return true
		l35:
			position, tokenIndex, depth = position35, tokenIndex35, depth35
			return false
		},
		/* 8 cluster <- <('%' rangeexpr Action6 key?)> */
		func() bool {
			position37, tokenIndex37, depth37 := position, tokenIndex, depth
			{
				position38 := position
				depth++
				if buffer[position] != rune('%') {
					goto l37
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l37
				}
				if !rules[RuleAction6]() {
					goto l37
				}
				{
					position39, tokenIndex39, depth39 := position, tokenIndex, depth
					if !rules[Rulekey]() {
						goto l39
					}
					goto l40
				l39:
					position, tokenIndex, depth = position39, tokenIndex39, depth39
				}
			l40:
				depth--
				add(Rulecluster, position38)
			}
			return true
		l37:
			position, tokenIndex, depth = position37, tokenIndex37, depth37
			return false
		},
		/* 9 group <- <('@' rangeexpr Action7)> */
		func() bool {
			position41, tokenIndex41, depth41 := position, tokenIndex, depth
			{
				position42 := position
				depth++
				if buffer[position] != rune('@') {
					goto l41
				}
				position++
				if !rules[Rulerangeexpr]() {
					goto l41
				}
				if !rules[RuleAction7]() {
					goto l41
				}
				depth--
				add(Rulegroup, position42)
			}
			return true
		l41:
			position, tokenIndex, depth = position41, tokenIndex41, depth41
			return false
		},
		/* 10 key <- <(':' literal Action8)> */
		func() bool {
			position43, tokenIndex43, depth43 := position, tokenIndex, depth
			{
				position44 := position
				depth++
				if buffer[position] != rune(':') {
					goto l43
				}
				position++
				if !rules[Ruleliteral]() {
					goto l43
				}
				if !rules[RuleAction8]() {
					goto l43
				}
				depth--
				add(Rulekey, position44)
			}
			return true
		l43:
			position, tokenIndex, depth = position43, tokenIndex43, depth43
			return false
		},
		/* 11 localkey <- <('$' literal Action9)> */
		func() bool {
			position45, tokenIndex45, depth45 := position, tokenIndex, depth
			{
				position46 := position
				depth++
				if buffer[position] != rune('$') {
					goto l45
				}
				position++
				if !rules[Ruleliteral]() {
					goto l45
				}
				if !rules[RuleAction9]() {
					goto l45
				}
				depth--
				add(Rulelocalkey, position46)
			}
			return true
		l45:
			position, tokenIndex, depth = position45, tokenIndex45, depth45
			return false
		},
		/* 12 function <- <(literal Action10 '(' funcargs ')')> */
		func() bool {
			position47, tokenIndex47, depth47 := position, tokenIndex, depth
			{
				position48 := position
				depth++
				if !rules[Ruleliteral]() {
					goto l47
				}
				if !rules[RuleAction10]() {
					goto l47
				}
				if buffer[position] != rune('(') {
					goto l47
				}
				position++
				if !rules[Rulefuncargs]() {
					goto l47
				}
				if buffer[position] != rune(')') {
					goto l47
				}
				position++
				depth--
				add(Rulefunction, position48)
			}
			return true
		l47:
			position, tokenIndex, depth = position47, tokenIndex47, depth47
			return false
		},
		/* 13 funcargs <- <((rangeexpr Action11 ';' funcargs) / (rangeexpr Action12))> */
		func() bool {
			position49, tokenIndex49, depth49 := position, tokenIndex, depth
			{
				position50 := position
				depth++
				{
					position51, tokenIndex51, depth51 := position, tokenIndex, depth
					if !rules[Rulerangeexpr]() {
						goto l52
					}
					if !rules[RuleAction11]() {
						goto l52
					}
					if buffer[position] != rune(';') {
						goto l52
					}
					position++
					if !rules[Rulefuncargs]() {
						goto l52
					}
					goto l51
				l52:
					position, tokenIndex, depth = position51, tokenIndex51, depth51
					if !rules[Rulerangeexpr]() {
						goto l49
					}
					if !rules[RuleAction12]() {
						goto l49
					}
				}
			l51:
				depth--
				add(Rulefuncargs, position50)
			}
			return true
		l49:
			position, tokenIndex, depth = position49, tokenIndex49, depth49
			return false
		},
		/* 14 regex <- <('/' <(!'/' .)*> '/' Action13)> */
		func() bool {
			position53, tokenIndex53, depth53 := position, tokenIndex, depth
			{
				position54 := position
				depth++
				if buffer[position] != rune('/') {
					goto l53
				}
				position++
				{
					position55 := position
					depth++
				l56:
					{
						position57, tokenIndex57, depth57 := position, tokenIndex, depth
						{
							position58, tokenIndex58, depth58 := position, tokenIndex, depth
							if buffer[position] != rune('/') {
								goto l58
							}
							position++
							goto l57
						l58:
							position, tokenIndex, depth = position58, tokenIndex58, depth58
						}
						if !matchDot() {
							goto l57
						}
						goto l56
					l57:
						position, tokenIndex, depth = position57, tokenIndex57, depth57
					}
					depth--
					add(RulePegText, position55)
				}
				if buffer[position] != rune('/') {
					goto l53
				}
				position++
				if !rules[RuleAction13]() {
					goto l53
				}
				depth--
				add(Ruleregex, position54)
			}
			return true
		l53:
			position, tokenIndex, depth = position53, tokenIndex53, depth53
			return false
		},
		/* 15 literal <- <<([a-z] / [A-Z] / ([0-9] / [0-9]) / '-' / '_')+>> */
		func() bool {
			position59, tokenIndex59, depth59 := position, tokenIndex, depth
			{
				position60 := position
				depth++
				{
					position61 := position
					depth++
					{
						position64, tokenIndex64, depth64 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l65
						}
						position++
						goto l64
					l65:
						position, tokenIndex, depth = position64, tokenIndex64, depth64
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l66
						}
						position++
						goto l64
					l66:
						position, tokenIndex, depth = position64, tokenIndex64, depth64
						{
							position68, tokenIndex68, depth68 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l69
							}
							position++
							goto l68
						l69:
							position, tokenIndex, depth = position68, tokenIndex68, depth68
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l67
							}
							position++
						}
					l68:
						goto l64
					l67:
						position, tokenIndex, depth = position64, tokenIndex64, depth64
						if buffer[position] != rune('-') {
							goto l70
						}
						position++
						goto l64
					l70:
						position, tokenIndex, depth = position64, tokenIndex64, depth64
						if buffer[position] != rune('_') {
							goto l59
						}
						position++
					}
				l64:
				l62:
					{
						position63, tokenIndex63, depth63 := position, tokenIndex, depth
						{
							position71, tokenIndex71, depth71 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l72
							}
							position++
							goto l71
						l72:
							position, tokenIndex, depth = position71, tokenIndex71, depth71
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l73
							}
							position++
							goto l71
						l73:
							position, tokenIndex, depth = position71, tokenIndex71, depth71
							{
								position75, tokenIndex75, depth75 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l76
								}
								position++
								goto l75
							l76:
								position, tokenIndex, depth = position75, tokenIndex75, depth75
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l74
								}
								position++
							}
						l75:
							goto l71
						l74:
							position, tokenIndex, depth = position71, tokenIndex71, depth71
							if buffer[position] != rune('-') {
								goto l77
							}
							position++
							goto l71
						l77:
							position, tokenIndex, depth = position71, tokenIndex71, depth71
							if buffer[position] != rune('_') {
								goto l63
							}
							position++
						}
					l71:
						goto l62
					l63:
						position, tokenIndex, depth = position63, tokenIndex63, depth63
					}
					depth--
					add(RulePegText, position61)
				}
				depth--
				add(Ruleliteral, position60)
			}
			return true
		l59:
			position, tokenIndex, depth = position59, tokenIndex59, depth59
			return false
		},
		/* 16 value <- <(<([a-z] / [A-Z] / ([0-9] / [0-9]) / '-' / '_' / '.')+> Action14)> */
		func() bool {
			position78, tokenIndex78, depth78 := position, tokenIndex, depth
			{
				position79 := position
				depth++
				{
					position80 := position
					depth++
					{
						position83, tokenIndex83, depth83 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l84
						}
						position++
						goto l83
					l84:
						position, tokenIndex, depth = position83, tokenIndex83, depth83
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l85
						}
						position++
						goto l83
					l85:
						position, tokenIndex, depth = position83, tokenIndex83, depth83
						{
							position87, tokenIndex87, depth87 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l88
							}
							position++
							goto l87
						l88:
							position, tokenIndex, depth = position87, tokenIndex87, depth87
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l86
							}
							position++
						}
					l87:
						goto l83
					l86:
						position, tokenIndex, depth = position83, tokenIndex83, depth83
						if buffer[position] != rune('-') {
							goto l89
						}
						position++
						goto l83
					l89:
						position, tokenIndex, depth = position83, tokenIndex83, depth83
						if buffer[position] != rune('_') {
							goto l90
						}
						position++
						goto l83
					l90:
						position, tokenIndex, depth = position83, tokenIndex83, depth83
						if buffer[position] != rune('.') {
							goto l78
						}
						position++
					}
				l83:
				l81:
					{
						position82, tokenIndex82, depth82 := position, tokenIndex, depth
						{
							position91, tokenIndex91, depth91 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l92
							}
							position++
							goto l91
						l92:
							position, tokenIndex, depth = position91, tokenIndex91, depth91
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l93
							}
							position++
							goto l91
						l93:
							position, tokenIndex, depth = position91, tokenIndex91, depth91
							{
								position95, tokenIndex95, depth95 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l96
								}
								position++
								goto l95
							l96:
								position, tokenIndex, depth = position95, tokenIndex95, depth95
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l94
								}
								position++
							}
						l95:
							goto l91
						l94:
							position, tokenIndex, depth = position91, tokenIndex91, depth91
							if buffer[position] != rune('-') {
								goto l97
							}
							position++
							goto l91
						l97:
							position, tokenIndex, depth = position91, tokenIndex91, depth91
							if buffer[position] != rune('_') {
								goto l98
							}
							position++
							goto l91
						l98:
							position, tokenIndex, depth = position91, tokenIndex91, depth91
							if buffer[position] != rune('.') {
								goto l82
							}
							position++
						}
					l91:
						goto l81
					l82:
						position, tokenIndex, depth = position82, tokenIndex82, depth82
					}
					depth--
					add(RulePegText, position80)
				}
				if !rules[RuleAction14]() {
					goto l78
				}
				depth--
				add(Rulevalue, position79)
			}
			return true
		l78:
			position, tokenIndex, depth = position78, tokenIndex78, depth78
			return false
		},
		/* 17 space <- <' '*> */
		func() bool {
			{
				position100 := position
				depth++
			l101:
				{
					position102, tokenIndex102, depth102 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l102
					}
					position++
					goto l101
				l102:
					position, tokenIndex, depth = position102, tokenIndex102, depth102
				}
				depth--
				add(Rulespace, position100)
			}
			return true
		},
		/* 18 q <- <('q' '(' <(!')' .)*> ')' Action15)> */
		func() bool {
			position103, tokenIndex103, depth103 := position, tokenIndex, depth
			{
				position104 := position
				depth++
				if buffer[position] != rune('q') {
					goto l103
				}
				position++
				if buffer[position] != rune('(') {
					goto l103
				}
				position++
				{
					position105 := position
					depth++
				l106:
					{
						position107, tokenIndex107, depth107 := position, tokenIndex, depth
						{
							position108, tokenIndex108, depth108 := position, tokenIndex, depth
							if buffer[position] != rune(')') {
								goto l108
							}
							position++
							goto l107
						l108:
							position, tokenIndex, depth = position108, tokenIndex108, depth108
						}
						if !matchDot() {
							goto l107
						}
						goto l106
					l107:
						position, tokenIndex, depth = position107, tokenIndex107, depth107
					}
					depth--
					add(RulePegText, position105)
				}
				if buffer[position] != rune(')') {
					goto l103
				}
				position++
				if !rules[RuleAction15]() {
					goto l103
				}
				depth--
				add(Ruleq, position104)
			}
			return true
		l103:
			position, tokenIndex, depth = position103, tokenIndex103, depth103
			return false
		},
		/* 20 Action0 <- <{ p.AddNull() }> */
		func() bool {
			{
				add(RuleAction0, position)
			}
			return true
		},
		/* 21 null <- <> */
		func() bool {
			{
				position112 := position
				depth++
				depth--
				add(Rulenull, position112)
			}
			return true
		},
		/* 22 Action1 <- <{ p.AddOperator(operatorIntersect) }> */
		func() bool {
			{
				add(RuleAction1, position)
			}
			return true
		},
		/* 23 Action2 <- <{ p.AddOperator(operatorSubtract) }> */
		func() bool {
			{
				add(RuleAction2, position)
			}
			return true
		},
		/* 24 Action3 <- <{ p.AddOperator(operatorUnion) }> */
		func() bool {
			{
				add(RuleAction3, position)
			}
			return true
		},
		/* 25 Action4 <- <{ p.AddBraces() }> */
		func() bool {
			{
				add(RuleAction4, position)
			}
			return true
		},
		/* 26 Action5 <- <{ p.AddGroupQuery() }> */
		func() bool {
			{
				add(RuleAction5, position)
			}
			return true
		},
		/* 27 Action6 <- <{ p.AddClusterLookup() }> */
		func() bool {
			{
				add(RuleAction6, position)
			}
			return true
		},
		/* 28 Action7 <- <{ p.AddGroupLookup() }> */
		func() bool {
			{
				add(RuleAction7, position)
			}
			return true
		},
		/* 29 Action8 <- <{ p.AddKeyLookup(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction8, position)
			}
			return true
		},
		/* 30 Action9 <- <{ p.AddLocalClusterLookup(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction9, position)
			}
			return true
		},
		/* 31 Action10 <- <{ p.AddFunction(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction10, position)
			}
			return true
		},
		/* 32 Action11 <- <{ p.AddFuncArg() }> */
		func() bool {
			{
				add(RuleAction11, position)
			}
			return true
		},
		/* 33 Action12 <- <{ p.AddFuncArg() }> */
		func() bool {
			{
				add(RuleAction12, position)
			}
			return true
		},
		nil,
		/* 35 Action13 <- <{ p.AddRegex(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction13, position)
			}
			return true
		},
		/* 36 Action14 <- <{ p.AddValue(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction14, position)
			}
			return true
		},
		/* 37 Action15 <- <{ p.AddConstant(buffer[begin:end]) }> */
		func() bool {
			{
				add(RuleAction15, position)
			}
			return true
		},
	}
	p.rules = rules
}