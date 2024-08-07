package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/filecoin-project/go-address"
	hamtv3 "github.com/filecoin-project/go-hamt-ipld/v3"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-cid"
	ipldcbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/urfave/cli/v2"
	"github.com/willscott/dc-accounting-utils/lib"
	dctypes "github.com/willscott/dc-accounting-utils/types"
)

var verifRegAddr address.Address

func main() {
	app := &cli.App{
		Name:  "dcacct",
		Usage: "Compute accounting state about datacap",
		Action: func(c *cli.Context) error {
			api, store, err := lib.GetBlockstore(c)
			if err != nil {
				return err
			}

			verifRegAddr, _ = address.NewFromString("f06")

			addr := c.String("account")
			baddr, err := address.NewFromString(addr)
			if err != nil {
				return err
			}

			bal, err := GetBalanceAtEpoch(c.Context, types.EmptyTSK, store, api, baddr)
			if err != nil {
				return err
			}
			fmt.Printf("balance: %d", bal)
			return nil
		},
		Flags: []cli.Flag{
			&lib.ApiFlag,
			&cli.StringFlag{
				Name:  "account",
				Usage: "the 'f0xxxx' address to get a report for",
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func GetBalanceAtEpoch(ctx context.Context, tsk types.TipSetKey, store blockstore.Blockstore, a api.FullNode, acct address.Address) (big.Int, error) {
	verifReg, err := a.StateGetActor(ctx, verifRegAddr, tsk)
	if err != nil {
		return big.Zero(), err
	}
	sr, err := store.Get(ctx, verifReg.Head)
	if err != nil {
		return big.Zero(), err
	}

	l := basicnode.Prototype.List.NewBuilder()
	dagcbor.Decode(l, bytes.NewBuffer(sr.RawData()))
	list := l.Build()
	// the hamt is rooted as the second item.
	verifierRoot, _ := list.LookupByIndex(1)
	vRC, _ := verifierRoot.AsLink()
	_, vRCid, _ := cid.CidFromBytes([]byte(vRC.Binary()))
	// now do a hamt lookup
	cStore := ipldcbor.NewCborStore(store)
	hamt, err := hamtv3.LoadNode(ctx, cStore, vRCid, hamtv3.UseTreeBitWidth(5), hamtv3.UseHashFunction(func(input []byte) []byte {
		res := sha256.Sum256(input)
		return res[:]
	}))
	if err != nil {
		return big.Zero(), err
	}

	var out dctypes.CborByteArray
	found, err := hamt.Find(ctx, string(acct.Bytes()), &out)
	if err != nil {
		return big.Zero(), err
	}
	if !found {
		return big.Zero(), errors.New("acct not found")
	}
	bi, _ := big.FromBytes(out)

	return bi, nil
}

/*
func unCbor(raw []byte, prot schema.TypedPrototype, pt interface{}) error {
	builder := prot.NewBuilder()
	if err := dagcbor.Decode(builder, bytes.NewBuffer(raw)); err != nil {
		return err
	}
	built := builder.Build()
	ptp, ok := pt.(*interface{})
	if !ok {
		return errors.New("invalid ptr")
	}
	*ptp = bindnode.Unwrap(built)
	return nil
}
*/
