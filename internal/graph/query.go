package graph

// adjacency builds forward or reverse adjacency lists on demand.
func (g *Graph) adjacency(reverse bool) [][]int32 {
	adj := make([][]int32, len(g.Nodes))
	for _, e := range g.Edges {
		s, d := e[0], e[1]
		if reverse {
			s, d = d, s
		}
		adj[s] = append(adj[s], d)
	}
	return adj
}

func bfs(adj [][]int32, starts []int32) []int32 {
	seen := make([]bool, len(adj))
	queue := append([]int32(nil), starts...)
	for _, s := range starts {
		seen[s] = true
	}
	var out []int32
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		out = append(out, n)
		for _, m := range adj[n] {
			if !seen[m] {
				seen[m] = true
				queue = append(queue, m)
			}
		}
	}
	return out
}

// Forward returns every node transitively reachable from starts (inclusive).
func (g *Graph) Forward(starts []int32) []int32 { return bfs(g.adjacency(false), starts) }

// Reverse returns every node that can transitively reach starts (inclusive).
func (g *Graph) Reverse(starts []int32) []int32 { return bfs(g.adjacency(true), starts) }

// SCCs returns non-trivial strongly connected components (size > 1, or a
// single node with a self-edge) via iterative Tarjan.
func (g *Graph) SCCs() [][]int32 {
	adj := g.adjacency(false)
	selfLoop := make(map[int32]bool)
	for _, e := range g.Edges {
		if e[0] == e[1] {
			selfLoop[e[0]] = true
		}
	}
	n := len(g.Nodes)
	const unvisited = -1
	index := make([]int, n)
	low := make([]int, n)
	onStack := make([]bool, n)
	for i := range index {
		index[i] = unvisited
	}
	var stack []int32
	var sccs [][]int32
	next := 0

	type frame struct {
		v  int32
		ei int
	}
	for start := 0; start < n; start++ {
		if index[start] != unvisited {
			continue
		}
		work := []frame{{int32(start), 0}}
		for len(work) > 0 {
			f := &work[len(work)-1]
			v := f.v
			if f.ei == 0 {
				index[v] = next
				low[v] = next
				next++
				stack = append(stack, v)
				onStack[v] = true
			}
			advanced := false
			for f.ei < len(adj[v]) {
				w := adj[v][f.ei]
				f.ei++
				if index[w] == unvisited {
					work = append(work, frame{w, 0})
					advanced = true
					break
				}
				if onStack[w] && index[w] < low[v] {
					low[v] = index[w]
				}
			}
			if advanced {
				continue
			}
			if low[v] == index[v] {
				var scc []int32
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					scc = append(scc, w)
					if w == v {
						break
					}
				}
				if len(scc) > 1 || selfLoop[scc[0]] {
					sccs = append(sccs, scc)
				}
			}
			work = work[:len(work)-1]
			if len(work) > 0 {
				p := work[len(work)-1].v
				if low[v] < low[p] {
					low[p] = low[v]
				}
			}
		}
	}
	return sccs
}

// Dead returns nodes not reachable from any root (main/init).
func (g *Graph) Dead() []int32 {
	alive := make([]bool, len(g.Nodes))
	for _, i := range g.Forward(g.Roots) {
		alive[i] = true
	}
	var dead []int32
	for i := range g.Nodes {
		if !alive[i] {
			dead = append(dead, int32(i))
		}
	}
	return dead
}
