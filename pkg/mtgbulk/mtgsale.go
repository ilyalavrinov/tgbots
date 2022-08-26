package mtgbulk

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
)

func searchMtgSale(cardname string) CardResult {
	result := newCardResult()
	addr := mtgSaleSearchURL(cardname)

	c := colly.NewCollector()
	c.SetRequestTimeout(20 * time.Second)
	c.OnHTML(".ctclass", func(e *colly.HTMLElement) {
		name1 := strings.ToLower(e.ChildText(".tnamec"))
		name2 := strings.ToLower(e.ChildText(".smallfont"))
		cardname := strings.ToLower(cardname)
		logger.Debugw("parsing mtgsale card",
			"name1", name1,
			"name2", name2,
			"cardname", cardname)
		if name1 == cardname || name2 == cardname {
			p := e.ChildText(".pprice")
			p = strings.Trim(p, " ₽")
			pVal, err := strconv.Atoi(p)
			if err != nil {
				logger.Errorw("Price cannot be parsed",
					"card", cardname,
					"price", p,
					"err", err)
				return
			}

			foil := false
			if e.ChildText(".foil") != "" {
				foil = true
			}
			count := e.ChildText(".colvo")
			count = strings.Trim(count, " шт.")
			countVal, err := strconv.Atoi(count)
			if err != nil {
				logger.Errorw("Count cannot be parsed",
					"card", cardname,
					"count", count,
					"err", err)
				return
			}

			if countVal > 0 {
				result.Available = true
				result.Prices = append(result.Prices, CardPrice{
					Price:    float32(pVal),
					Foil:     foil,
					Currency: RUR,
					Quantity: countVal,
					Platform: MtgSale,
					Trader:   "mtgsale",
					URL:      addr, // TODO: correct it! - there's a direct link to a card instead of a search
				})
			}
		}
	})

	err := c.Visit(addr)
	if err != nil {
		logger.Errorw("Unable to visit with scraper",
			"url", addr,
			"err", err)
	}
	return result
}

func mtgSaleSearchURL(cardname string) string {
	return fmt.Sprintf("https://mtgsale.ru/home/search-results?Name=%s&Lang=Any&Type=Any&Color=Any&Rarity=Any", url.PathEscape(cardname))
}
