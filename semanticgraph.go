package wikiassignment

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/RoaringBitmap/roaring"
	"github.com/pkg/errors"
)

const (
	topicNamespaceID    = 6666
	categoryNamespaceID = 14
	articleNamespaceID  = 0
)

type semanticGraph struct {
	Dumps            func(string) (io.ReadCloser, error)
	TopicAssignments map[uint32][]uint32
	Filters          []Filter
}

func (p semanticGraph) Build(ctx context.Context) (g mapGraph, ids2CatDistance map[uint32]uint32, namespace2Ids map[int]*roaring.Bitmap, err error) {
	gl := &mapGraphLoader{mapGraph(map[uint32][]uint32{}), map[int]*roaring.Bitmap{}, nTitle2ID{map[string]uint32{}}, nil}
	g = gl.Edges
	namespace2Ids = gl.Namespace2IDs

	for _, ID := range []int{topicNamespaceID, categoryNamespaceID, articleNamespaceID} {
		namespace2Ids[ID] = roaring.NewBitmap()
	}

	topicIds := gl.Namespace2IDs[topicNamespaceID]
	gl.AddNodes(p.topicSource())
	gl.AddNodes(p.pageSource(ctx))
	gl.AddEdges(p.topiclinksSource(gl))
	gl.AddEdges(p.categorylinksSource(ctx, gl))

	nodes := g.Nodes()
	gl.Filter(p.Filters...)
	unwantedNodes := roaring.AndNot(nodes, g.Nodes())
	for _, idsSet := range namespace2Ids { //filter unwanted nodes from namespaces
		idsSet.AndNot(unwantedNodes)
	}

	gl.Filter(Filter{true, topicIds.ToArray(), -1}) //filter unrelated nodes

	ids2CatDistance = g.Distances(topicIds)
	gl.AddEdges(p.pagelinksSource(ctx, gl))
	gl.Filter(Filter{true, topicIds.ToArray(), -1}) //filter unrelated nodes
	nodes = g.Nodes()
	for _, idsSet := range namespace2Ids {
		idsSet.And(nodes)
	}
	if gl.Err != nil {
		g, ids2CatDistance, namespace2Ids, err = nil, nil, nil, gl.Err
	}
	return
}

//Graph nodes: Topics
func (p semanticGraph) topicSource() pageSourcer {
	pp := make([]page, 0, len(p.TopicAssignments))
	for topicID := range p.TopicAssignments {
		pp = append(pp, page{topicID, topicNamespaceID, fmt.Sprint("Topic: ", topicID)})
	}
	return &slicePageSource{pp}
}

type slicePageSource struct {
	pp []page
}

func (s *slicePageSource) Next() (p page, err error) {
	if len(s.pp) == 0 {
		err = io.EOF
		return
	}
	p, s.pp = s.pp[0], s.pp[1:]
	return
}

func (s *slicePageSource) Close() error {
	s.pp = []page{}
	return nil
}

//Graph nodes: pages - articles & categories
func (p semanticGraph) pageSource(ctx context.Context) pageSourcer {
	isValid := map[int]bool{topicNamespaceID: true, categoryNamespaceID: true, articleNamespaceID: true}
	return &rPageSource{p.dumpIterator(ctx, "pagetable"), func(ns int) bool { return isValid[ns] }}
}

type rPageSource struct {
	rReadCloser
	IsValid func(ns int) (ok bool)
}

func (ps rPageSource) Next() (p page, err error) {
	p, err = ps.next()
	for err == nil && !ps.IsValid(p.Namespace) {
		p, err = ps.next()
	}
	return
}

func (ps rPageSource) next() (p page, err error) {
	var ss []string
LOOP:
	for {
		ss, err = ps.Read()
		switch {
		case err != nil:
			return
		case len(ss) < 6:
			err = errors.Errorf("Error: Invalid serialization expected at least 6, found %v", len(ss))
			return
		case ss[5] != "0": //so this is a Redirect
			//grab another page
		default:
			break LOOP
		}
	}

	ID, err := strconv.ParseUint(ss[0], 10, 32)
	if err != nil {
		err = errors.Wrapf(err, "Error: error while parsing ID from %v", ss[0])
		return
	}

	namespace, err := strconv.ParseInt(ss[1], 10, 32)
	if err != nil {
		err = errors.Wrapf(err, "Error: error while parsing namespace from %v", ss[1])
		return
	}

	return page{uint32(ID), int(namespace), ss[2]}, nil
}

//Graph edges: topic links with categories
func (p semanticGraph) topiclinksSource(gl *mapGraphLoader) edgeSourcer {
	pageIDs := roaring.Or(gl.Namespace2IDs[categoryNamespaceID], gl.Namespace2IDs[articleNamespaceID])
	ee := make([]edge, 0, 10*len(p.TopicAssignments))
	for topicID, pp := range p.TopicAssignments {
		for _, p := range pp {
			if !pageIDs.Contains(p) {
				return errorEdgeSource{errors.Errorf("Error: %v is not a page", p)}
			}
			ee = append(ee, edge{p, topicID})
		}
	}
	return &sliceEdgeSource{ee}
}

type sliceEdgeSource struct {
	ee []edge
}

func (s *sliceEdgeSource) Next() (e edge, err error) {
	if len(s.ee) == 0 {
		err = io.EOF
		return
	}
	e, s.ee = s.ee[0], s.ee[1:]
	return
}

func (s *sliceEdgeSource) Close() error {
	s.ee = []edge{}
	return nil
}

//Graph edges: categorylinks - links between categories and articles
func (p semanticGraph) categorylinksSource(ctx context.Context, gl *mapGraphLoader) edgeSourcer {
	articleIds := gl.Namespace2IDs[articleNamespaceID]
	categoryIds := gl.Namespace2IDs[categoryNamespaceID]
	ev := newValidator(edgeDomain{articleIds, categoryIds}, edgeDomain{categoryIds, categoryIds})
	scategoryNamespaceID := fmt.Sprint(categoryNamespaceID)
	extractFields := func(ss []string) (from, toNamespace, toTitle string, err error) {
		if len(ss) < 2 {
			err = errors.Errorf("Error: Invalid serialization expected at least 2, found %v in %v", len(ss), ss)
			return
		}
		return ss[0], scategoryNamespaceID, ss[1], nil
	}
	return rEdgeSource{p.dumpIterator(ctx, "categorylinkstable"), ev, gl.Title2ID, extractFields}
}

//Graph edges: pagelinks - links between articles
func (p semanticGraph) pagelinksSource(ctx context.Context, gl *mapGraphLoader) edgeSourcer {
	articleIds := gl.Namespace2IDs[articleNamespaceID]
	categorizedArticles := roaring.And(articleIds, gl.Edges.Nodes())
	uncategorizedArticles := roaring.AndNot(articleIds, categorizedArticles)
	ev := newValidator(edgeDomain{categorizedArticles, categorizedArticles}, edgeDomain{uncategorizedArticles, articleIds})
	extractFields := func(ss []string) (from, toNamespace, toTitle string, err error) {
		if len(ss) < 4 {
			err = errors.Errorf("Error: Invalid serialization expected at least 4, found %v in %v", len(ss), ss)
			return
		}
		return ss[0], ss[3], ss[2], nil
	}
	return rEdgeSource{p.dumpIterator(ctx, "pagelinkstable"), ev, gl.Title2ID, extractFields}
}

type rEdgeSource struct {
	rReadCloser
	ev            myValidator
	title2ID      func(namespace int, title string) (ID uint32, ok bool)
	extractFields func(ss []string) (from, toNamespace, toTitle string, err error)
}

func (ps rEdgeSource) Next() (e edge, err error) {
	var ss []string
	for ss, err = ps.Read(); err == nil; ss, err = ps.Read() {
		var from, toNamespace, toTitle string
		from, toNamespace, toTitle, err = ps.extractFields(ss)
		if err != nil {
			return
		}

		var ok bool
		e, ok, err = ps.parseEdge(from, toNamespace, toTitle)
		if err != nil {
			return
		}

		if ok && ps.ev.Validate(e) {
			return
		} //else grab another edge
	}
	return
}

func (ps rEdgeSource) parseEdge(sFrom, sToNamespace, toTitle string) (e edge, ok bool, err error) {
	//parse edge tail id
	from, err := strconv.ParseUint(sFrom, 10, 32)
	if err != nil {
		err = errors.Wrapf(err, "Error: error while parsing ID from %v", sFrom)
		return
	}

	//parse edge top namespace
	toNamespace, err := strconv.ParseInt(sToNamespace, 10, 32)
	if err != nil {
		err = errors.Wrapf(err, "Error: error while parsing namespace from %v", sToNamespace)
		return
	}

	//converts namespace+title of the tail to its id
	to, ok := ps.title2ID(int(toNamespace), toTitle)

	e = edge{uint32(from), to}

	return
}

func newValidator(validDomains ...edgeDomain) myValidator {
	return myValidator{validDomains}
}

type edgeDomain struct {
	From, To *roaring.Bitmap
}

type myValidator struct {
	domains []edgeDomain
}

func (ed myValidator) Validate(e edge) (ok bool) {
	for _, vd := range ed.domains {
		if vd.From.Contains(e.From) && vd.To.Contains(e.To) {
			ok = true
			break
		}
	}
	return
}

func (p semanticGraph) dumpIterator(ctx context.Context, filename string) (rr rReadCloser) {
	r, err := p.Dumps(filename)
	if err != nil {
		return errorRReader{err}
	}

	bRead := csv.NewReader(r).Read
	read := func() (record []string, err error) {
		record, err = bRead()
		if err != io.EOF {
			err = errors.Wrap(err, "Error while reading from "+filename)
		}
		return
	}

	return rReadCloseAdapter{read, r.Close}
}

type rReadCloser interface {
	Read() (record []string, err error)
	io.Closer
}

type rReadCloseAdapter struct {
	read  func() (record []string, err error)
	close func() error
}

func (r rReadCloseAdapter) Read() (record []string, err error) {
	return r.read()
}

func (r rReadCloseAdapter) Close() error {
	return r.close()
}

type errorRReader struct {
	err error
}

func (r errorRReader) Read() (record []string, err error) {
	if r.err == nil {
		r.err = io.ErrUnexpectedEOF
	}
	return nil, r.err
}

func (r errorRReader) Close() error {
	if r.err == nil {
		r.err = io.ErrUnexpectedEOF
	}
	return r.err
}

type errorEdgeSource struct {
	err error
}

func (r errorEdgeSource) Next() (e edge, err error) {
	if r.err == nil {
		r.err = io.ErrUnexpectedEOF
	}
	return edge{}, r.err
}

func (r errorEdgeSource) Close() error {
	if r.err == nil {
		r.err = io.ErrUnexpectedEOF
	}
	return r.err
}
