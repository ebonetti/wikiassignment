package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	json "github.com/json-iterator/go"

	"github.com/negapedia/wikiassignment/nationalization"
)

func main() {
	lang2nationalization := nationalization.Sync(nationalizations()...)
	for lang, i18n := range lang2nationalization {
		if err := writeJSON(lang+".json", i18n); err != nil {
			panic(err.Error())
		}
	}
}

func nationalizations() (nationalizations []nationalization.Nationalization) {
	if files, err := ioutil.ReadDir("./"); err == nil {
		for _, f := range files {
			n := nationalization.Nationalization{}
			err := readJSON(f.Name(), &n)
			switch {
			case f.IsDir():
				//Do nothing
			case !strings.HasSuffix(f.Name(), ".json"):
				//Do nothing
			case err != nil:
				fmt.Println("While reading", f.Name(), "encountered", err)
			case len(n.Topics) == 0:
				//Do nothing
			default:
				nationalizations = append(nationalizations, n)
			}
		}
	}

	if len(nationalizations) == 0 {
		panic("No valid nationalizations found!")
	}

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
