// Package wikiassignment provides utility functions for automatically assigning wikipedia pages to topics.
package wikiassignment

import (
	"context"
	"io"

	"github.com/RoaringBitmap/roaring"
	"github.com/ebonetti/absorbingmarkovchain"
)

//From transforms the sematic graph from the input into a page-topic assignment
func From(ctx context.Context, dumps func(string) (io.ReadCloser, error), topic2Categories map[uint32][]uint32, filters []Filter) (page2Topic map[uint32]uint32, namespaces struct{ Topics, Categories, Articles []uint32 }, err error) {
	amcData := amcData{}
	page2Topic, err = chainFrom(ctx, semanticGraph{dumps, topic2Categories, filters}, &amcData).AbsorptionAssignments(ctx)
	switch {
	case amcData.err != nil:
		page2Topic, err = nil, amcData.err
		return
	case err != nil:
		return
	}

	for _, t := range namespaces.Topics {
		page2Topic[t] = t
	}

	namespaces.Topics = amcData.namespace2Ids[topicNamespaceID].ToArray()
	namespaces.Categories = amcData.namespace2Ids[categoryNamespaceID].ToArray()
	namespaces.Articles = amcData.namespace2Ids[articleNamespaceID].ToArray()

	return
}

//Filter represents a filter to be applied to the semantic graph before the transformation into assignment
type Filter struct {
	IsWhitelist bool
	Parents     []uint32
	Dept        int
}

type amcData struct {
	err           error
	namespace2Ids map[int]*roaring.Bitmap
}

func chainFrom(ctx context.Context, d semanticGraph, amcd *amcData) *absorbingmarkovchain.AbsorbingMarkovChain {
	g, IDs2CatDistance, namespace2Ids, err := d.Build(ctx)

	if err != nil {
		amcd.err = err
		return nil
	}
	amcd.namespace2Ids = namespace2Ids

	articlesIds := namespace2Ids[articleNamespaceID]
	weighter := func(from, to uint32) (weight float64, err error) { //amc weigherweight<=1
		switch {
		case articlesIds.Contains(to): //penalized link (this link was added by pagelink)
			weight = 1.0 / 200
		default: //valuable link (this link was added by categorylink)
			d := IDs2CatDistance[to] + 1 - IDs2CatDistance[from] //d is non negative; weight=1 iff d=0
			weight = 1 / float64(1+10*d)
		}
		return
	}

	nodes := roaring.NewBitmap()
	for _, ids := range namespace2Ids {
		nodes.Or(ids)
	}
	absorbingNodes := namespace2Ids[articleNamespaceID]
	edges := func(from uint32) []uint32 { return g[from] }

	return absorbingmarkovchain.New(nodes, absorbingNodes, edges, weighter)
}
