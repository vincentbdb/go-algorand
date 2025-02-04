// Copyright (C) 2019 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package basics

import (
	"github.com/vincentbdb/go-algorand/config"
	"github.com/vincentbdb/go-algorand/crypto"
	"github.com/algorand/go-codec/codec"
)

// RoundInterval is a number of rounds
type RoundInterval uint64

// MicroAlgos is our unit of currency.  It is wrapped in a struct to nudge
// developers to use an overflow-checking library for any arithmetic.
type MicroAlgos struct {
	Raw uint64
}

// LessThan implements arithmetic comparison for MicroAlgos
func (a MicroAlgos) LessThan(b MicroAlgos) bool {
	return a.Raw < b.Raw
}

// GreaterThan implements arithmetic comparison for MicroAlgos
func (a MicroAlgos) GreaterThan(b MicroAlgos) bool {
	return a.Raw > b.Raw
}

// IsZero implements arithmetic comparison for MicroAlgos
func (a MicroAlgos) IsZero() bool {
	return a.Raw == 0
}

// ToUint64 converts the amount of algos to uint64
func (a MicroAlgos) ToUint64() uint64 {
	return a.Raw
}

// RewardUnits returns the number of reward units in some number of algos
func (a MicroAlgos) RewardUnits(proto config.ConsensusParams) uint64 {
	return a.Raw / proto.RewardUnit
}

// CodecEncodeSelf implements codec.Selfer to encode MicroAlgos as a simple int
func (a MicroAlgos) CodecEncodeSelf(enc *codec.Encoder) {
	enc.MustEncode(a.Raw)
}

// CodecDecodeSelf implements codec.Selfer to decode MicroAlgos as a simple int
func (a *MicroAlgos) CodecDecodeSelf(dec *codec.Decoder) {
	dec.MustDecode(&a.Raw)
}

// Round represents a protocol round index
type Round uint64

// OneTimeIDForRound maps a round to the identifier for which ephemeral key
// should be used for that round.  keyDilution specifies the number of keys
// in the bottom-level of the two-level key structure.
func OneTimeIDForRound(round Round, keyDilution uint64) crypto.OneTimeSignatureIdentifier {
	return crypto.OneTimeSignatureIdentifier{
		Batch:  uint64(round) / keyDilution,
		Offset: uint64(round) % keyDilution,
	}
}

// SubSaturate subtracts two rounds with saturation arithmetic that does not
// wrap around past zero, and instead returns 0 on underflow.
func (round Round) SubSaturate(x Round) Round {
	if round < x {
		return 0
	}

	return round - x
}
