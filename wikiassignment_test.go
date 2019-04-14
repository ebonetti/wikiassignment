package wikiassignment

import (
	"context"
	"testing"

	"github.com/ebonetti/wikiassignment/nationalization"
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

func TestNationalizations(t *testing.T) {
	en, err := nationalization.New("en")
	if err != nil {
		t.Error("Error while fetching nationalization ", err)
	}
	for _, lang := range nationalization.List() {
		n, err := nationalization.New(lang)
		if err != nil {
			t.Error("Error while fetching nationalization ", err)
		}
		for i, topic := range en.Topics {
			switch {
			case topic.ID != n.Topics[i].ID:
				t.Error("Topic ID not found ", topic.Title)
			case topic.Title != n.Topics[i].Title:
				t.Error("Topic Title not found ", topic.Title)
			}
		}
	}
}

func TestNationalizationSync(t *testing.T) {
	n := nationalization.Nationalization{
		Language: "en",
		Topics: []struct {
			nationalization.Page
			Categories []nationalization.Page `json:",omitempty"`
		}{
			{nationalization.Page{2147483637, "Culture and the arts"}, []nationalization.Page{{4892515, "Category:Arts"}}},
		},
	}
	if nationalization.Sync(n)["en"].Topics[0].Categories[0].Title != "Category:Arts" {
		t.Error("Category:Arts not found")
	}

}
