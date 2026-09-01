package memory

import (
	"math"
	"sort"
	"strings"
)

// TfIdfScorer lite: cosine similarity without vector DB, sync.Map per bot_id not needed for now (pure function)
type Scorer struct{}

func NewScorer() *Scorer { return &Scorer{} }

// Score returns cosine similarity between query and doc (0-1)
func (s *Scorer) Score(query, doc string) float64 {
	qTokens := tokenize(query)
	dTokens := tokenize(doc)
	if len(qTokens) == 0 || len(dTokens) == 0 {
		return 0
	}
	// term frequency
	qTf := termFreq(qTokens)
	dTf := termFreq(dTokens)
	// vocab
	vocab := make(map[string]bool)
	for k := range qTf {
		vocab[k] = true
	}
	for k := range dTf {
		vocab[k] = true
	}
	// idf: log(N/df) where N=2, df 1 or 2
	// For 2 docs, idf = log(2/df)
	var dot, normQ, normD float64
	for term := range vocab {
		df := 0
		if _, ok := qTf[term]; ok {
			df++
		}
		if _, ok := dTf[term]; ok {
			df++
		}
		idf := math.Log(2.0 / float64(df))
		// if df=2, idf=0 => common terms ignored, keep at least small value
		if idf == 0 {
			idf = 0.1
		}
		qv := float64(qTf[term]) * idf
		dv := float64(dTf[term]) * idf
		dot += qv * dv
		normQ += qv * qv
		normD += dv * dv
	}
	if normQ == 0 || normD == 0 {
		return 0
	}
	return dot / (math.Sqrt(normQ) * math.Sqrt(normD))
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	// simple split
	fields := strings.Fields(s)
	// filter stopwords small
	stop := map[string]bool{"the": true, "a": true, "an": true, "and": true, "or": true, "is": true, "are": true, "di": true, "dan": true, "yang": true, "ini": true, "itu": true}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, ".,!?:;\"'()[]")
		if f == "" || stop[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

func termFreq(tokens []string) map[string]int {
	m := make(map[string]int)
	for _, t := range tokens {
		m[t]++
	}
	return m
}

// SelectRelevant returns top memories relevant to query, within token budget, fallback recent 5 if score < threshold
func SelectRelevant(query string, memories []string, tokenBudget int, scorer *Scorer) []string {
	if len(memories) == 0 {
		return nil
	}
	if scorer == nil {
		scorer = NewScorer()
	}
	type scored struct {
		text  string
		score float64
		idx   int
	}
	scoredList := make([]scored, 0, len(memories))
	for i, m := range memories {
		scoredList = append(scoredList, scored{text: m, score: scorer.Score(query, m), idx: i})
	}
	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].score > scoredList[j].score })
	// if top score < 0.15, fallback recent 5
	if len(scoredList) > 0 && scoredList[0].score < 0.15 {
		if len(memories) <= 5 {
			return memories
		}
		return memories[len(memories)-5:]
	}
	// greedy by budget (estimate tokens via heuristic chars/4)
	budget := tokenBudget
	if budget <= 0 {
		budget = 500
	}
	var out []string
	used := 0
	for _, s := range scoredList {
		cost := (len(s.text) + 3) / 4
		if used+cost > budget {
			continue
		}
		out = append(out, s.text)
		used += cost
	}
	if len(out) == 0 && len(scoredList) > 0 {
		out = []string{scoredList[0].text}
	}
	return out
}
