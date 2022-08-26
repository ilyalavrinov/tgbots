package mtgbulkbuy

import (
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/ilyalavrinov/tgbots/pkg/mtgbulk"
	"github.com/ilyalavrinov/tgbots/pkg/tgbotbase"
	"github.com/tealeg/xlsx"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

type searchHandler struct {
	tgbotbase.BaseHandler
}

func NewSearchHandler() tgbotbase.IncomingMessageHandler {
	handler := searchHandler{}
	return &handler
}

func (h *searchHandler) Init(outMsgCh chan<- tgbotapi.Chattable, srvCh chan<- tgbotbase.ServiceMsg) tgbotbase.HandlerTrigger {
	h.OutMsgCh = outMsgCh
	return tgbotbase.NewHandlerTrigger(regexp.MustCompile(".*"), nil)
}

func (h *searchHandler) Name() string {
	return "bulk_search"
}

func (h *searchHandler) HandleOne(msg tgbotapi.Message) {
	r := strings.NewReader(msg.Text)
	res, err := mtgbulk.ProcessText(r)
	var reply tgbotapi.Chattable
	if err != nil {
		r := tgbotapi.NewMessage(msg.Chat.ID, err.Error())
		r.BaseChat.ReplyToMessageID = msg.MessageID
		reply = r
	} else {
		fxls := xlsx.NewFile()
		sh, err := fxls.AddSheet("min_prices_all")
		if err != nil {
			r := tgbotapi.NewMessage(msg.Chat.ID, err.Error())
			r.BaseChat.ReplyToMessageID = msg.MessageID
			reply = r
		} else {
			minPrices := make(map[string]int, len(res.MinPricesNoDelivery))
			for card, pp := range res.MinPricesNoDelivery {
				minPrices[card] = int(pp[0].Price)
			}
			t := mtgbulk.NewPossessionTable(res.MinPricesMatrix)
			t.ToXlsxSheet(sh, minPrices)
		}

		f, err := ioutil.TempFile("", "*-mtgbulk.xlsx")
		fxls.Write(f)
		f.Close()

		reply = tgbotapi.NewDocumentUpload(msg.Chat.ID, f.Name())
	}

	h.OutMsgCh <- reply
}
