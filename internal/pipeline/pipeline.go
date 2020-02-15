// Copyright (C) 2020  David N. Cohen
// This file is part of github.com/dncohen/rcl
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

// Pipeline package
//
// Command that expect RCL transactions as input or output can use
// pipeline helper function to encode and decode transactions to stdin
// or stdout.  Or, to files.  Pipeline uses JSON as the underlying
// encoding.
package pipeline

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/rubblelabs/ripple/data"
)

var emptyHash = data.Hash256{}

func DecodeInput(c chan data.Transaction, r io.Reader) error {
	dec := json.NewDecoder(r)

	for dec.More() {

		// we rely on rubblelabs' ability to decode into
		// TransactionWithMetaData (even though we don't expect metadata to
		// actually be present).
		tx := &data.TransactionWithMetaData{}

		err := dec.Decode(tx)
		if err != nil {
			if errors.Is(err, io.EOF) { // not reached
				break
			}
			return err
		}

		// TODO(dnc): try to eliminate unwanted empty hash when tx unsigned
		if tx.Transaction.GetBase().Hash == emptyHash {
			//tx.Transaction.GetBase().Hash = nil
		}

		c <- tx.Transaction // empty metadata discarded here
	}

	return nil
}

func EncodeOutput(w io.Writer, c chan data.Transaction) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")

	for tx := range c {
		err := enc.Encode(tx)
		if err != nil {
			return err
		}
	}
	return nil
}
