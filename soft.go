package lightsout

import (
	"fmt"
	"math/rand"

	"github.com/unixpickle/autofunc"
	"github.com/unixpickle/num-analysis/linalg"
	"github.com/unixpickle/weakai/neuralnet"
)

// SoftSolve attempts to solve the game via gradient
// descent, finding a fuzzy approximation of the solution.
//
// This will usually fail, since gradient descent is not
// guaranteed to find the global minimum.
func SoftSolve(g *State) []Move {
	solution := &autofunc.Variable{Vector: make(linalg.Vector, 25)}
	for i := range solution.Vector {
		solution.Vector[i] = rand.NormFloat64()
	}

	initState := &autofunc.Variable{Vector: make(linalg.Vector, 25)}
	for i := 0; i < 25; i++ {
		if (*g)&(1<<uint(i)) != 0 {
			initState.Vector[i] = 1
		}
	}

	desired := make(linalg.Vector, 25)
	for i := 0; i < 500; i++ {
		probs := autofunc.Sigmoid{}.Apply(solution)
		state := SoftMover{}.Apply(autofunc.Concat(probs, initState))
		cost := neuralnet.AbsCost{}.Cost(desired, state)
		grad := autofunc.NewGradient([]*autofunc.Variable{solution})
		cost.PropagateGradient([]float64{1}, grad)
		grad.AddToVars(-5)
	}

	moves := make([]Move, 0, 25)
	state := *g
	for i, x := range solution.Vector {
		if x > 0 {
			m := Move{Row: i / 5, Col: i % 5}
			moves = append(moves, m)
			state.Move(m)
		}
	}
	if !state.Solved() {
		return nil
	}
	return moves
}

// SoftMover is an autofunc.RFunc for applying moves to a
// probabilistic game state.
type SoftMover struct{}

// Apply applies a set of moves to a game state.
//
// The first 25 components describe the probability of
// making each of the 25 moves.
// Each probability may range between 0 and 1, independent
// of all the other moves.
//
// The remaining 25 components of the input correspond to
// the light state probabilities, where 1 is "on" and 0 is
// "off".
//
// The result is a new set of light state probabilities.
func (_ SoftMover) Apply(in autofunc.Result) autofunc.Result {
	if len(in.Output()) != 50 {
		panic(fmt.Sprintf("expected input size %d (got %d)", 50, len(in.Output())))
	}
	return autofunc.Pool(in, func(in autofunc.Result) autofunc.Result {
		start := autofunc.Slice(in, 25, 50)
		probs := autofunc.Split(25, autofunc.Slice(in, 0, 25))
		var idx int
		return autofunc.Fold(start, probs, func(s, prob autofunc.Result) autofunc.Result {
			comp := autofunc.AddScaler(autofunc.Scale(prob, -1), 1)
			idx++
			return autofunc.Add(
				autofunc.ScaleFirst(s, comp),
				autofunc.ScaleFirst(SoftMover{}.softMove(s, idx-1), prob),
			)
		})
	})
}

func (_ SoftMover) softMove(board autofunc.Result, idx int) autofunc.Result {
	indices := SoftMover{}.indicesForMove(idx)
	return &softMoveResult{
		Input:   board,
		Flipped: indices,
		OutVec:  SoftMover{}.complementValues(board.Output(), indices),
	}
}

func (_ SoftMover) indicesForMove(idx int) []int {
	x := idx % BoardSize
	y := idx / BoardSize
	res := make([]int, 1, 5)
	res[0] = idx
	if x > 0 {
		res = append(res, idx-1)
	}
	if x < 4 {
		res = append(res, idx+1)
	}
	if y > 0 {
		res = append(res, idx-5)
	}
	if y < 4 {
		res = append(res, idx+5)
	}
	return res
}

func (_ SoftMover) complementValues(in linalg.Vector, idxs []int) linalg.Vector {
	res := make(linalg.Vector, len(in))
	copy(res, in)
	for _, idx := range idxs {
		res[idx] = 1 - in[idx]
	}
	return res
}

type softMoveResult struct {
	Input   autofunc.Result
	Flipped []int
	OutVec  linalg.Vector
}

func (s *softMoveResult) Output() linalg.Vector {
	return s.OutVec
}

func (s *softMoveResult) Constant(g autofunc.Gradient) bool {
	return s.Input.Constant(g)
}

func (s *softMoveResult) PropagateGradient(u linalg.Vector, g autofunc.Gradient) {
	if s.Constant(g) {
		return
	}
	for _, i := range s.Flipped {
		u[i] *= -1
	}
	s.Input.PropagateGradient(u, g)
}
