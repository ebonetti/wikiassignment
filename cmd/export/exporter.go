package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"

	"github.com/RoaringBitmap/roaring"
	"github.com/ebonetti/overpedia/nationalization"
	"github.com/ebonetti/wikiassignment"
	"github.com/ebonetti/wikidump"
)

var lang string

func init() {
	flag.StringVar(&lang, "lang", "it", "Wikipedia nationalization to parse (en,it).")
}

const tmpDir = "."

func main() {
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

	page2Topic, namespaces, err := wikiassignment.From(context.Background(), tmpDir, dumps, topic2Categories, nationalization.Filter)
	if err != nil {
		return
	}

	articles := roaring.BitmapOf(namespaces.Articles...)
	for pageID, TopicID := range page2Topic {
		_, ok := nationalization.Article2Topic[pageID]
		switch {
		case ok:
			//Page already assigned by custom assignment, do nothing
		case !articles.Contains(pageID):
			//Page is not an article, do nothing
		default:
			nationalization.Article2Topic[pageID] = TopicID
		}
	}

	fmt.Println(nationalization.Article2Topic[pageID])
}

type readClose struct {
	io.Reader
	Closer func() error
}

func (r readClose) Close() error {
	return r.Closer()
}
