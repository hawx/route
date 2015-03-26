package route

import (
	"net/http"
	"path"
	"strings"
)

/*

The idea here is that any path, say /user/me/create, can be simply turned
into a tree by splitting on '/' and using these as names for the edges:

  ( ) --[user]--> ( ) --[me]--> ( ) --[create]--> (http.Handler)

We can then add other branches, like /user/me and /user/me/edit:

  ( ) --[user]--> ( ) --[me]--> (http.Handler) --[create]--> (http.Handler)
                                      |
                                      | --[edit]--> (http.Handler)

And now to retrieve a particular handler we can take the request path /user/me,
split on '/', and traverse the tree to get the http.Handler.

Now let's say instead of /user/me/create we use a parameter to get
/user/:name/create. Now instead of using an "exact" edge -- one where we can
simply compare string values to traverse -- we say that if a "wild" edge exists,
and no "exact" edge matches, we take the "wild" edge.

Finally consider the case of greedy parameters. Here we have a route like
/image/\*path where /image/some/photo.jpg and /image/no/really/this/photo.jpg
would both match. It acts like a leaf on the tree, and I've called it a "greedy"
leaf since it will match any input. These are always considered after exact and
wild matches have failed.

There is one interesting edge case to discuss. Consider the following tree.

  ( ) --[image]--> ( ) --[my]--> ( ) --[photo.jpg]--> (http.Handler)
                    |
                    |--[*path]--> (http.Handler)

For a path like /image/my/cat.gif we would start by following the image->my
edges but then hit a dead-end, when in fact we could have matched
image->*path. Therefore we must be careful in situations like this to backtrack.

*/

func newLookup() *treeLookup {
	return &treeLookup{root: &node{children: map[string]*node{}, value: nil}}
}

type treeLookup struct {
	root *node
}

type node struct {
	// children is a map of path fragments, eg. /create, /user, etc. to a list of
	// children.
	children map[string]*node

	// wildedge is set if the path fragment was :something, the edge then contains
	// the next node.
	wildedge *wildedge

	// greedyleaf contains a greedyleaf if the path fragment was *something, the
	// leaf then contains the value.
	greedyleaf *greedyleaf

	// value contains the handler, if any.
	value http.Handler
}

type wildedge struct {
	// child at end of edge
	child *node

	// name of parameter
	name string
}

type greedyleaf struct {
	// value contain the handler.
	value http.Handler

	// name of parameter
	name string
}

func (look *treeLookup) Add(path string, handler http.Handler) {
	if path != "/" && strings.HasSuffix(path, "/") {
		panic("cannot insert path with trailing slash: " + path)
	}

	parts := strings.Split(path, "/")[1:]

	look.root.add(parts, handler)
}

func (curr *node) add(parts []string, handler http.Handler) {
	part := parts[0]
	parts = parts[1:]

	child, ok := curr.children[part]
	if !ok {
		child = &node{children: map[string]*node{}, value: nil}

		if strings.HasPrefix(part, ":") {
			// Check if we already have a wildedge, if so check it has same name, then
			// move to its child. Otherwise create new wildedge
			if curr.wildedge != nil {
				if curr.wildedge.name != part[1:] {
					panic("wildedge with different name already registered")
				}
				child = curr.wildedge.child

			} else {
				if part == ":" {
					panic("parameter name is empty")
				}
				curr.wildedge = &wildedge{name: part[1:], child: child}
			}

		} else if strings.HasPrefix(part, "*") {
			if len(parts) > 0 {
				panic("path after greedy parameter")
			}
			if part == "*" {
				panic("greedy parameter name is empty")
			}
			if curr.greedyleaf != nil {
				panic("greedy parameter already registered")
			}

			curr.greedyleaf = &greedyleaf{name: part[1:], value: handler}
			return
		} else {
			curr.children[part] = child
		}
	}

	// go deeper into the tree
	if len(parts) > 0 {
		child.add(parts, handler)
		return
	}

	// child has a value
	child.value = handler
}

func (look *treeLookup) Get(path string) (http.Handler, map[string]string) {
	params := map[string]string{}

	if path != "/" && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	parts := strings.Split(path, "/")[1:]

	return look.root.get(parts, params)
}

func (curr *node) get(parts []string, pars map[string]string) (http.Handler, map[string]string) {
	if len(parts) == 0 {
		// If it has a greedyleaf we have an empty match
		if curr.greedyleaf != nil {
			pars[curr.greedyleaf.name] = ""
			return curr.greedyleaf.value, pars
		}

		return curr.value, pars
	}

	child, ok := curr.children[parts[0]]
	if !ok {
		if curr.wildedge != nil {
			// If we have a parameter, add the parameter and make the child the node
			// at the end of the edge.
			pars[curr.wildedge.name] = parts[0]
			child = curr.wildedge.child

		} else if curr.greedyleaf != nil {
			// If we have a greedyleaf, add the parameter and return the handler.
			pars[curr.greedyleaf.name] = strings.Join(parts, "/")
			return curr.greedyleaf.value, pars

		} else {
			// If no matches, return the nil value and params so far.
			return nil, pars
		}
	}

	// Go deeper into the tree
	handler, pars := child.get(parts[1:], pars)

	if handler == nil && curr.wildedge != nil {
		if child == curr.wildedge.child {
			// If we added a parameter at this depth, but there was no handler further on,
			// remove it.
			delete(pars, curr.wildedge.name)

		} else {
			// If we didn't take the wildedge last time, do now
			pars[curr.wildedge.name] = parts[0]
			child = curr.wildedge.child

			handler, pars = child.get(parts[1:], pars)
		}
	}

	// If we had no match deeper in the tree, try to match a greedyleaf.
	if handler == nil && curr.greedyleaf != nil {
		pars[curr.greedyleaf.name] = strings.Join(parts, "/")
		return curr.greedyleaf.value, pars
	}

	return handler, pars
}

// Taken from net/http
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}
	return np
}
