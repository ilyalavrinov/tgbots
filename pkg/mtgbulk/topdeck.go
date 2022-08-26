package mtgbulk

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

var re = regexp.MustCompile("JSON\\.parse\\(\\\"(.*)\\\"\\)\\,")

type topdeckCard struct {
	EngName string `json:"rus_name"`
	RusName string `json:"eng_name"`
	URL     string `json:"url"`
	Seller  struct {
		Name string `json:"name"`
	} `json:"seller"`
	Qty    int    `json:"qty"`
	Cost   int    `json:"cost"`
	Source string `json:"source"`
}

func searchTopDeck(cardname string) CardResult {
	cardname = strings.ToLower(cardname)
	result := newCardResult()
	addr := topDeckSearchURL(cardname)

	c := colly.NewCollector()

	c.OnHTML("script", func(e *colly.HTMLElement) {
		matches := re.FindAllSubmatch([]byte(e.Text), -1)
		if len(matches) == 0 {
			return
		}
		text := matches[0][1]
		pos := 0
		finalTxt := ""
		for pos < len(text) {
			if text[pos] == '\\' && text[pos+1] == 'u' {
				s := fmt.Sprintf("'%s'", text[pos:pos+6])
				c, err := strconv.Unquote(s)
				if err != nil {
					logger.Errorw("Unquote failed",
						"err", err)
					return
				}
				finalTxt = finalTxt + c
				pos += 6
			} else {
				finalTxt = finalTxt + string(text[pos])
				pos++
			}
		}
		finalTxt = strings.ReplaceAll(finalTxt, "\\\"", "")
		finalTxt = strings.ReplaceAll(finalTxt, "\\", "")
		//fmt.Printf("RESULT %s\n", finalTxt)

		dec := json.NewDecoder(strings.NewReader(finalTxt))
		_, err := dec.Token()
		if err != nil {
			logger.Errorw("get opening failed",
				"err", err)
			return
		}
		for dec.More() {
			var c topdeckCard
			err := dec.Decode(&c)
			if c.Source != "topdeck" {
				// some other shop like spellmarket or mtgsale
				continue
			}
			if err != nil {
				logger.Errorw("decode failed",
					"err", err)
				continue
			}
			if strings.ToLower(c.RusName) != cardname && strings.ToLower(c.EngName) != cardname {
				continue
			}

			logger.Debugw("card found",
				"cardname", cardname,
				"ru_name", c.RusName,
				"en_name", c.EngName,
				"cost", c.Cost,
				"qty", c.Qty)

			result.Available = true
			result.Prices = append(result.Prices, CardPrice{
				Price:    float32(c.Cost),
				Foil:     false,
				Currency: RUR,
				Quantity: c.Qty,
				Platform: TopDeck,
				Trader:   c.Seller.Name,
				URL:      c.URL,
			})
		}
		_, err = dec.Token()
		if err != nil {
			logger.Errorw("read closing failed",
				"err", err)
			return
		}
	})

	err := c.Visit(addr)
	if err != nil {
		logger.Errorw("Unable to scrape",
			"url", addr,
			"err", err)
	}

	return result
}

func topDeckSearchURL(cardname string) string {
	cardname = strings.ReplaceAll(cardname, " ", "+")
	return fmt.Sprintf("https://topdeck.ru/apps/toptrade/singles/search?q=%s", cardname)
}
