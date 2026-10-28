package searchindex

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/search"
)

const (
	k1 = 1.2
	b  = 0.75

	weightShortname = 6.0
	weightTitle     = 3.5
	weightTopics    = 2.5
	weightAuthors   = 1.5
	weightAbstract  = 1.0
)

type posting struct {
	docID   int
	tf      float64
	inTitle bool
	inID    bool
}

type docInfo struct {
	entry       entry.Entry
	docLen      float64
	year        int
	shortnameLC string
	titleLC     string
	authorLC    string
	topicsLC    []string
}

// BM25Index implements port.SearchIndex using in-memory inverted indices with BM25 ranking.
type BM25Index struct {
	docs     []docInfo
	inverted map[string][]posting
	docFreq  map[string]int
	avgDocLen float64
	mu       sync.RWMutex
}

var _ port.SearchIndex = (*BM25Index)(nil)

// NewBM25Index creates an empty BM25 search index.
func NewBM25Index() *BM25Index {
	return &BM25Index{
		inverted: make(map[string][]posting),
		docFreq:  make(map[string]int),
	}
}

// Build indexes all entries in-memory.
func (idx *BM25Index) Build(entries []entry.Entry) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	n := len(entries)
	idx.docs = make([]docInfo, n)
	idx.inverted = make(map[string][]posting)
	idx.docFreq = make(map[string]int)

	var totalLen float64

	for i := range n {
		e := entries[i]
		yr, _ := strconv.Atoi(e.Year)

		var topicsLC []string
		for _, t := range e.Topics {
			topicsLC = append(topicsLC, strings.ToLower(t))
		}

		info := docInfo{
			entry:       e,
			year:        yr,
			shortnameLC: strings.ToLower(e.Shortname),
			titleLC:     strings.ToLower(e.Title),
			authorLC:    strings.ToLower(strings.Join(e.Authors, " ")),
			topicsLC:    topicsLC,
		}

		// Weighted token frequencies
		termTFs := make(map[string]float64)
		inTitle := make(map[string]bool)
		inID := make(map[string]bool)

		// 1. Shortname tokens
		for _, tok := range Tokenize(e.Shortname) {
			termTFs[tok] += weightShortname
			inID[tok] = true
		}

		// 2. Title tokens
		for _, tok := range Tokenize(e.Title) {
			termTFs[tok] += weightTitle
			inTitle[tok] = true
		}

		// 3. Topics tokens
		for _, t := range e.Topics {
			for _, tok := range Tokenize(t) {
				termTFs[tok] += weightTopics
			}
		}

		// 4. Authors tokens
		for _, a := range e.Authors {
			for _, tok := range Tokenize(a) {
				termTFs[tok] += weightAuthors
			}
		}

		// 5. Abstract tokens
		abstractTokens := Tokenize(e.Abstract)
		for _, tok := range abstractTokens {
			termTFs[tok] += weightAbstract
		}

		docLen := float64(len(abstractTokens) + len(termTFs))
		info.docLen = docLen
		totalLen += docLen
		idx.docs[i] = info

		for term, tf := range termTFs {
			idx.inverted[term] = append(idx.inverted[term], posting{
				docID:   i,
				tf:      tf,
				inTitle: inTitle[term],
				inID:    inID[term],
			})
			idx.docFreq[term]++
		}
	}

	if n > 0 {
		idx.avgDocLen = totalLen / float64(n)
	}

	return nil
}

// Search executes BM25 search with topic, author, and year filtering.
func (idx *BM25Index) Search(q search.Query) (*search.Result, error) {
	start := time.Now()
	q.Normalize()

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	numDocs := len(idx.docs)
	if numDocs == 0 {
		return &search.Result{
			TotalHits:       0,
			Offset:          q.Offset,
			Limit:           q.Limit,
			Matches:         nil,
			TookMicrosecond: time.Since(start).Microseconds(),
		}, nil
	}

	rawText := strings.TrimSpace(q.Text)
	queryTokens := Tokenize(rawText)
	rawTextLC := strings.ToLower(rawText)

	scores := make(map[int]float64)

	if len(queryTokens) == 0 && rawText == "" {
		// No text query: return all documents matching filters
		for docID := range numDocs {
			if idx.matchesFilter(docID, q) {
				scores[docID] = 1.0
			}
		}
	} else {
		for _, qTerm := range queryTokens {
			df := idx.docFreq[qTerm]
			if df == 0 {
				// Try prefix match on shortnames and terms
				for term, postings := range idx.inverted {
					if strings.HasPrefix(term, qTerm) {
						idf := math.Log(1.0 + (float64(numDocs)-float64(idx.docFreq[term])+0.5)/(float64(idx.docFreq[term])+0.5))
						if idf < 0 {
							idf = 0.01
						}
						for _, p := range postings {
							if !idx.matchesFilter(p.docID, q) {
								continue
							}
							lenNorm := 1.0 - b + b*(idx.docs[p.docID].docLen/idx.avgDocLen)
							bm25Val := idf * ((p.tf * (k1 + 1.0)) / (p.tf + k1*lenNorm))
							scores[p.docID] += bm25Val * 0.7 // slight prefix discount
						}
					}
				}
				continue
			}

			// Standard BM25 IDF
			idf := math.Log(1.0 + (float64(numDocs)-float64(df)+0.5)/(float64(df)+0.5))
			if idf < 0 {
				idf = 0.01
			}

			for _, p := range idx.inverted[qTerm] {
				if !idx.matchesFilter(p.docID, q) {
					continue
				}
				lenNorm := 1.0 - b + b*(idx.docs[p.docID].docLen/idx.avgDocLen)
				bm25Val := idf * ((p.tf * (k1 + 1.0)) / (p.tf + k1*lenNorm))
				scores[p.docID] += bm25Val
			}
		}

		// Exact matches boost
		for docID := range scores {
			doc := &idx.docs[docID]
			if doc.shortnameLC == rawTextLC {
				scores[docID] += 100.0 // Direct exact entry name hit
			} else if strings.Contains(doc.shortnameLC, rawTextLC) {
				scores[docID] += 25.0
			}
			if strings.Contains(doc.titleLC, rawTextLC) {
				scores[docID] += 15.0
			}
		}
	}

	type docScore struct {
		docID int
		score float64
	}

	ranked := make([]docScore, 0, len(scores))
	for docID, score := range scores {
		ranked = append(ranked, docScore{docID: docID, score: score})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			// Newer entries first on tie
			return idx.docs[ranked[i].docID].year > idx.docs[ranked[j].docID].year
		}
		return ranked[i].score > ranked[j].score
	})

	totalHits := len(ranked)
	matches := make([]search.Match, 0, q.Limit)

	startIdx := q.Offset
	if startIdx < totalHits {
		endIdx := startIdx + q.Limit
		if endIdx > totalHits {
			endIdx = totalHits
		}

		for _, item := range ranked[startIdx:endIdx] {
			doc := idx.docs[item.docID]
			snippet := generateSnippet(doc.entry.Abstract, queryTokens)
			matches = append(matches, search.Match{
				Entry:   doc.entry,
				Score:   math.Round(item.score*100) / 100,
				Snippet: snippet,
			})
		}
	}

	return &search.Result{
		TotalHits:       totalHits,
		Offset:          q.Offset,
		Limit:           q.Limit,
		Matches:         matches,
		TookMicrosecond: time.Since(start).Microseconds(),
	}, nil
}

func (idx *BM25Index) matchesFilter(docID int, q search.Query) bool {
	doc := &idx.docs[docID]

	if q.YearFrom > 0 && doc.year < q.YearFrom {
		return false
	}
	if q.YearTo > 0 && doc.year > q.YearTo {
		return false
	}
	if q.Topic != "" {
		topicFilter := strings.ToLower(q.Topic)
		matched := false
		for _, t := range doc.topicsLC {
			if strings.Contains(t, topicFilter) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if q.Author != "" {
		authorFilter := strings.ToLower(q.Author)
		if !strings.Contains(doc.authorLC, authorFilter) {
			return false
		}
	}
	return true
}

func generateSnippet(abstract string, queryTerms []string) string {
	if abstract == "" {
		return ""
	}
	if len(queryTerms) == 0 {
		if len(abstract) > 160 {
			return strings.TrimSpace(abstract[:160]) + "..."
		}
		return abstract
	}

	sentences := strings.Split(abstract, ". ")
	bestSentence := sentences[0]
	maxHits := -1

	for _, s := range sentences {
		sLC := strings.ToLower(s)
		hits := 0
		for _, q := range queryTerms {
			if strings.Contains(sLC, q) {
				hits++
			}
		}
		if hits > maxHits {
			maxHits = hits
			bestSentence = s
		}
	}

	bestSentence = strings.TrimSpace(bestSentence)
	if !strings.HasSuffix(bestSentence, ".") {
		bestSentence += "."
	}
	if len(bestSentence) > 200 {
		return strings.TrimSpace(bestSentence[:197]) + "..."
	}
	return bestSentence
}
