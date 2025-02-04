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

package transactions

import (
	"bytes"

	"github.com/vincentbdb/go-algorand/crypto"
)

// LogicSig contains logic for validating a transaction.
// LogicSig is signed by an account, allowing delegation of operations.
// OR
// LogicSig defines a contract account.
type LogicSig struct {
	_struct struct{} `codec:",omitempty,omitemptyarray"`

	// Logic signed by Sig or Msig, OR hashed to be the Address of an account.
	Logic []byte `codec:"l"`

	Sig  crypto.Signature   `codec:"sig"`
	Msig crypto.MultisigSig `codec:"msig"`

	// Args are not signed, but checked by Logic
	Args [][]byte `codec:"arg"`
}

// Blank returns true if there is no content in this LogicSig
func (lsig *LogicSig) Blank() bool {
	return len(lsig.Logic) == 0
}

// Len returns the length of Logic plus the length of the Args
// This is limited by config.ConsensusParams.LogicSigMaxSize
func (lsig *LogicSig) Len() int {
	lsiglen := len(lsig.Logic)
	for _, arg := range lsig.Args {
		lsiglen += len(arg)
	}
	return lsiglen
}

// Equal returns true if both LogicSig are equivalent
func (lsig *LogicSig) Equal(b *LogicSig) bool {
	if len(lsig.Logic) == 0 && len(b.Logic) == 0 {
		return true
	}
	sigs := lsig.Sig == b.Sig && lsig.Msig.Equal(b.Msig)
	if !sigs {
		return false
	}
	if len(lsig.Args) != len(b.Args) {
		return false
	}
	for i, a := range lsig.Args {
		if !bytes.Equal(a, b.Args[i]) {
			return false
		}
	}
	return true
}
