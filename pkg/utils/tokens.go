package utils

import (
	"math/big"
	"sort"
)

type Partitioner struct {
	RingOffset *big.Int
	RingRange  *big.Int
}

var one = bigInt(1)

var Murmur3Partitioner = Partitioner{
	RingOffset: new(big.Int).Lsh(bigInt(-1), 63), // -1<<63
	RingRange:  new(big.Int).Lsh(one, 64),        // 1<<64
}

var RandomPartitioner = Partitioner{
	RingOffset: new(big.Int),               // 0
	RingRange:  new(big.Int).Lsh(one, 127), // 1<<127
}

// ComputeTokens takes a list of numbers indicating the number of nodes in each datacenter, and returns a recommended
// list of tokens for the given partitioner (one token per node, grouped by datacenter).
func ComputeTokens(dcCounts []int, partitioner Partitioner) [][]string {
	dcOffset := computeDcOffset(dcCounts, partitioner)
	tokensPerDc := make([][]string, len(dcCounts))
	for dc, dcCount := range dcCounts {
		start := new(big.Int).Mul(dcOffset, bigInt(dc))
		increment := new(big.Int).Div(partitioner.RingRange, bigInt(dcCount))
		tokens := make([]*big.Int, dcCount)
		for i := 0; i < dcCount; i++ {
			// token := i*increment + start + ringOffset
			token := bigInt(i)
			token = token.Mul(token, increment)
			token = token.Add(token, start)
			token = token.Add(token, partitioner.RingOffset)

			if token.Cmp(partitioner.RingOffset) < 0 {
				token = token.Add(token, partitioner.RingRange)
			}

			tokens[i] = token
		}
		sort.Slice(tokens, func(i, j int) bool { return tokens[i].Cmp(tokens[j]) < 0 })

		tokensPerDc[dc] = make([]string, len(tokens))
		for i, token := range tokens {
			tokensPerDc[dc][i] = token.String()
		}
	}
	return tokensPerDc
}

// computeDcOffset calculates an offset to spread the starting point of each datacenter: our strategy is to start each
// DC from a "zero" point, and distribute its tokens evenly across the ring; however nodes cannot share tokens, so the
// "zeros" must be slightly apart.
func computeDcOffset(dcCounts []int, partitioner Partitioner) *big.Int {

	maxDcCount := 0
	for _, dcCount := range dcCounts {
		if dcCount > maxDcCount {
			maxDcCount = dcCount
		}
	}

	// Ensure that we preserve the original order, i.e. the offset does not push us past the next token of the previous
	// DC. For example, if there are 3 DCs with 4 nodes each, and we picture the ring as a 360° circle, the first two
	// tokens of DC1 would be at 0° and 90°. So the next two starting points must be between 0 and 90.
	// 360 / (3*4*2) gets us there.
	divisor := len(dcCounts) * maxDcCount * 2

	// Copied verbatim from the DSE code that this is adapted from. The intent is not clear (it looks like this will
	// almost always kick in, so the previous computation is irrelevant). But we keep it as-is to be able to compare
	// results.
	const minDivisor = 235
	if divisor < minDivisor {
		divisor = minDivisor
	}

	result := new(big.Int).Neg(partitioner.RingRange)
	return result.Div(result, bigInt(divisor))
}

func bigInt(i int) *big.Int {
	return new(big.Int).SetInt64(int64(i))
}
