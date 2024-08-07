package lib

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/blockstore"
	lru "github.com/hashicorp/golang-lru"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ibs "github.com/ipfs/go-ipfs-blockstore"
)

type proxyingBlockstore struct {
	ctx context.Context
	api api.FullNode

	cache *lru.ARCCache
}

func (pb *proxyingBlockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	if block, ok := pb.cache.Get(cid); ok {
		return blocks.NewBlockWithCid(block.([]byte), cid)
	}

	// fmt.Printf("fetching cid via rpc: %v\n", cid)
	item, err := pb.api.ChainReadObj(ctx, cid)
	if err != nil {
		return nil, err
	}
	block, err := blocks.NewBlockWithCid(item, cid)
	if err != nil {
		return nil, err
	}

	pb.cache.Add(cid, item)

	return block, nil
}

func (pb *proxyingBlockstore) Put(ctx context.Context, b blocks.Block) error {
	return nil
}

func (pb *proxyingBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return nil, errors.New("not supported")
}

func (pb *proxyingBlockstore) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	return nil
}

func (pb *proxyingBlockstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	_, err := pb.Get(ctx, c)
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (pb *proxyingBlockstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	i, err := pb.Get(ctx, c)
	if err != nil {
		return 0, err
	}
	return len(i.RawData()), nil
}

func (pb *proxyingBlockstore) PutMany(context.Context, []blocks.Block) error {
	return nil
}

func (pb *proxyingBlockstore) HashOnRead(enabled bool) {
	// ignore
}

// StoreFor creates a blockstore proxying access to a lotus node.
func StoreFor(ctx context.Context, client api.FullNode) blockstore.Blockstore {
	cs := 32 * 1024
	rcs := os.Getenv("BLOCK_CACHE_SIZE")
	if rcs != "" {
		var err error
		cs, err = strconv.Atoi(rcs)
		if err != nil {
			fmt.Printf("Failed to parse cache size: %v\n", err)
			cs = 32 * 1024
		}
	}

	cache, err := lru.NewARC(cs)
	if err != nil {
		return nil
	}

	bs := &proxyingBlockstore{
		ctx:   ctx,
		api:   client,
		cache: cache,
	}
	return &LotusBS{bs}
}

type LotusBS struct {
	ibs.Blockstore
}

func (*LotusBS) DeleteMany(context.Context, []cid.Cid) error {
	return errors.New("not supported")
}

func (*LotusBS) Flush(context.Context) error {
	return errors.New("not supported")
}

func (l *LotusBS) View(ctx context.Context, c cid.Cid, f func([]byte) error) error {
	b, err := l.Get(ctx, c)
	if err != nil {
		return err
	}
	return f(b.RawData())
}
