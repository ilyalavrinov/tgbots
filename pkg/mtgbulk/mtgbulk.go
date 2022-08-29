package mtgbulk

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// TODO: remove this ugly hack
var cardLib Library
var libOnce sync.Once

type NamesRequest struct {
	Cards map[string]int

	DeliveryFee int
	onlySingles *bool
}

func (req *NamesRequest) hasOnlySingles() bool {
	if req.onlySingles != nil {
		return *req.onlySingles
	}

	onlySingles := true
	for _, n := range req.Cards {
		if n > 1 {
			onlySingles = false
			break
		}
	}
	req.onlySingles = &onlySingles
	return *req.onlySingles
}

func NewNamesRequest() NamesRequest {
	return NamesRequest{
		Cards: make(map[string]int),
	}
}

type PlatformType int

const (
	MtgSale      PlatformType = iota
	MtgTrade     PlatformType = iota
	SpellMarket  PlatformType = iota
	AutumnsMagic PlatformType = iota
	TopDeck      PlatformType = iota
)

var shops = map[PlatformType]bool{
	MtgSale:      true,
	SpellMarket:  true,
	AutumnsMagic: true,
}

func (pt PlatformType) String() string {
	switch pt {
	case MtgSale:
		return "MtgSale"
	case MtgTrade:
		return "MtgTrade"
	case SpellMarket:
		return "SpellMarket"
	case AutumnsMagic:
		return "AutumsMagic"
	case TopDeck:
		return "TopDeck"
	}
	return ""
}

type CurrencyType int

const (
	RUR CurrencyType = iota
	USD CurrencyType = iota
)

func (c CurrencyType) String() string {
	res := ""
	switch c {
	case RUR:
		res = "₽"
	case USD:
		res = "$"
	}
	return res
}

func (c CurrencyType) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

type CardPrice struct {
	Price    float32
	Foil     bool
	Currency CurrencyType
	Quantity int

	Platform PlatformType
	Trader   string
	URL      string
}

func (cp *CardPrice) SellerFullName() string {
	if shops[cp.Platform] {
		return cp.Trader
	}
	return cp.Trader + "@" + cp.Platform.String()
}

type CardResult struct {
	Available bool
	Prices    []CardPrice
}

func newCardResult() CardResult {
	return CardResult{
		Available: false,
		Prices:    make([]CardPrice, 0),
	}
}

func (c *CardResult) merge(other CardResult) {
	if c.Available || other.Available {
		c.Available = true
	}
	c.Prices = append(c.Prices, other.Prices...)
}

func (c *CardResult) sortByPrice() {
	sort.Slice(c.Prices, func(i, j int) bool {
		return c.Prices[i].Price < c.Prices[j].Price
	})
}

type NamesResult struct {
	AllSortedCards    map[string]CardResult
	NotAvailableCards []string

	MinPricesNoDelivery          map[string][]CardPrice
	WithDeliveryByEliminateFewer map[string]CardPrice
	MinPricesMatrix              *PossessionMatrix
}

func ProcessByNames(req NamesRequest) (*NamesResult, error) {
	logger.Debugw("Incoming ProcessByNames request",
		"count", len(req.Cards))

	result := &NamesResult{
		AllSortedCards: make(map[string]CardResult, len(req.Cards)),
	}

	// TODO: remove this ugly hack
	libOnce.Do(func() {
		dumpPath := "./scryfall.all.dump"
		var err error
		cardLib, err = NewInMemoryLibrary(dumpPath)
		if err != nil {
			panic(err)
		}
	})

	for name := range req.Cards {
		allNames, err := cardLib.CardAliases(name)
		if err != nil {
			logger.Errorw("could not get all names for card, is it missing?",
				"err", err)
			return result, err
		}

		englishName, err := cardLib.EnglishName(name)
		if err != nil {
			logger.Errorw("could not get english name for card, is it missing?",
				"err", err)
			return result, err
		}

		cardRes := newCardResult()
		cardRes.merge(searchMtgSale(name))
		cardRes.merge(searchMtgTrade(name))
		cardRes.merge(searchSpellMarket(name, allNames))
		cardRes.merge(searchAutumnsMagic(englishName, allNames))
		cardRes.merge(searchTopDeck(name))
		cardRes.sortByPrice()
		if cardRes.Available {
			result.AllSortedCards[name] = cardRes
		} else {
			result.NotAvailableCards = append(result.NotAvailableCards, name)
		}
	}

	greedyMinPrices, err := calcGreedyMinPrices(req, result.AllSortedCards)
	if err != nil {
		logger.Errorw("could not calculate greedy min prices",
			"err", err)
		return result, err
	}
	result.MinPricesNoDelivery = greedyMinPrices

	result.MinPricesMatrix = fillMinPricesMatrix(result.AllSortedCards)

	if req.DeliveryFee > 0 && req.hasOnlySingles() {
		eliminateFewer, err := evaluateConsideringDelivery(req, result.AllSortedCards, greedyMinPrices)
		if err != nil {
			logger.Errorw("could not calculate min prices with delivery",
				"err", err)
			return result, err
		}
		result.WithDeliveryByEliminateFewer = eliminateFewer
	}

	return result, nil
}

var quantityRe *regexp.Regexp = regexp.MustCompile("^(\\d+)x?\\s*(.*)$")

func parseLine(line string) (string, int, error) {
	quantity := 1
	cardname := line
	var err error
	if quantityRe.MatchString(line) {
		matches := quantityRe.FindAllStringSubmatch(line, -1)

		quantity, err = strconv.Atoi(matches[0][1])
		if err != nil {
			return "", 0, err
		}
		cardname = matches[0][2]
		if len(cardname) == 0 {
			return "", 0, fmt.Errorf("empty cardname")
		}
	}

	return cardname, quantity, nil
}

func ProcessText(r io.Reader) (*NamesResult, error) {
	cards := NewNamesRequest()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		logger.Debugw("New line read from body",
			"line", line)
		line = strings.Trim(line, " ")
		if len(line) == 0 {
			continue
		}
		name, quantity, err := parseLine(line)
		if err != nil {
			logger.Warnw("could not parse line",
				"err", err,
				"line", line)
			return nil, err
		}

		logger.Debugw("Parsed line",
			"line", line,
			"cardname", name,
			"quantity", quantity)

		if _, found := cards.Cards[name]; found {
			logger.Warnw("Duplicated card",
				"name", name)
			return nil, fmt.Errorf("Card with name %q is duplicated in the list", name)
		}

		if quantity <= 0 {
			logger.Warnw("Illegal requested quantity",
				"name", name,
				"quantity", quantity)
			return nil, fmt.Errorf("Illegal quantity for card %q has been requested: %d", name, quantity)
		}

		cards.Cards[name] = quantity
	}
	if err := scanner.Err(); err != nil {
		logger.Warnw("Error reading body",
			"err", err)
		return nil, err
	}

	if len(cards.Cards) == 0 {
		logger.Warnw("Empty card list")
		return nil, fmt.Errorf("Empty card list")
	}

	result, err := ProcessByNames(cards)
	if err != nil {
		logger.Warnw("Could not process request",
			"err", err)
		return nil, err
	}

	return result, nil
}

func calcGreedyMinPrices(req NamesRequest, cards map[string]CardResult) (map[string][]CardPrice, error) {
	result := make(map[string][]CardPrice, len(req.Cards))

	for name, reqCount := range req.Cards {
		cardData, found := cards[name]
		if !found || !cardData.Available {
			logger.Debugw("card not ready for greedy calc, skipping",
				"name", name)
			continue
		}

		cardsFound := 0
		for _, p := range cardData.Prices {
			if cardsFound >= reqCount {
				break
			}
			toAdd := p
			if toAdd.Quantity > reqCount-cardsFound {
				toAdd.Quantity = reqCount - cardsFound
			}
			result[name] = append(result[name], toAdd)
			cardsFound += toAdd.Quantity
			logger.Debugw("greedy min price add result",
				"name", name,
				"qty", toAdd.Quantity,
				"price", toAdd.Price,
				"totalFound", cardsFound,
				"reqCount", reqCount)
		}
	}

	return result, nil
}

func fillMinPricesMatrix(cards map[string]CardResult) *PossessionMatrix {
	m := NewPossessionMatrix()
	for c, res := range cards {
		for _, p := range res.Prices {
			m.AddCard(p.SellerFullName(), c, int(p.Price))
		}
	}
	return m
}

func evaluateConsideringDelivery(req NamesRequest, cards map[string]CardResult, minPrices map[string][]CardPrice) (map[string]CardPrice, error) {
	sellerCards := make(map[string]map[string]bool) // trader -> cardnames -> true
	cardSellers := make(map[string]map[string]bool) // cardname -> traders -> true
	cardSellerMinPrice := make(map[sellerCardPair]float32)
	sellersWithUniqueCards := make(map[string][]string) // trader -> cardnames
	for cardname, res := range cards {
		if !res.Available {
			logger.Debugw("delivery calc card not available",
				"card", cardname)
		}

		cardSellers[cardname] = make(map[string]bool)

		sellers := make(map[string]struct{})
		for _, cardprice := range res.Prices {
			trader := cardprice.Trader
			cardSellers[cardname][trader] = true
			if _, found := sellerCards[trader]; !found {
				sellerCards[trader] = make(map[string]bool, len(cards))
			}
			sellerCards[trader][cardname] = true
			sellers[trader] = struct{}{}

			pair := sellerCardPair{seller: trader, cardname: cardname}
			price := cardSellerMinPrice[pair]
			if price == 0 || price > cardprice.Price {
				cardSellerMinPrice[pair] = cardprice.Price
				logger.Debugw("card seller new min price",
					"card", cardname,
					"seller", trader,
					"price", cardprice.Price)
			}
		}

		if len(sellers) == 1 {
			uniqueSeller := ""
			for s := range sellers {
				uniqueSeller = s
			}
			sellersWithUniqueCards[uniqueSeller] = append(sellersWithUniqueCards[uniqueSeller], cardname)
			logger.Debugw("trader uniquely sells a card",
				"trader", uniqueSeller,
				"card", cardname)
		}
	}
	logger.Debugw("stats collected",
		"totalSellers", len(sellerCards),
		"totalUniqueSellers", len(sellersWithUniqueCards))

	evalData := deliveryEvalData{
		deliveryFee:        req.DeliveryFee,
		cards:              cards,
		sellerCards:        sellerCards,
		cardSellers:        cardSellers,
		sellerCardMinPrice: cardSellerMinPrice,
	}
	evaluateDeliveryViaPermutation(evalData)

	return nil, nil
}

type sellerCardPair struct {
	seller, cardname string
}

type deliveryEvalData struct {
	deliveryFee              int
	cards                    map[string]CardResult
	sellerCards, cardSellers map[string]map[string]bool
	sellerCardMinPrice       map[sellerCardPair]float32
}

func evaluateDeliveryViaPermutation(data deliveryEvalData) (int, []sellerCardPair) {
	cost, result := iteratePermutation(map[string]bool{}, []sellerCardPair{}, data)
	logger.Debugw("permutation best result",
		"cost", cost)
	return cost, result
}

func iteratePermutation(cardsPicked map[string]bool, resultSet []sellerCardPair, data deliveryEvalData) (int, []sellerCardPair) {
	if len(cardsPicked) == len(data.cards) {
		cost := 0
		sellersMet := make(map[string]bool)
		for _, cs := range resultSet {
			cost += int(data.sellerCardMinPrice[cs])
			if !sellersMet[cs.seller] {
				sellersMet[cs.seller] = true
				cost += data.deliveryFee
			}
		}
		logger.Debugw("permutation end",
			"cost", cost)
		return cost, resultSet
	}

	bestCost := math.MaxInt32
	var bestResult []sellerCardPair
	for card := range data.cards {
		if cardsPicked[card] {
			logger.Debugw("card already picked",
				"card", card,
				"picked", len(cardsPicked))
			continue
		}
		cardsPicked[card] = true
		logger.Debugw("card picked",
			"card", card,
			"picked", len(cardsPicked))
		for seller := range data.cardSellers[card] {
			logger.Debugw("seller picked",
				"card", card,
				"picked", len(cardsPicked),
				"seller", seller)
			resultSet = append(resultSet, sellerCardPair{seller: seller, cardname: card})
			price, rs := iteratePermutation(cardsPicked, resultSet, data)
			if price < bestCost {
				logger.Debugw("permutation better result",
					"cost", price,
					"cards picked", len(cardsPicked),
					"card", card,
					"seller", seller)
				bestCost = price
				bestResult = rs
			}
			resultSet = resultSet[0 : len(resultSet)-1]
		}
		delete(cardsPicked, card)
	}

	logger.Debugw("permutation exhausted",
		"cost", bestCost,
		"picked", len(cardsPicked))
	return bestCost, bestResult
}
