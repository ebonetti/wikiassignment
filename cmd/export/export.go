package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/RoaringBitmap/roaring"
	"github.com/ebonetti/absorbingmarkovchain"
	"github.com/ebonetti/overpedia/nationalization"
	"github.com/ebonetti/wikiassignment"
	"github.com/ebonetti/wikidump"
)

var lang string

func init() {
	flag.StringVar(&lang, "lang", "it", "Wikipedia nationalization to parse (en,it).")
}

func main() {
	tmpDir, err := ioutil.TempDir(".", ".")
	if err != nil {
		log.Fatalf("%+v", err)
	}
	defer os.Remove(tmpDir)

	nationalization, err := nationalization.New(lang)
	if err != nil {
		log.Fatalf("%+v", err)
	}

	latestDump, err := wikidump.Latest(tmpDir, lang, "pagetable", "categorylinkstable", "pagelinkstable")
	if err != nil {
		log.Fatalf("%+v", err)
	}

	dumps := func(name string) (r io.ReadCloser, err error) {
		rawReader, err := latestDump.Open(name)(context.Background())
		if err != nil {
			return
		}
		r = readClose{wikidump.SQL2CSV(rawReader), rawReader.Close}
		return
	}

	topic2Categories := map[uint32][]uint32{}
	for _, t := range nationalization.Topics {
		topic2Categories[t.ID] = t.Categories
	}
	for pageID, TopicID := range nationalization.Article2Topic {
		topic2Categories[TopicID] = append(topic2Categories[TopicID], pageID)
	}

	amcData := _amcData{}
	weighter, err := chainFrom(context.Background(), tmpDir, wikiassignment.SemanticGraphSources{dumps, topic2Categories, nationalization.Filter}, &amcData).AbsorptionProbabilities(context.Background())
	switch {
	case amcData.err != nil:
		weighter, err = nil, amcData.err
		log.Fatalf("%+v", err)
	case err != nil:
		log.Fatalf("%+v", err)
	}

	topics := amcData.namespace2Ids[wikiassignment.TopicNamespaceID].ToArray()
	categories := amcData.namespace2Ids[wikiassignment.CategoryNamespaceID].ToArray()
	articles := amcData.namespace2Ids[wikiassignment.ArticleNamespaceID].ToArray()

	IDPartition := map[string][]uint32{"topics": topics, "categories": categories, "articles": articles}
	esport2JSON("IDPartition.json", IDPartition) ///////////////////////////////////////////////////

	//Export to csv absorption probabilities
	f, err := os.Create("absorptionprobabilities.csv")
	if err != nil {
		log.Fatalf("%+v", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	headers := []string{"PageID"}
	for i, topicID := range topics {
		t := nationalization.Topics[i]
		if t.ID != topicID {
			log.Fatal("Invalid ordering of Topics")
		}
		headers = append(headers, strings.Split(t.Title, " ")[0])
	}
	fmt.Fprintln(w, headers)

	for _, articleID := range articles {
		data := []interface{}{articleID}
		for _, topicID := range topics {
			weight, err := weighter(articleID, topicID)
			if err != nil {
				log.Fatalf("%+v", err)
			}
			data = append(data, weight)
		}
		fmt.Fprintln(w, data)
	}
}

type _amcData struct {
	err           error
	namespace2Ids map[int]*roaring.Bitmap
}

func chainFrom(ctx context.Context, tmpDir string, d wikiassignment.SemanticGraphSources, amcd *_amcData) *absorbingmarkovchain.AbsorbingMarkovChain {
	g, IDs2CatDistance, namespace2Ids, err := d.Build(ctx)

	esport2JSON("semanticgraph.json", g) ///////////////////////////////////////////////////

	if err != nil {
		amcd.err = err
		return nil
	}
	amcd.namespace2Ids = namespace2Ids

	articlesIds := namespace2Ids[wikiassignment.ArticleNamespaceID]
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
	absorbingNodes := namespace2Ids[wikiassignment.TopicNamespaceID]
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

func esport2JSON(filename string, v interface{}) {
	w, err := os.Create(filename)
	if err != nil {
		log.Fatalf("%+v", err)
	}
	defer w.Close()

	jsonWriter := json.NewEncoder(w)

	err = jsonWriter.Encode(v)
	if err != nil {
		log.Fatalf("%+v", err)
	}
}
