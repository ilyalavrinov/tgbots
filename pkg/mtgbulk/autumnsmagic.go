package mtgbulk

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly"
)

func searchAutumnsMagic(searchName string, names map[string]bool) CardResult {
	searchName = strings.ToLower(searchName)
	result := newCardResult()
	addr := autumnsMagickSearchURL(searchName)

	c := colly.NewCollector()
	c.SetRequestTimeout(20 * time.Second)
	c.OnHTML(".product-wrapper", func(e *colly.HTMLElement) {
		name := e.ChildText(".card-name a")
		if !names[strings.ToLower(name)] {
			logger.Debugw("skipping",
				"name", name)
			return
		}

		qtyStr := e.ChildText(".product-description span")
		qtyStr = strings.ReplaceAll(qtyStr, " шт.", "")
		qty, err := strconv.Atoi(qtyStr)
		if err != nil {
			logger.Errorw("card qty convert failed",
				"err", err)
			return
		}
		priceStr := e.ChildText(".product-price span.product-default-price")
		priceStr = strings.TrimSpace(priceStr)
		priceStr = strings.ReplaceAll(priceStr, " руб.", "")
		price, err := strconv.Atoi(priceStr)
		if err != nil {
			logger.Errorw("card price convert failed",
				"err", err)
			return
		}
		logger.Debugw("card",
			"searchName", searchName,
			"name", name,
			"price", price,
			"count", qty)

		result.Available = true
		result.Prices = append(result.Prices, CardPrice{
			Price:    float32(price),
			Foil:     false, // TODO: get this info
			Currency: RUR,
			Quantity: qty,
			Platform: MtgTrade,
			Trader:   "AutumnsMagic",
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

func autumnsMagickSearchURL(searchName string) string {
	searchName = strings.ReplaceAll(searchName, " ", "+")
	return fmt.Sprintf("https://autumnsmagic.com/catalog?search=%s", searchName)
}
