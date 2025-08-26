package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/huh"
	progressbar "github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

type Row struct {
	Era     string
	Nature  string
	Scholar string
	Sorted  string
	Raw     string
	Rhyme   string
	Info    string
}

func validateText(s string) error {
	if s == "" {
		return errors.New("no characters provided")
	}
	for _, ch := range s {
		if !unicode.Is(unicode.Han, ch) {
			return fmt.Errorf("invalid character: %q", ch)
		}
	}
	return nil
}

func runTUI() (string, string, []string, error) {
	var text string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("輸入待查字串").Value(&text).Validate(func(v string) error {
				return validateText(v)
			}),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", nil, err
	}

	chars := []rune(text)
	rows, err := fetchRows(string(chars[0]))
	if err != nil {
		return "", "", nil, err
	}
	options := make([]huh.Option[string], len(rows))
	for i, r := range rows {
		key := r.Era + "|" + r.Nature + "|" + r.Scholar
		label := fmt.Sprintf("%s %s %s", r.Era, r.Nature, r.Scholar)
		options[i] = huh.NewOption(label, key)
	}

	var selected []string
	output := "output.csv"
	confirm := false
	form = huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().Title("選擇學者").Options(options...).Value(&selected).Validate(func(v []string) error {
				if len(v) == 0 {
					return errors.New("at least one scholar required")
				}
				return nil
			}),
			huh.NewInput().Title("保存路徑").Value(&output),
			huh.NewConfirm().Title("開始導出?").Value(&confirm),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", nil, err
	}
	if !confirm {
		return "", "", nil, errors.New("aborted")
	}
	return text, output, selected, nil
}

func process(text, output string, selected []string) error {
	if err := validateText(text); err != nil {
		return err
	}
	chars := []rune(text)
	logger.Infof("processing %d characters", len(chars))

	firstRows, err := fetchRows(string(chars[0]))
	if err != nil {
		return err
	}
	logger.Infof("retrieved %d scholar rows for %c", len(firstRows), chars[0])
	allKeys := make([]string, len(firstRows))
	for i, r := range firstRows {
		allKeys[i] = r.Era + "|" + r.Nature + "|" + r.Scholar
	}
	if len(selected) == 0 {
		selected = allKeys
	}

	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()
	w.Write([]string{"字", "學者", "擬音[經整理]", "擬音[原材料]", "韻部", "原表其他信息"})

	bar := progressbar.NewOptions(len(chars),
		progressbar.OptionSetDescription("Processing"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetElapsedTime(true),
		progressbar.OptionSetTheme(progressbar.Theme{Saucer: "=", SaucerHead: ">", SaucerPadding: " ", BarStart: "[", BarEnd: "]"}),
		progressbar.OptionClearOnFinish(),
	)

	for idx, ch := range chars {
		logger.Infof("fetching %c (%d/%d)", ch, idx+1, len(chars))
		var rows []Row
		if idx == 0 {
			rows = firstRows
		} else {
			time.Sleep(1 * time.Second)
			rows, err = fetchRows(string(ch))
			if err != nil {
				logger.WithError(err).Warnf("fetch %c failed", ch)
				bar.Add(1)
				continue
			}
		}
		m := make(map[string]Row)
		for _, r := range rows {
			key := r.Era + "|" + r.Nature + "|" + r.Scholar
			m[key] = r
		}
		for _, key := range selected {
			r, ok := m[key]
			if !ok {
				w.Write([]string{string(ch), "", "", "", "", ""})
				continue
			}
			w.Write([]string{string(ch), r.Scholar, r.Sorted, r.Raw, r.Rhyme, r.Info})
		}
		bar.Add(1)
	}
	logger.Infof("output written to %s", output)
	return nil
}

func fetchRows(char string) ([]Row, error) {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		logger.WithFields(logrus.Fields{"char": char, "attempt": attempt}).Debug("requesting")
		rows, err := fetchRowsOnce(char)
		if err == nil && len(rows) > 0 {
			logger.WithFields(logrus.Fields{"char": char, "rows": len(rows)}).Debug("fetched")
			return rows, nil
		}
		if err == nil {
			err = fmt.Errorf("empty response")
		}
		lastErr = err
		logger.WithFields(logrus.Fields{"char": char, "attempt": attempt}).WithError(err).Warn("fetch failed")
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	return nil, fmt.Errorf("fetch %s failed: %w", char, lastErr)
}

func fetchRowsOnce(char string) ([]Row, error) {
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}

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
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logger.SetLevel(logrus.DebugLevel)

	if len(os.Args) == 1 {
		text, output, selected, err := runTUI()
		if err != nil {
			logger.Fatal(err)
		}
		if err := process(text, output, selected); err != nil {
			logger.Fatal(err)
		}
		return
	}

	text := os.Args[1]
	output := "output.csv"
	if len(os.Args) >= 3 {
		output = os.Args[2]
	}
	if err := process(text, output, nil); err != nil {
		logger.Fatal(err)
	}
}
