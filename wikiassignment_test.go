package wikiassignment

import (
	"context"
	"testing"

	"github.com/negapedia/wikiassignment/nationalization"
)

func TestUnit(t *testing.T) {
	const lang = "vec"
	n, err := nationalization.New(lang)
	if err != nil {
		t.Error("Error while fetching nationalization ", err)
	}

	page2Topic, ns, err := From(context.Background(), "", lang)
	if err != nil {
		t.Error("Error while processing ", err)
	}
	for i, topic := range n.Topics {
		if topic.ID != ns.Topics[i] {
			t.Error("Topic not found ", topic.Title)
		}
	}
	for _, topic := range n.Topics {
		for _, category := range topic.Categories {
			computedTopic, ok := page2Topic[category.ID]
			if ok && computedTopic != topic.ID {
				t.Error(category.Title, "is not assigned to ", topic.Title)
			}
		}
	}
	if len(page2Topic) != len(ns.Articles)+len(ns.Categories)+len(ns.Topics) {
		t.Error("Mismatch in node count.")
	}
}
