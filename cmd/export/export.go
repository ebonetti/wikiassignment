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
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ebonetti/wikipage"

	"github.com/pkg/errors"

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

	esport2JSON("semanticgraph.json", g) ///////////////////////////////////////////////////

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
	esport2JSON("partition.json", IDPartition)
}

func (data data) EsportAbsorptionProbabilities() {
	const filename = "absorptionprobabilities.csv"
	fmt.Println("Exporting", filename)
	defer fmt.Println("Done")
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
	fmt.Println("Exporting", filename)
	defer fmt.Println("Done")
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

func esport2JSON(filename string, v interface{}) {
	fmt.Println("Exporting", filename)
	defer fmt.Println("Done")
	w, err := os.Create(filename)
	if err != nil {
		log.Panicf("%v", err)
	}
	defer w.Close()

	jsonWriter := json.NewEncoder(w)

	err = jsonWriter.Encode(v)
	if err != nil {
		log.Panicf("%v", err)
	}
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

	bytes, err := ioutil.ReadFile(lang)
	if err != nil {
		log.Fatalf("Custom %s file not found", lang)
		return
	}

	if err = json.Unmarshal(bytes, &n); err != nil {
		log.Fatalf("Error while parsing %s: %v", lang, err)
	}
	lang = n.Language

	sanitizePageIDs(&n)
	return
}

func sanitizePageIDs(n *nationalization.Nationalization) {
	for i, t := range n.Topics {
		for j, c := range t.Categories {
			n.Topics[i].Categories[j].ID = title2ID(c.Title)
		}
	}
	for i, f := range n.Filters {
		n.Filters[i].ID = title2ID(f.Title)
	}
}

func title2ID(title string) uint32 {
	const base = "https://%v.wikipedia.org/w/api.php?action=query&redirects&format=json&formatversion=2&titles=%v"
	page := get(queryFrom(base, lang, title))
	if page.Missing {
		log.Fatal("Not found ", title)
	}
	return page.ID
}

func queryFrom(base string, lang string, infos ...interface{}) (query string) {
	infoString := make([]string, len(infos))
	for i, info := range infos {
		infoString[i] = fmt.Sprint(info)
	}
	return fmt.Sprintf(base, lang, url.QueryEscape(strings.Join(infoString, "|")))
}

func get(query string) (page mayMissingPage) {
	for t := time.Second; t < time.Minute; t *= 2 { //exponential backoff
		pd, err := pagesDataFrom(query)
		switch {
		case err != nil:
			page.Missing = true
		case len(pd.Query.Pages) == 0:
			page.Missing = true
			return
		default:
			page = pd.Query.Pages[0]
			return
		}
		fmt.Println(err)
		time.Sleep(t)
	}

	return
}

type pagesData struct {
	Batchcomplete interface{}
	Warnings      interface{}
	Query         struct {
		Pages []mayMissingPage
	}
}

type mayMissingPage struct {
	ID        uint32 `json:"pageid"`
	Title     string
	Namespace int `json:"ns"`
	Missing   bool
}

var client = &http.Client{Timeout: time.Minute}

func pagesDataFrom(query string) (pd pagesData, err error) {
	fail := func(e error) (pagesData, error) {
		pd, err = pagesData{}, errors.Wrapf(e, "Error with the following query: %v", query)
		return pd, err
	}

	resp, err := client.Get(query)
	if err != nil {
		return fail(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fail(err)
	}

	err = json.Unmarshal(body, &pd)
	if err != nil {
		return fail(err)
	}

	if pd.Batchcomplete == nil {
		return fail(errors.Errorf("Incomplete batch with the following query: %v", query))
	}

	if pd.Warnings != nil {
		return fail(errors.Errorf("Warnings - %v - with the following query: %v", pd.Warnings, query))
	}

	return
}
