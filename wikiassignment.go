// Package wikiassignment provides utility functions for automatically assigning wikipedia pages to topics.
package wikiassignment

import (
	"context"
	"io"

	"github.com/ebonetti/overpedia/nationalization"
	"github.com/ebonetti/wikidump"

	"github.com/RoaringBitmap/roaring"
	"github.com/ebonetti/absorbingmarkovchain"
)

//From transforms the sematic graph from the input into a page-topic assignment
func From(ctx context.Context, tmpDir, lang string) (page2Topic map[uint32]uint32, namespaces struct{ Topics, Categories, Articles []uint32 }, err error) {
	latestDump, err := wikidump.Latest(tmpDir, lang, "metahistory7zdump", "pagetable", "redirecttable", "categorylinkstable", "pagelinkstable")
	if err != nil {
		return
	}
	dumps := func(name string) (r io.ReadCloser, err error) {
		rawReader, err := latestDump.Open(name)(ctx)
		if err != nil {
			return
		}
		r = readClose{wikidump.SQL2CSV(rawReader), rawReader.Close}
		return
	}

	nationalization, err := nationalization.New(lang)
	if err != nil {
		return
	}
	topicAssignments := map[uint32][]uint32{}
	for _, t := range nationalization.Topics {
		for _, p := range append(t.Categories, t.Articles...) {
			topicAssignments[t.ID] = append(topicAssignments[t.ID], p.ID)
		}
	}
	filters := []uint32{}
	for _, p := range nationalization.Filters {
		filters = append(filters, p.ID)
	}

	amcData := amcData{}
	page2Topic, err = chainFrom(ctx, tmpDir, SemanticGraphSources{dumps, topicAssignments, []Filter{{false, filters, 1}}}, &amcData).AbsorptionAssignments(ctx)
	switch {
	case amcData.err != nil:
		page2Topic, err = nil, amcData.err
		return
	case err != nil:
		return
	}

	namespaces.Topics = amcData.namespace2Ids[TopicNamespaceID].ToArray()
	namespaces.Categories = amcData.namespace2Ids[CategoryNamespaceID].ToArray()
	namespaces.Articles = amcData.namespace2Ids[ArticleNamespaceID].ToArray()

	for _, t := range namespaces.Topics {
		page2Topic[t] = t
	}

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

func chainFrom(ctx context.Context, tmpDir string, d SemanticGraphSources, amcd *amcData) *absorbingmarkovchain.AbsorbingMarkovChain {
	g, IDs2CatDistance, namespace2Ids, err := d.Build(ctx)

	if err != nil {
		amcd.err = err
		return nil
	}
	amcd.namespace2Ids = namespace2Ids

	articlesIds := namespace2Ids[ArticleNamespaceID]
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
	absorbingNodes := namespace2Ids[TopicNamespaceID]
	edges := func(from uint32) []uint32 { return g[from] }

	return absorbingmarkovchain.New(tmpDir, nodes, absorbingNodes, edges, weighter)
}

type readClose struct {
	io.Reader
	Closer func() error
}

func (r readClose) Close() error {
	return r.Closer()
}
