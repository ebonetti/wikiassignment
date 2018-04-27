package wikiassignment

import (
	"fmt"
	"io"

	"github.com/RoaringBitmap/roaring"
	"github.com/pkg/errors"
)

type mapGraphLoader struct {
	Edges         mapGraph
	Namespace2IDs map[int]*roaring.Bitmap
	nTitle2ID
	Err error
}

type pageSourcer interface {
	Next() (page, error)
	io.Closer
}

type page struct {
	ID        uint32
	Namespace int
	Title     string
}

func (gl *mapGraphLoader) AddNodes(nss ...pageSourcer) *mapGraphLoader {
	for _, ns := range nss {
		if gl.Err != nil {
			return gl
		}

		nodes := roaring.NewBitmap()
		for _, ids := range gl.Namespace2IDs {
			nodes.Or(ids)
		}

		var p page
		var err error

		next := ns.Next
		for p, err = next(); err == nil; p, err = next() {
			if nodes.Contains(p.ID) {
				err = errors.Errorf("Error: adding duplicate node with id %v", p.ID)
				break
			}
			nodes.Add(p.ID)
			gl.AddPage(p.Namespace, p.Title, p.ID)
			nsSet, ok := gl.Namespace2IDs[p.Namespace]
			if !ok {
				nsSet = roaring.NewBitmap()
				gl.Namespace2IDs[p.Namespace] = nsSet
			}
			nsSet.Add(p.ID)
		}

		gl.Err = ns.Close()
		if err != io.EOF { // err != nil due to "for" condition
			gl.Err = err
		}
	}
	return gl
}

type edge struct {
	From, To uint32
}

type edgeSourcer interface {
	Next() (edge, error)
	io.Closer
}

func (gl *mapGraphLoader) AddEdges(ess ...edgeSourcer) *mapGraphLoader {
	for _, es := range ess {
		if gl.Err != nil {
			return gl
		}

		var e edge
		var err error
		next := es.Next

		for e, err = next(); err == nil; e, err = next() {
			gl.Edges.Add(e.From, e.To)
		}

		gl.Err = es.Close()
		if err != io.EOF { // err != nil due to "for" condition
			gl.Err = err
		}
	}
	return gl
}

func (gl *mapGraphLoader) Filter(ff ...Filter) *mapGraphLoader {
	if gl.Err != nil || len(ff) == 0 {
		return gl
	}

	whitelist := gl.Edges.Nodes()
	for _, f := range ff {
		nodes := gl.Edges.InSubgraph(roaring.BitmapOf(f.Parents...), f.Dept)
		if f.IsWhitelist {
			whitelist = nodes
		} else {
			whitelist.AndNot(nodes)
		}
		gl.Edges.ApplySubgraph(whitelist)
	}

	return gl
}

type nTitle2ID struct {
	m map[string]uint32
}

func (d nTitle2ID) AddPage(namespace int, title string, ID uint32) {
	d.m[prefixedTitle(namespace, title)] = ID
}

func (d nTitle2ID) Title2ID(namespace int, title string) (ID uint32, ok bool) {
	ID, ok = d.m[prefixedTitle(namespace, title)]
	return
}

func prefixedTitle(namespace int, title string) string {
	return fmt.Sprintf("%v %v", namespace, title)
}
