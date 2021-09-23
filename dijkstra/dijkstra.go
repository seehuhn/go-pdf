// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package dijkstra

// ShortestPath implements ShortestPath's algorithm
// https://en.wikipedia.org/wiki/Dijkstra%27s_algorithm
//     vertices: 0, 1, ..., n, start at 0, end at n
//     edges: (k, l) with 0 <= k < l <= n
func ShortestPath(cost func(i, j int) int, n int) (int, []int) {
	dist := make([]int, n)
	to := make([]int, n)
	for i := 0; i < n; i++ {
		dist[i] = cost(i, n)
		to[i] = n
	}

	pos := n
	for pos > 0 {
		bestNode, bestDist := 0, dist[0]
		for i := 1; i < pos; i++ {
			if dist[i] < bestDist {
				bestNode = i
				bestDist = dist[i]
			}
		}
		pos = bestNode

		for i := 0; i < pos; i++ {
			alt := bestDist + cost(i, pos)
			if alt < dist[i] {
				dist[i] = alt
				to[i] = pos
			}
		}
	}

	res := []int{0}
	pos = 0
	for pos < n {
		pos = to[pos]
		res = append(res, pos)
	}
	return dist[0], res
}
