package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ebonetti/wikipage"

	json "github.com/json-iterator/go"

	"github.com/RoaringBitmap/roaring"
	"github.com/ebonetti/absorbingmarkovchain"
	"github.com/ebonetti/wikiassignment"
	"github.com/ebonetti/wikiassignment/nationalization"
	"github.com/ebonetti/wikidump"
)

var lang, date, tmpDir string

func init() {
	flag.StringVar(&lang, "lang", "it", "Wikipedia nationalization to parse.")
	flag.StringVar(&date, "date", "latest", "Wikipedia dump date in the format AAAAMMDD.")
}

func main() {
	flag.Parse()
	tmpDir, err := ioutil.TempDir(".", ".")
	if err != nil {
		log.Panicf("%v", err)
	}
	defer os.Remove(tmpDir)

	os.Rename("pages.csv", "oldpages.csv")

	data := data{Nationalization: fetchNationalization()}
	ctx := context.Background()
	weighter, err := data.Chain(ctx).AbsorptionProbabilities(ctx)
	switch {
	case data.Err != nil:
		weighter, err = nil, data.Err
		log.Fatalf("%+v", err)
	case err != nil:
		log.Fatalf("%+v", err)
	}

	data.Weighter = func(pageID, topicID uint32) (weight float64) {
		weight, err := weighter(pageID, topicID)
		if err != nil {
			log.Fatalf("%+v", err)
		}
		return
	}

	data.EsportPartition()
	data.EsportAbsorptionProbabilities()
	data.EsportPages()
}

type data struct {
	Err error
	nationalization.Nationalization
	Dumps         func(name string) (r io.ReadCloser, err error)
	Namespace2Ids map[int]*roaring.Bitmap
	Weighter      func(pageID, topicID uint32) (weight float64)
}

func (data *data) Chain(ctx context.Context) *absorbingmarkovchain.AbsorbingMarkovChain {
	wikimediaDumps, err := fetchDump()
	if err != nil {
		log.Fatalf("%+v", err)
	}

	data.Dumps = func(name string) (r io.ReadCloser, err error) {
		rawReader, err := wikimediaDumps.Open(name)(context.Background())
		if err != nil {
			return
		}
		r = readClose{wikidump.SQL2CSV(rawReader), rawReader.Close}
		return
	}

	topic2Categories := map[uint32][]uint32{}
	for _, t := range data.Nationalization.Topics {
		for _, page := range t.Categories {
			topic2Categories[t.ID] = append(topic2Categories[t.ID], page.ID)
		}
	}

	filters := []uint32{}
	for _, p := range data.Nationalization.Filters {
		filters = append(filters, p.ID)
	}

	g, IDs2CatDistance, namespace2Ids, err := wikiassignment.SemanticGraphSources{data.Dumps, topic2Categories, []wikiassignment.Filter{{false, filters, 1}}}.Build(ctx)

	if err := writeJSON("semanticgraph.json", g); err != nil {
		log.Panicf("%v", err)
	}

	if err != nil {
		data.Err = err
		return nil
	}
	data.Namespace2Ids = namespace2Ids

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

func (data data) EsportPartition() {
	topics := data.Namespace2Ids[wikiassignment.TopicNamespaceID].ToArray()
	categories := data.Namespace2Ids[wikiassignment.CategoryNamespaceID].ToArray()
	articles := data.Namespace2Ids[wikiassignment.ArticleNamespaceID].ToArray()

	IDPartition := map[string][]uint32{"topics": topics, "categories": categories, "articles": articles}

	if err := writeJSON("partition.json", IDPartition); err != nil {
		log.Panicf("%v", err)
	}
}

func (data data) EsportAbsorptionProbabilities() {
	const filename = "absorptionprobabilities.csv"
	f, err := os.Create(filename)
	if err != nil {
		log.Panicf("%v", err)
	}
	defer f.Close()

	bf := bufio.NewWriter(f)
	defer bf.Flush()

	w := csv.NewWriter(bf)
	defer w.Flush()

	write := func(ss []string) {
		if err := w.Write(ss); err != nil {
			log.Panicf("%v", err)
		}
	}

	headers := []string{"PageID"}
	for _, t := range data.Nationalization.Topics {
		headers = append(headers, strings.Split(t.Title, " ")[0])
	}
	write(headers)

	for _, pageID := range roaring.Or(data.Namespace2Ids[wikiassignment.CategoryNamespaceID], data.Namespace2Ids[wikiassignment.ArticleNamespaceID]).ToArray() {
		row := []string{fmt.Sprint(pageID)}
		for _, t := range data.Nationalization.Topics {
			row = append(row, fmt.Sprint(data.Weighter(pageID, t.ID)))
		}
		write(row)
	}
}

func (data data) EsportPages() {
	const filename = "pages.csv"
	f, err := os.Create(filename)
	if err != nil {
		log.Panicf("%v", err)
	}
	defer f.Close()

	bf := bufio.NewWriter(f)
	defer bf.Flush()

	w := csv.NewWriter(bf)
	defer w.Flush()

	write := func(ss ...string) {
		if err := w.Write(ss); err != nil {
			log.Panicf("%v", err)
		}
	}

	write("id", "title", "abstract", "topicid")

	writeRow := func(ID uint32, title, abstract string, topicID uint32) {
		write(fmt.Sprint(ID), title, abstract, fmt.Sprint(topicID))
	}

	for _, t := range data.Nationalization.Topics {
		writeRow(t.ID, t.Title, "", 0)
	}

	for page := range data.Pages() {
		perm := rand.Perm(len(data.Nationalization.Topics))
		bestTopicID, bestw := uint32(0), -1.0
		for _, p := range perm {
			topicID := data.Nationalization.Topics[p].ID
			w := data.Weighter(page.ID, topicID)
			if w > bestw {
				bestTopicID = topicID
				bestw = w
			}
		}
		writeRow(page.ID, page.Title, page.Abstract, bestTopicID)
	}
}

func (data data) Pages() <-chan wikipage.WikiPage {
	results := make(chan wikipage.WikiPage, 20)
	go func() {
		defer close(results)
		pageIDs := data.Namespace2Ids[wikiassignment.ArticleNamespaceID]

		var r io.ReadCloser
		r, err := os.Open("oldpages.csv")
		csvReader := csv.NewReader(r)
		IDPosition, titlePosition, abstractPosition := 0, 1, 2
		if err != nil {
			r, err = data.Dumps("pagetable")
			if err != nil {
				log.Panicf("%v", err)
			}
			csvReader = csv.NewReader(r)
			IDPosition, titlePosition, abstractPosition = 0, 2, 13
		} else {
			csvReader.Read() //Discard header
		}

		defer r.Close()

		for {
			ss, err := csvReader.Read()
			ss = append(ss, "")
			ID, err1 := strconv.ParseUint(ss[IDPosition], 10, 32)
			uint32ID := uint32(ID)
			switch {
			case err == io.EOF:
				return
			case err != nil:
				log.Panicf("%v", err)
			case err1 != nil:
				log.Panicf("%v", err1)
			case !pageIDs.Contains(uint32ID):
				//pass
			default:
				results <- wikipage.WikiPage{uint32ID, ss[titlePosition], ss[abstractPosition]}
			}
		}
	}()

	return results
}

type readClose struct {
	io.Reader
	Closer func() error
}

func (r readClose) Close() error {
	return r.Closer()
}

func fetchDump() (wikimediaDumps wikidump.Wikidump, err error) {
	if date == "latest" {
		fmt.Printf("Using latest %v dump\n", lang)
		return wikidump.Latest(tmpDir, lang, "pagetable", "redirecttable", "categorylinkstable", "pagelinkstable", "metahistory7zdump")
	}

	var t time.Time
	t, err = time.Parse("20060102", date)
	if err != nil {
		return
	}

	fmt.Printf("Using %v dump dated %v\n", lang, t)

	return wikidump.From(tmpDir, lang, t)
}

func fetchNationalization() (n nationalization.Nationalization) {
	n, err := nationalization.New(lang)
	if err == nil {
		return
	}

	filename := lang
	if err := readJSON(filename, &n); err != nil {
		log.Fatalf("While reading %v, arised the follwing: %v", filename, err.Error())
	}
	lang = n.Language
	n = nationalization.Sync(n)[lang]
	writeJSON(filename, n)
	return
}

func writeJSON(filename string, v interface{}) (err error) {
	f, err := os.Create(filename)
	if err != nil {
		return
	}

	w := bufio.NewWriter(f)

	defer func() {
		wErr := w.Flush()
		fErr := f.Close()
		switch {
		case err != nil:
			//Do nothing
		case wErr != nil:
			err = wErr
		case fErr != nil:
			err = fErr
		}
	}()

	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}

func readJSON(filename string, v interface{}) (err error) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}

	r := bufio.NewReader(f)

	return json.NewDecoder(r).Decode(v)
}
