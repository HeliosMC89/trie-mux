// Package trie implements a minimal tree based url path router (or mux) for Go.

package trie

import (
	"fmt"
	"regexp"
	"strings"
)

// Options is options for Trie.
type Options struct {
	// Ignore case when matching URL path.
	IgnoreCase bool

	// If enabled, the trie will detect if the current path can't be matched but
	// a handler for the fixed path exists.
	// Matched.FPR will returns either a fixed redirect path or an empty string.
	// For example when "/api/foo" defined and matching "/api//foo",
	// The result Matched.FPR is "/api/foo".
	FixedPathRedirect bool

	// If enabled, the trie will detect if the current path can't be matched but
	// a handler for the path with (without) the trailing slash exists.
	// Matched.TSR will returns either a redirect path or an empty string.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo
	// For example when "/api/foo" defined and matching "/api/foo/",
	// The result Matched.TSR is "/api/foo".
	TrailingSlashRedirect bool
}

var (
	wordReg        = regexp.MustCompile("^\\w+$")
	doubleColonReg = regexp.MustCompile("^::\\w*$")
	defaultOptions = Options{
		IgnoreCase:            true,
		TrailingSlashRedirect: true,
		FixedPathRedirect:     true,
	}
)

// New returns a trie
//
//  trie := New()
//  // disable IgnoreCase, TrailingSlashRedirect and FixedPathRedirect
//  trie := New(Options{})
//
func New(args ...Options) *Trie {
	opts := defaultOptions
	if len(args) > 0 {
		opts = args[0]
	}

	return &Trie{
		ignoreCase: opts.IgnoreCase,
		fpr:        opts.FixedPathRedirect,
		tsr:        opts.TrailingSlashRedirect,
		root: &Node{
			parentNode:      nil,
			literalChildren: map[string]*Node{},
			Methods:         map[string]interface{}{},
		},
	}
}

// Trie represents a trie that defining patterns and matching URL.
type Trie struct {
	ignoreCase bool
	fpr        bool
	tsr        bool
	root       *Node
}

// Define define a pattern on the trie and returns the endpoint node for the pattern.
//
//  trie := New()
//  node1 := trie.Define("/a")
//  node2 := trie.Define("/a/b")
//  node3 := trie.Define("/a/b")
//  // node2.parentNode == node1
//  // node2 == node3
//
func (t *Trie) Define(pattern string) *Node {
	if strings.Contains(pattern, "//") {
		panic(fmt.Errorf(`Multi-slash exist: "%s"`, pattern))
	}

	_pattern := strings.TrimPrefix(pattern, "/")
	node := defineNode(t.root, strings.Split(_pattern, "/"), t.ignoreCase)

	if node.pattern == "" {
		node.pattern = pattern
	}
	return node
}

// Match try to match path. It will returns a Matched instance that
// includes	*Node, Params and Tsr flag when matching success, otherwise a nil.
//
//  matched := trie.Match("/a/b")
//
// The defined pattern can contain three types of parameters:
//
//  Syntax         Type
//  :name          named parameter
//  :name*         named with catch-all parameter
//  :name(regexp)  named with regexp parameter
//
// Named parameters are dynamic path segments. They match anything until the
// next '/' or the path end:
//
//  Defined: /api/:type/:ID
//
//  Requests:
//   /api/user/123             matched: type="user", ID="123"
//   /api/user                 no match
//   /api/user/123/comments    no match
//
// Named with catch-all parameters match anything until the path end, including the
// directory index (the '/' before the catch-all). Since they match anything
// until the end, catch-all parameters must always be the final path element.
//
//  Defined: /files/:filepath*
//
//  Requests:
//   /files                              no match
//   /files/LICENSE                      matched: filepath="LICENSE"
//   /files/templates/article.html       matched: filepath="templates/article.html"
//
// Named with regexp parameters match anything using regexp until the
// next '/' or the path end:
//
//  Defined: /api/:type/:ID(^\\d+$)
//
//  Requests:
//   /api/user/123             matched: type="user", ID="123"
//   /api/user                 no match
//   /api/user/abc             no match
//   /api/user/123/comments    no match
//
// The value of parameters is saved on the matched.Params. Retrieve the value of a parameter by name:
//
//  type := matched.Params["type"]
//  id   := matched.Params["ID"]
//
func (t *Trie) Match(path string) *Matched {
	_path := path
	parent := t.root
	if t.fpr {
		path = fixPath(_path)
	}
	frags := strings.Split(strings.TrimPrefix(path, "/"), "/")

	res := &Matched{}
	for i, frag := range frags {
		_frag := frag
		if t.ignoreCase {
			_frag = strings.ToLower(frag)
		}

		node, named := matchNode(parent, _frag)
		if node == nil {
			// TrailingSlashRedirect: /acb/efg/ -> /acb/efg
			if t.tsr && frag == "" && len(frags) == (i+1) && parent.endpoint {
				res.TSR = path[:len(path)-1]
				if t.fpr && path != _path {
					res.FPR = res.TSR
					res.TSR = ""
				}
			}
			return res
		}
		parent = node

		if named {
			if res.Params == nil {
				res.Params = map[string]string{}
			}
			if node.wildcard {
				res.Params[node.name] = strings.Join(frags[i:], "/")
				break
			} else {
				res.Params[node.name] = frag
			}
		}
	}

	if parent.endpoint {
		res.Node = parent
		if t.fpr && path != _path {
			res.FPR = path
			res.Node = nil
		}
	} else if t.tsr && parent.literalChildren[""] != nil {
		// TrailingSlashRedirect: /acb/efg -> /acb/efg/
		res.TSR = path + "/"
		if t.fpr && path != _path {
			res.FPR = res.TSR
			res.TSR = ""
		}
	}
	return res
}

// Node represents a node on defined patterns that can be matched.
type Node struct {
	// Methods defined on the node
	//
	//  trie := New()
	//  trie.Define("/").Handle("GET", handler1)
	//  trie.Define("/").Handle("PUT", handler2)
	//
	//  trie.Match("/").Node.AllowMethods == "GET, PUT"
	//
	AllowMethods string

	// Method & Handler map defined on the node
	//
	//  trie := New()
	//  trie.Define("/api").Handle("GET", func handler1() {})
	//  trie.Define("/api").Handle("PUT", func handler2() {})
	//
	//  trie.Match("/api").Node.Methods["GET"].(func()) == handler1
	//  trie.Match("/api").Node.Methods["PUT"].(func()) == handler2
	//
	Methods map[string]interface{}

	pattern         string
	name            string
	endpoint        bool
	wildcard        bool
	regex           *regexp.Regexp
	parentNode      *Node
	varyChild       *Node
	literalChildren map[string]*Node
}

// Handle is used to mount a handler with a method name to the node.
//
//  t := New()
//  node := t.Define("/a/b")
//  node.Handle("GET", handler1)
//  node.Handle("POST", handler1)
//
func (n *Node) Handle(method string, handler interface{}) {
	if n.Methods[method] != nil {
		panic(fmt.Errorf(`"%s" already defined`, n.pattern))
	}
	n.Methods[method] = handler
	if n.AllowMethods == "" {
		n.AllowMethods = method
	} else {
		n.AllowMethods += ", " + method
	}
}

// Matched is a result returned by Trie.Match.
type Matched struct {
	// Either a Node pointer when matched or nil
	Node *Node

	// Either a map contained matched values or empty map.
	Params map[string]string

	// If FixedPathRedirect enabled, it may returns a redirect path,
	// otherwise a empty string.
	FPR string

	// If TrailingSlashRedirect enabled, it may returns a redirect path,
	// otherwise a empty string.
	TSR string
}

func defineNode(parent *Node, frags []string, ignoreCase bool) *Node {
	frag := frags[0]
	frags = frags[1:]
	child := parseNode(parent, frag, ignoreCase)

	if len(frags) == 0 {
		child.endpoint = true
		return child
	} else if child.wildcard {
		panic(fmt.Errorf(`Can't define pattern after wildcard: "%s"`, child.pattern))
	}
	return defineNode(child, frags, ignoreCase)
}

func matchNode(parent *Node, frag string) (child *Node, named bool) {
	if child = parent.literalChildren[frag]; child != nil {
		return
	}

	if child = parent.varyChild; child != nil {
		if child.regex != nil && !child.regex.MatchString(frag) {
			child = nil
		} else {
			named = true
		}
	}
	return
}

func parseNode(parent *Node, frag string, ignoreCase bool) *Node {
	literalChildren := parent.literalChildren

	_frag := frag
	if doubleColonReg.MatchString(frag) {
		_frag = frag[1:]
	}
	if ignoreCase {
		_frag = strings.ToLower(_frag)
	}

	if literalChildren[_frag] != nil {
		return literalChildren[_frag]
	}

	node := &Node{
		parentNode:      parent,
		literalChildren: map[string]*Node{},
		Methods:         map[string]interface{}{},
	}

	if frag == "" {
		literalChildren[frag] = node
	} else if doubleColonReg.MatchString(frag) {
		// pattern "/a/::" should match "/a/:"
		// pattern "/a/::bc" should match "/a/:bc"
		// pattern "/a/::/bc" should match "/a/:/bc"
		literalChildren[_frag] = node
	} else if frag[0] == ':' {
		var name, regex string
		name = frag[1:]
		trailing := name[len(name)-1]
		if trailing == ')' {
			if index := strings.IndexRune(name, '('); index > 0 {
				regex = name[index+1 : len(name)-1]
				if len(regex) > 0 {
					name = name[0:index]
					node.regex = regexp.MustCompile(regex)
				} else {
					panic(fmt.Errorf(`Invalid pattern: "%s"`, frag))
				}
			}
		} else if trailing == '*' {
			name = name[0 : len(name)-1]
			node.wildcard = true
		}
		// name must be word characters `[0-9A-Za-z_]`
		if !wordReg.MatchString(name) {
			panic(fmt.Errorf(`Invalid pattern: "%s"`, frag))
		}
		node.name = name
		if child := parent.varyChild; child != nil {
			if child.name != name || child.wildcard != node.wildcard {
				panic(fmt.Errorf(`Invalid pattern: "%s"`, frag))
			}
			if child.regex != nil && child.regex.String() != regex {
				panic(fmt.Errorf(`Invalid pattern: "%s"`, frag))
			}
			return child
		}

		parent.varyChild = node
	} else if frag[0] == '*' || frag[0] == '(' || frag[0] == ')' {
		panic(fmt.Errorf(`Invalid pattern: "%s"`, frag))
	} else {
		literalChildren[_frag] = node
	}

	return node
}

func fixPath(path string) string {
	if !strings.Contains(path, "//") {
		return path
	}
	return fixPath(strings.Replace(path, "//", "/", -1))
}
