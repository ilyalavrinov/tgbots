package mtgbulk

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
)

func searchMtgTrade(cardname string) CardResult {
	cardname = strings.ToLower(cardname)
	result := newCardResult()
	addr := mtgTradeSearchURL(cardname)

	visitedPages := make(map[string]bool)
	c := colly.NewCollector()
	defCopy := *(http.DefaultTransport.(*http.Transport))
	var tr *http.Transport = &defCopy
	tr.ResponseHeaderTimeout = 20 * time.Second
	c.WithTransport(tr)
	c.OnHTML(".search-item", func(e *colly.HTMLElement) {
		nameEn := strings.ToLower(e.ChildText(".catalog-title"))
		if nameEn != cardname {
			matched := false
			e.ForEach("p", func(i int, eP *colly.HTMLElement) {
				if !matched {
					nameRu := strings.ToLower(strings.TrimSpace(eP.Text))
					logger.Debugw("search item analyze Russian name",
						"cardname", cardname,
						"nameEn", nameEn,
						"nameRu", nameRu)
					if nameRu == cardname {
						matched = true
					}
				}
			})

			if !matched {
				return
			}
		}

		e.ForEach("table.search-card", func(i int, eTable *colly.HTMLElement) {
			trader := eTable.ChildText("tbody .trader-name a")

			eTable.ForEach("tbody tr", func(i int, eTR *colly.HTMLElement) {
				price, err := strconv.ParseFloat(eTR.ChildText(".catalog-rate-price"), 32)
				if err != nil {
					logger.Errorw("card price convert failed",
						"err", err)
					return
				}

				quantity, err := strconv.Atoi(eTR.ChildText(".sale-count"))
				if err != nil {
					logger.Errorw("card count convert failed",
						"err", err)
					return
				}

				foil := false
				if eTR.ChildAttr("img.foil", "src") != "" {
					foil = true
				}

				logger.Debugw("card",
					"row_index", i,
					"trader", trader,
					"price", price,
					"count", quantity,
					"foil", foil,
					"quality", eTR.ChildText(".js-card-quality-tooltip")) // TODO: to CardPrice

				result.Available = true
				result.Prices = append(result.Prices, CardPrice{
					Price:    float32(price),
					Foil:     foil,
					Currency: RUR,
					Quantity: quantity,
					Platform: MtgTrade,
					Trader:   trader,
					URL:      addr, // TODO: correct it! - it's just a search result, but we can get a direct link to a card at a seller
				})
			})
		})
	})

	c.OnHTML("span.pagination-item", func(e *colly.HTMLElement) {
		page := e.Text
		visitedPages[e.Text] = true
		logger.Debugw("Visited page",
			"page", page)
	})

	c.OnHTML("a.pagination-item", func(e *colly.HTMLElement) {
		page := e.Attr("title")
		if visitedPages[page] {
			return
		}
		visitedPages[page] = true
		url := e.Attr("href")
		logger.Debugw("Visiting page",
			"page", page,
			"url", url)
		e.Request.Visit(url)
	})

	err := c.Visit(addr)
	if err != nil {
		logger.Errorw("Unable to visit with scraper",
			"url", addr,
			"err", err)
	}
	return result
}

func mtgTradeSearchURL(cardname string) string {
	cardname = strings.ReplaceAll(cardname, " ", "+")
	return fmt.Sprintf("http://mtgtrade.net/search/?query=%s", cardname)
}
