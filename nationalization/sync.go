package nationalization

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/ebonetti/absorbingmarkovchain"

	"github.com/pkg/errors"
)

//Sync returns the synced nationalizations derived from input and Wikipedia langLinks.
func Sync(nn ...Nationalization) (lang2Nationalization map[string]Nationalization) {
	if len(nn) == 0 {
		return
	}

	const cacheFilename = ".query2PageCache.json"
	query2PageCache := map[string]mayMissingPage{}
	readJSON(cacheFilename, &query2PageCache)
	defer writeJSON(cacheFilename, query2PageCache)

	return langLinksGraphFrom(nn, query2PageCache).resync().nationalizations()
}

func langLinksGraphFrom(nationalizations []Nationalization, query2PageCache map[string]mayMissingPage) (g langLinksGraph) {
	g = langLinksGraph{map[i18lPage]i18lPage{}, query2PageCache}
	for _, n := range nationalizations {
		for _, t := range n.Topics {
			i18lTopic := i18lPage{dummyLang, t.Page}
			for _, c := range t.Categories {
				pp := g.DFS([]i18lPage{}, i18lPage{n.Language, c}, 0) //sanitize input
				if len(pp) == 0 {
					continue
				}
				g.Assignments[pp[0]] = i18lTopic
			}
		}
		for _, filter := range n.Filters {
			pp := g.DFS([]i18lPage{}, i18lPage{n.Language, filter}, 0) //sanitize input
			if len(pp) == 0 {
				continue
			}
			g.Assignments[pp[0]] = dummyI18lFilter
		}
	}
	return
}

type langLinksGraph struct {
	Assignments     map[i18lPage]i18lPage
	Query2PageCache map[string]mayMissingPage
}

func (g langLinksGraph) nationalizations() (lang2Nationalization map[string]Nationalization) {
	lang2Nationalization = map[string]Nationalization{}
	for from, to := range g.Assignments {
		n, ok := lang2Nationalization[from.Lang]
		if !ok {
			n = newNationalization(from.Lang)
		}

		if to == dummyI18lFilter {
			n.Filters = pageAdd(n.Filters, from.Page)
		} else {
			position := sort.Search(len(n.Topics), func(i int) bool { return n.Topics[i].ID >= to.ID })
			n.Topics[position].Categories = pageAdd(n.Topics[position].Categories, from.Page)
		}

		lang2Nationalization[from.Lang] = n
	}
	return
}

func (g langLinksGraph) resync() langLinksGraph {
	nodes := []i18lPage{}
	for p := range g.Assignments {
		nodes = g.DFS(nodes, p, -1)
	}
	t := translator(nodes)

	isoG := mapGraph{}
	nodesIDs, absorbingNodesIDs := roaring.New(), roaring.New()
	for _, from := range nodes {
		fromID := t.ToID(from)
		nodesIDs.Add(fromID)
		for _, to := range g.DFS([]i18lPage{}, from, 1) {
			toID := t.ToID(to)
			isoG.Add(fromID, toID)
			if to.Lang != dummyLang {
				isoG.Add(toID, fromID)
			} else {
				absorbingNodesIDs.Add(toID)
			}
		}
	}

	edges := func(from uint32) []uint32 { return isoG[from] }
	ID2distance := isoG.Distances(absorbingNodesIDs)
	weighter := func(from, to uint32) (weight float64, err error) {
		d := ID2distance[to] + 1 - ID2distance[from] //d is non negative; weight=1 iff d=0
		return 1 / float64(1+10*d), nil
	}
	assigner, err := absorbingmarkovchain.New(".", nodesIDs, absorbingNodesIDs, edges, weighter).AbsorptionAssignments(context.Background())
	if err != nil {
		panic(err)
	}

	for transient, absorbing := range assigner {
		g.Assignments[t.ToPage(transient)] = t.ToPage(absorbing)
	}

	return g
}

func (g langLinksGraph) DFS(visited []i18lPage, p i18lPage, depth int) []i18lPage {
	if exist, _ := i18lPageExist(visited, p); exist {
		return visited
	}

	if p.Lang == dummyLang {
		visited, _ = i18lPageAdd(visited, p)
		return visited
	}

	const categoryNamespace = 14
	page := g.LangLinks(p, categoryNamespace)
	if page.Missing {
		return visited
	}

	p = i18lPage{p.Lang, Page{ID: page.ID, Title: page.Title}}
	visited, existed := i18lPageAdd(visited, p)
	if existed || depth == 0 {
		return visited
	}

	for _, langLink := range page.LangLinks {
		visited = g.DFS(visited, i18lPage{langLink.Lang, Page{Title: langLink.Title}}, depth-1)
	}

	if p, ok := g.Assignments[p]; ok {
		visited = g.DFS(visited, p, depth-1)
	}

	return visited
}

func (g langLinksGraph) LangLinks(p i18lPage, namespace int) (page mayMissingPage) {
	query := queryFrom("https://%v.wikipedia.org/w/api.php?action=query&prop=langlinks&lllimit=500&redirects&format=json&formatversion=2&titles=%v", p.Lang, []interface{}{p.Title})
	page, ok := g.Query2PageCache[query]
	if ok {
		return
	}

	pd, err := pagesDataFrom(query)
	for t := time.Second; err != nil && t < time.Minute; t *= 2 { //exponential backoff
		time.Sleep(t)
		pd, err = pagesDataFrom(query)
	}

	page.Title = p.Title
	page.Missing = true
	URL := fmt.Sprintf("https://%v.wikipedia.org/wiki/%v", p.Lang, strings.Replace(p.Title, " ", "_", -1))
	switch {
	case err != nil:
		fmt.Printf("Discarded page %v : %v\n", URL, err)
	case len(pd.Query.Pages) == 0:
		fmt.Printf("Discarded page %v : query %v returns an empty page list\n", URL, query)
	case pd.Query.Pages[0].Missing:
		fmt.Printf("Discarded page %v : not found\n", URL)
	case pd.Query.Pages[0].Namespace != namespace:
		fmt.Printf("Discarded page %v : expected namespace %v, found %v\n", URL, namespace, pd.Query.Pages[0].Namespace)
	default:
		page = pd.Query.Pages[0]
	}
	g.Query2PageCache[query] = page
	return
}

func queryFrom(base string, lang string, infos []interface{}) (query string) {
	infoString := make([]string, len(infos))
	for i, info := range infos {
		infoString[i] = fmt.Sprint(info)
	}
	return fmt.Sprintf(base, lang, url.QueryEscape(strings.Join(infoString, "|")))
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
	LangLinks []langLink
}

type langLink struct {
	Lang, Title string
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

const dummyLang = "..."

var dummyI18lFilter = i18lPage{dummyLang, Page{^uint32(0), "Filter"}}

func pageAdd(a []Page, x Page) []Page {
	position := sort.Search(len(a), func(i int) bool { return a[i].Title >= x.Title })
	if exist := position < len(a) && a[position].Title == x.Title; exist {
		return a
	}

	a = append(a, Page{})
	copy(a[position+1:], a[position:])
	a[position] = x

	return a
}

func newNationalization(lang string) Nationalization {
	n, _ := New("en")
	n.Language = lang
	for i := range n.Topics {
		n.Topics[i].Categories = nil
	}
	n.Filters = nil
	return n
}

func writeJSON(filename string, v interface{}) error {
	JSONData, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, JSONData, os.ModePerm)
}

func readJSON(filename string, v interface{}) error {
	JSONData, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(JSONData, v)
}
