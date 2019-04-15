package nationalization

import (
	"testing"
)

func TestNationalizations(t *testing.T) {
	en, err := New("en")
	if err != nil {
		t.Error("Error while fetching nationalization ", err)
	}
	for _, lang := range List() {
		n, err := New(lang)
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
	n := Nationalization{
		Language: "en",
		Topics: []struct {
			Page
			Categories []Page `json:",omitempty"`
		}{
			{Page{2147483637, "Culture and the arts"}, []Page{{4892515, "Category:Arts"}}},
		},
	}
	if Sync(n)["en"].Topics[0].Categories[0].Title != "Category:Arts" {
		t.Error("Category:Arts not found")
	}
}
