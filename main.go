package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Row struct {
	Era     string
	Nature  string
	Scholar string
	Sorted  string
	Raw     string
	Rhyme   string
	Info    string
}

func fetchRows(char string) ([]Row, error) {
	form := url.Values{}
	form.Set("word", char)
	form.Set("bianti", "no")
	req, err := http.NewRequest("POST", "http://www.kaom.net/ny_word8.php", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://www.kaom.net")
	req.Header.Set("Referer", "http://www.kaom.net/ny_word.php")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	table := doc.Find("table").First()
	var rows []Row
	var curEra, curNature, curScholar string
	table.Find("tr").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			return
		}
		var texts []string
		s.Find("td").Each(func(_ int, c *goquery.Selection) {
			texts = append(texts, strings.TrimSpace(c.Text()))
		})
		n := len(texts)
		if n < 7 {
			return
		}
		// extract from end to handle missing leading cells
		sorted := texts[n-6]
		raw := texts[n-5]
		rhyme := texts[n-4]
		info := texts[n-3]
		if n-8 >= 0 {
			curScholar = texts[n-8]
		}
		if n-9 >= 0 {
			curNature = texts[n-9]
		}
		if n-10 >= 0 {
			curEra = texts[n-10]
		}
		row := Row{Era: curEra, Nature: curNature, Scholar: curScholar, Sorted: sorted, Raw: raw, Rhyme: rhyme, Info: info}
		rows = append(rows, row)
	})
	return rows, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: bulk-export <text> [output.csv]")
		os.Exit(1)
	}
	text := os.Args[1]
	output := "output.csv"
	if len(os.Args) >= 3 {
		output = os.Args[2]
	}

	chars := []rune(text)
	if len(chars) == 0 {
		log.Fatal("no characters provided")
	}

	firstRows, err := fetchRows(string(chars[0]))
	if err != nil {
		log.Fatal(err)
	}
	keys := make([]string, len(firstRows))
	for i, r := range firstRows {
		keys[i] = r.Era + "|" + r.Nature + "|" + r.Scholar
	}

	file, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	w.Write([]string{"字", "學者", "擬音[經整理]", "擬音[原材料]", "韻部", "原表其他信息"})

	for idx, ch := range chars {
		var rows []Row
		if idx == 0 {
			rows = firstRows
		} else {
			time.Sleep(500 * time.Millisecond)
			var err error
			rows, err = fetchRows(string(ch))
			if err != nil {
				log.Printf("fetch %c failed: %v", ch, err)
				continue
			}
		}
		m := make(map[string]Row)
		for _, r := range rows {
			key := r.Era + "|" + r.Nature + "|" + r.Scholar
			m[key] = r
		}
		for _, key := range keys {
			r, ok := m[key]
			if !ok {
				w.Write([]string{string(ch), "", "", "", "", ""})
				continue
			}
			w.Write([]string{string(ch), r.Scholar, r.Sorted, r.Raw, r.Rhyme, r.Info})
		}
	}
}
