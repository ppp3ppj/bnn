package visitor

import (
	"fmt"
	"strings"

	"github.com/ppp3ppj/bnn/ast"
	bnnlog "github.com/ppp3ppj/bnn/internal/log"
)

// Resolve returns the bunches sorted in dependency order using Kahn's algorithm.
// Bunches with no dependencies come first. When multiple bunches are ready at
// the same time their original declaration order is preserved.
// Returns an error if a cycle is detected (validate first to get a better message).
func Resolve(m *ast.ManifestNode) ([]ast.BunchNode, error) {
	n := len(m.Bunches)
	if n == 0 {
		return nil, nil
	}

	// name → position in original slice
	index := make(map[string]int, n)
	for i, b := range m.Bunches {
		index[b.Name] = i
	}

	// inDegree[i] = number of unresolved dependencies for bunch i
	inDegree := make([]int, n)
	// dependents[i] = indices of bunches that directly depend on bunch i
	dependents := make([][]int, n)

	for i, b := range m.Bunches {
		for _, dep := range b.Depends {
			j, ok := index[dep]
			if !ok {
				return nil, fmt.Errorf("[bnn] bunch '%s' — depends on '%s' which is not declared", b.Name, dep)
			}
			inDegree[i]++
			dependents[j] = append(dependents[j], i)
		}
	}

	// seed queue with every bunch that has no dependencies, in original order
	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	result := make([]ast.BunchNode, 0, n)
	for len(queue) > 0 {
		// pop front
		cur := queue[0]
		queue = queue[1:]
		result = append(result, m.Bunches[cur])

		// collect newly ready dependents, sort by original index to keep stable order
		ready := make([]int, 0)
		for _, dep := range dependents[cur] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				ready = append(ready, dep)
			}
		}
		// insert in ascending index order to preserve declaration order
		insertSorted(ready, &queue)
	}

	if bnnlog.Enabled() {
		names := make([]string, len(result))
		for i, b := range result {
			names[i] = b.Name
		}
		bnnlog.Debug("resolve: order → %s", strings.Join(names, " → "))
	}

	if len(result) != n {
		// find names still stuck (in-degree > 0)
		var stuck []string
		for i, b := range m.Bunches {
			if inDegree[i] > 0 {
				stuck = append(stuck, b.Name)
			}
		}
		return nil, fmt.Errorf("[bnn] circular dependency — cannot resolve order for: %s", strings.Join(stuck, ", "))
	}

	return result, nil
}

// insertSorted merges new (already sorted ascending) into queue preserving order.
func insertSorted(new []int, queue *[]int) {
	if len(new) == 0 {
		return
	}
	// sort new ascending (insertion sort — list is tiny)
	for i := 1; i < len(new); i++ {
		for j := i; j > 0 && new[j] < new[j-1]; j-- {
			new[j], new[j-1] = new[j-1], new[j]
		}
	}
	*queue = append(*queue, new...)
}
