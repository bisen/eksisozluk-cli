package main

import (
	"github.com/yhat/scrape"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var scraper Scraper

type Scraper struct {
	entryMatcher     func(n *html.Node) bool
	authorMatcher    func(n *html.Node) bool
	dateMatcher      func(n *html.Node) bool
	entryListMatcher func(n *html.Node) bool
	topicListMatcher func(n *html.Node) bool
	indexListMatcher func(n *html.Node) bool
	contentMatcher   func(n *html.Node) bool
}

func init() {

	entryListMatcher := func(n *html.Node) bool {
		return strings.Contains(scrape.Attr(n, "id"), "entry-list")
	}

	entryMatcher := func(n *html.Node) bool {
		return strings.Contains(scrape.Attr(n, "class"), "content")
	}

	authorMatcher := func(n *html.Node) bool {
		return strings.Contains(scrape.Attr(n, "class"), "entry-author")
	}

	dateMatcher := func(n *html.Node) bool {
		return strings.Contains(scrape.Attr(n, "class"), "entry-date")
	}

	topicListMatcher := func(n *html.Node) bool {
		return strings.Contains(scrape.Attr(n, "class"), "topic-list")
	}

	indexListMatcher := func(n *html.Node) bool {
		return strings.Contains(scrape.Attr(n, "id"), "index-section")
	}

	contentMatcher := func(n *html.Node) bool {
		return strings.Contains(scrape.Attr(n, "id"), "content-body")
	}

	scraper = Scraper{entryMatcher, authorMatcher, dateMatcher, entryListMatcher, topicListMatcher, indexListMatcher, contentMatcher}
}

func (s Scraper) GetEntries(text string, parameter Parameter) []Entry {

	baseURL := "https://eksisozluk.com/?q=" + url.QueryEscape(text)

	resp, err := http.Get(baseURL)
	if err != nil {
		panic(err)
	}

	redirectedURL := resp.Request.URL.String()

	entryList := make([]Entry, 0)
	startPage := parameter.PageNumber

	for parameter.Limit > len(entryList) {

		paginationURL := redirectedURL + "?p=" + strconv.Itoa(startPage)

		if parameter.Sukela {
			paginationURL = paginationURL + "&a=nice"
		}

		additionalEntryList := getEntries(s, paginationURL)
		if len(additionalEntryList) == 0 {
			break
		}
		if len(entryList)+len(additionalEntryList) > parameter.Limit {
			entryList = append(entryList, additionalEntryList[0:(parameter.Limit-len(entryList))]...)
		} else {
			entryList = append(entryList, additionalEntryList...)
		}

		startPage = startPage + 1
	}

	return entryList
}

func (s Scraper) GetPopularTopics(parameter Parameter) []Topic {

	baseURL := "https://eksisozluk.com/basliklar/populer"

	topicList := make([]Topic, 0)
	startPage := parameter.PageNumber

	for parameter.Limit > len(topicList) {
		paginationURL := baseURL + "?p=" + strconv.Itoa(startPage)
		additionalTopicList := getTopics(s, paginationURL)
		if len(additionalTopicList) == 0 {
			break
		}
		if len(topicList)+len(additionalTopicList) > parameter.Limit {
			topicList = append(topicList, additionalTopicList[0:(parameter.Limit-len(topicList))]...)
		} else {
			topicList = append(topicList, additionalTopicList...)
		}

		startPage = startPage + 1
	}

	return topicList

}

func (s Scraper) GetDEBE(parameter Parameter) []Debe {

	debeTopics := getTopics(s, "https://eksisozluk.com/debe")

	debeList := make([]Debe, 0)

	for _, t := range debeTopics {
		if len(debeList) >= parameter.Limit {
			break
		}
		t.Count = 1 // Auto-correct count to 1 since only one entry is provided in DEBE
		currentDebe := Debe{}
		currentDebe.DebeTopic = t
		entryList := getEntries(s, t.Link)
		currentDebe.DebeEntry = entryList[0]
		debeList = append(debeList, currentDebe)
	}

	return debeList
}

func getEntries(s Scraper, eksiURL string) []Entry {

	log.Println("URL to check: " + eksiURL)
	resp, err := http.Get(eksiURL)
	if err != nil {
		panic(err)
	}
	root, err := html.Parse(resp.Body)
	if err != nil {
		panic(err)
	}

	entryList := make([]Entry, 0)

	entryListNode, found := scrape.Find(root, s.entryListMatcher)

	if found == false {
		return entryList
	}

	scrapedEntries := scrape.FindAll(entryListNode, s.entryMatcher)

	if len(scrapedEntries) == 0 {
		return entryList
	}
	for _, scrappedEntry := range scrapedEntries {
		authorNode, authorCheck := scrape.Find(scrappedEntry.Parent, s.authorMatcher)
		dateNode, dateCheck := scrape.Find(scrappedEntry.Parent, s.dateMatcher)

		entry := Entry{}
		entry.Text = scrape.Text(scrappedEntry)

		if authorCheck {
			entry.Author = scrape.Text(authorNode)
		}
		if dateCheck {
			idDate := scrape.Text(dateNode)
			splitted := strings.SplitAfterN(idDate, " ", 2)
			entry.Id = strings.TrimSpace(splitted[0])
			entry.Date = strings.TrimSpace(splitted[1])
		}
		entryList = append(entryList, entry)
	}

	return entryList
}

func getTopics(s Scraper, topicURL string) []Topic {

	log.Println("Checking URL: " + topicURL)
	topicList := make([]Topic, 0)

	resp, err := http.Get(topicURL)
	if err != nil {
		panic(err)
	}
	root, err := html.Parse(resp.Body)
	if err != nil {
		panic(err)
	}

	contentNode, _ := scrape.Find(root, s.contentMatcher)

	topicListNode, _ := scrape.Find(contentNode, s.topicListMatcher)

	topicLists := scrape.FindAll(topicListNode, scrape.ByTag(atom.Li))
	for _, topicNode := range topicLists {

		topic := Topic{}
		topicLink, _ := scrape.Find(topicNode, scrape.ByTag(atom.A))
		topic.Link = "https://eksisozluk.com" + scrape.Attr(topicLink, "href")

		titleAndCount := scrape.Text(topicNode)

		countIndex := strings.LastIndex(titleAndCount, " ")

		topic.Title = strings.TrimSpace(titleAndCount[0:countIndex])
		countString := titleAndCount[countIndex:]

		topicCountInt, _ := strconv.Atoi(strings.TrimSpace(countString))
		topic.Count = int64(topicCountInt)

		topicList = append(topicList, topic)
	}

	return topicList
}
