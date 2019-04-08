package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ebonetti/wikiassignment/nationalization"
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
