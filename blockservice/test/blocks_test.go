package bstest

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/ipfs/boxo/blockservice"
	blockstore "github.com/ipfs/boxo/blockstore"
	offline "github.com/ipfs/boxo/exchange/offline"
	u "github.com/ipfs/boxo/util"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
)

func newObject(data []byte) blocks.Block {
	return blocks.NewBlock(data)
}

func TestBlocks(t *testing.T) {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bs := New(bstore, offline.Exchange(bstore))
	defer bs.Close()

	o := newObject([]byte("beep boop"))
	h := cid.NewCidV0(u.Hash([]byte("beep boop")))
	if !o.Cid().Equals(h) {
		t.Error("Block key and data multihash key not equal")
	}

	err := bs.AddBlock(context.Background(), o)
	if err != nil {
		t.Error("failed to add block to BlockService", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	b2, err := bs.GetBlock(ctx, o.Cid())
	if err != nil {
		t.Error("failed to retrieve block from BlockService", err)
		return
	}

	if !o.Cid().Equals(b2.Cid()) {
		t.Error("Block keys not equal.")
	}

	if !bytes.Equal(o.RawData(), b2.RawData()) {
		t.Error("Block data is not equal.")
	}
}

func makeObjects(n int) []blocks.Block {
	var out []blocks.Block
	for i := 0; i < n; i++ {
		out = append(out, newObject([]byte(fmt.Sprintf("object %d", i))))
	}
	return out
}

func TestGetBlocksSequential(t *testing.T) {
	servs := Mocks(4)
	for _, s := range servs {
		defer s.Close()
	}
	objs := makeObjects(50)

	var cids []cid.Cid
	for _, o := range objs {
		cids = append(cids, o.Cid())
		err := servs[0].AddBlock(context.Background(), o)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Log("one instance at a time, get blocks concurrently")

	for i := 1; i < len(servs); i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
		defer cancel()
		out := servs[i].GetBlocks(ctx, cids)
		gotten := make(map[string]blocks.Block)
		for blk := range out {
			if _, ok := gotten[blk.Cid().KeyString()]; ok {
				t.Fatal("Got duplicate block!")
			}
			gotten[blk.Cid().KeyString()] = blk
		}
		if len(gotten) != len(objs) {
			t.Fatalf("Didnt get enough blocks back: %d/%d", len(gotten), len(objs))
		}
	}
}
