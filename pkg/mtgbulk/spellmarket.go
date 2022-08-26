package mtgbulk

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

func searchSpellMarket(searchName string, names map[string]bool) CardResult {
	result := newCardResult()
	addr := spellMarketSearchURL(searchName)
	c := colly.NewCollector()

	currency1 := &http.Cookie{Name: "currency", Value: "RUB"}
	currency2 := &http.Cookie{Name: "prmn_currency", Value: "RUB"}
	c.SetCookies(addr, []*http.Cookie{currency1, currency2})

	c.OnHTML("div.product-wrapper", func(e *colly.HTMLElement) {
		if strings.Contains(e.Attr("class"), "outofstock") {
			return
		}

		name := strings.ToLower(e.ChildText(".name"))
		if !names[name] {
			return
		}

		pStr := e.ChildText(".price")
		pStr = strings.ReplaceAll(pStr, " р.", "")
		price, err := strconv.ParseFloat(pStr, 32)
		if err != nil {
			logger.Errorw("card price convert failed",
				"err", err)
			return
		}

		qty, err := strconv.Atoi(e.ChildText(".quantity span"))
		if err != nil {
			logger.Errorw("card qty convert failed",
				"err", err)
			return
		}

		logger.Debugw("card found",
			"searchName", searchName,
			"name", name,
			"price", price,
			"qty", qty)

		result.Available = true
		result.Prices = append(result.Prices, CardPrice{
			Price:    float32(price),
			Foil:     false, // TODO
			Currency: RUR,
			Quantity: qty,
			Platform: SpellMarket,
			Trader:   "spellmarket",
			URL:      addr, // TODO: correct it! - it's just a search result, but we can get a direct link to a card at a seller
		})
	})

	err := c.Visit(addr)
	if err != nil {
		logger.Errorw("Unable to visit with scraper",
			"url", addr,
			"err", err)
	}

	return result
}

func spellMarketSearchURL(searchName string) string {
	return fmt.Sprintf("https://spellmarket.ru/search?search=%s%s", url.PathEscape(searchName), url.PathEscape("&limit=1000"))
}
