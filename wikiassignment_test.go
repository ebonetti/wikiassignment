package wikiassignment

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/ebonetti/wikidump"
)

func TestUnit(t *testing.T) {
	dumps := func(name string) (io.ReadCloser, error) {
		text := ""
		switch name {
		case "pagetable":
			text = pageCSV
		case "categorylinkstable":
			text = categorylinksCSV
		case "pagelinkstable":
			text = pagelinksCSV
		default:
			t.Error("Unknown name", name)
		}
		return readClose{strings.NewReader(text), func() error { return nil }}, nil
	}
	topic2Pages := map[uint32][]uint32{
		0: {2, 4},
		1: {3, 4, 7},
	}
	page2Topic, ns, err := From(context.Background(), "", dumps, topic2Pages, nil)
	if err != nil {
		t.Error("Error while processing ", err)
	}

	for _, topic := range ns.Topics {
		if _, ok := topic2Pages[topic]; !ok {
			t.Error("Topic not found ", topic)
		}
	}
	for _, topic := range page2Topic {
		if _, ok := topic2Pages[topic]; !ok {
			t.Error(topic, "is not a topic, wrong assignment in", page2Topic)
		}
	}
	if len(page2Topic) != len(ns.Articles)+len(ns.Categories)+len(ns.Topics) {
		t.Error("Mismatch in node count.")
	}

	for i := uint32(0); i < 8; i++ {
		topic, ok := page2Topic[i]
		switch {
		case !ok:
			t.Error("Not found page", i)
		case topic != i%2:
			t.Error("Page", i, "should be assigned to", i%2, "but it's assigned to", topic)
		}
	}
}

type readClose struct {
	io.Reader
	Closer func() error
}

func (r readClose) Close() error {
	return r.Closer()
}

const pageCSV = `2,14,"Two","",0,0,0,0.42927655132,"20180417104603","20180417104822",95868109,14075,0,"wikitext",NULL
3,14,"Three","",0,0,0,0.42927655132,"20180417104603","20180417104822",95868109,14075,0,"wikitext",NULL
4,14,"Four","",0,0,0,0.42927655132,"20180417104603","20180417104822",95868109,14075,0,"wikitext",NULL
5,0,"Five","",0,0,0,0.42927655132,"20180417104603","20180417104822",95868109,14075,0,"wikitext",NULL
6,0,"Six","",0,0,0,0.42927655132,"20180417104603","20180417104822",95868109,14075,0,"wikitext",NULL
7,0,"Seven","",0,0,0,0.42927655132,"20180417104603","20180417104822",95868109,14075,0,"wikitext",NULL`

const categorylinksCSV = `2,"Four",")KAEC9QA","2017-03-11 18:45:11","","uca-it-u-kn","page"
3,"Four",")KAEC9QA","2018-01-26 12:45:39","","uca-it-u-kn","page"
4,"Two",")KAEC9QA","2018-01-26 12:45:39","","uca-it-u-kn","page"
5,"Three",")KAEC9QA","2018-01-26 12:45:39","","uca-it-u-kn","page"
6,"Two",")KAEC9QA","2018-03-02 11:38:29","","uca-it-u-kn","page"
6,"Four",")KAEC9QA","2018-01-26 12:45:39","","uca-it-u-kn","page"
7,"Three",")KAEC9QA","2018-01-26 12:45:39","","uca-it-u-kn","page"
7,"Four",")KAEC9QA","2018-01-26 12:45:39","","uca-it-u-kn","page"`

const pagelinksCSV = `5,0,"Seven",0
7,0,"Five",0`

func BenchmarkUnit(t *testing.B) {
	w, err := wikidump.Latest(".", "it", "pagetable", "categorylinkstable", "pagelinkstable")
	if err != nil {
		t.Error("Error while fetching wikidump ", err)
	}
	dumps := func(name string) (r io.ReadCloser, err error) {
		return w.Open(name)(context.Background())
	}

	topic2Categories := map[uint32][]uint32{
		21474800: {40553, 2047954, 5299452, 24754, 24741, 45721, 28486, 24821, 40558, 1494108, 27759},         //Cultura ed arte
		21474801: {24992, 463693, 1922055, 785776, 60716, 6390441},                                            //Geografia e luoghi
		21474802: {128360, 2605417, 1847540, 159928, 27684, 41409, 540748, 5300176},                           //Salute e benessere
		21474803: {24848, 1081804, 327908, 2979727},                                                           //Storia ed eventi
		21474804: {24852, 42517, 34541, 65875, 627354},                                                        //Matematica e logica
		21474805: {1106438, 2047961, 2048144, 24711, 27885, 2630789, 24566, 52701, 1300144, 24922, 269321},    //Scienze naturali e fisiche
		21474806: {27066, 401115, 75804, 3097777, 47275, 5286346},                                             //Persone e se stessi
		21474807: {24638, 2662229, 24798, 210754, 43123, 74247, 43239},                                        //Filosofia e pensiero
		21474808: {33147, 250486, 210754, 3283925, 2817978, 55906, 5404155, 3474584, 3474584, 403768, 495194}, //Religioni e credenze
		21474809: {51051, 43272, 2730680, 78440, 240649, 1588868},                                             //Società e scienze sociali
		21474810: {31787, 75186, 24695, 34201, 4695135, 27328, 3479877, 355530, 55915, 4120609, 24707},        //Tecnologie e scienze applicate
	}

	filters := []Filter{{false, []uint32{1641518}, 1}, {false, []uint32{24814}, -1}}
	page2Topic, ns, err := From(context.Background(), "", dumps, topic2Categories, filters)
	if err != nil {
		t.Error("Error while processing ", err)
	}
	for _, topic := range ns.Topics {
		if _, ok := topic2Categories[topic]; !ok {
			t.Error("Topic not found ", topic)
		}
	}
	for _, topic := range page2Topic {
		if _, ok := topic2Categories[topic]; !ok {
			t.Error(topic, "is not a topic, wrong assignment in", page2Topic)
		}
	}
	if len(page2Topic) != len(ns.Articles)+len(ns.Categories)+len(ns.Topics) {
		t.Error("Mismatch in node count.")
	}
}
