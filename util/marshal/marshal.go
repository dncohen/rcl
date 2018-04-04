package marshal

import (
	"encoding/gob"
	"encoding/hex"
	"io"
	"log"

	"github.com/rubblelabs/ripple/data"
	"github.com/y0ssar1an/q"
)

func register() {
	// When decoding, we need to specify in advance the types gob will decode.
	// When encoding, we register here to spare the caller the trouble.

	// Should have all tx types here, I am putting them here when needed.
	gob.Register(&data.AccountSet{})
	gob.Register(&data.OfferCancel{})
	gob.Register(&data.OfferCreate{})
	gob.Register(&data.Payment{})
	gob.Register(&data.TrustSet{})

}

// Returns io.EOF at end of input.
func DecodeTransactions(in io.Reader, txs chan (data.Transaction)) error {

	// Register instances of what we accept.
	register()

	// Input is first gob encoded, then something terminal/file friendly.  Decode in reverse.  Tempting to use base64 (it's concise) but that encoder buffers, which leads to unwanted delays when piping output from one process to another.
	//b64Decoder := base64.NewDecoder(base64.StdEncoding, in)
	outerDecoder := hex.NewDecoder(in)
	gobDecoder := gob.NewDecoder(outerDecoder)

	// decode until EOF
	count := 0
	for {
		// Decode into interface.
		var tx data.Transaction

		err := gobDecoder.Decode(&tx) // Decode into *pointer* to interface
		if err != nil {
			if err != io.EOF || count == 0 {
				q.Q(err)
				log.Println(err)
			}
			return err
		}
		count++
		txs <- tx
	}

}

func EncodeTransactions(out io.Writer, txs chan (data.Transaction)) error {
	// Register instances of what we accept.
	register()

	//b64Encoder := base64.NewEncoder(base64.StdEncoding, out)
	//defer b64Encoder.Close()
	outerEncoder := hex.NewEncoder(out)
	gobEncoder := gob.NewEncoder(outerEncoder)

	var err error
	count := 0
	for tx := range txs {
		count++
		err = gobEncoder.Encode(&tx) // Encode a *pointer* to the interface
		if err != nil {
			log.Println(err)
			return err
		}
	}

	return nil
}
