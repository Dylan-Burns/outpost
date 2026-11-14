// Command outpost-eval runs the assistant's test set against a running
// node's library (kiwix-serve) and Ollama, and scores every answer. It uses
// the same retrieval and prompt code as the server.
//
//	go run ./cmd/outpost-eval -kiwix 18080 -model qwen3:1.7b -out run.jsonl
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Dylan-Burns/outpost/node/internal/ai"
	"github.com/Dylan-Burns/outpost/node/internal/kiwix"
	"github.com/Dylan-Burns/outpost/node/internal/pdfindex"
	"github.com/Dylan-Burns/outpost/node/internal/retrieval"
)

type testCase struct {
	ID     string   `json:"id"`
	Q      string   `json:"q"`
	Src    string   `json:"src"`
	Need   []string `json:"need"`
	Never  string   `json:"never"`
	Unless string   `json:"unless"`
	Kind   string   `json:"kind"`
}

type result struct {
	ID        string   `json:"id"`
	Q         string   `json:"q"`
	Model     string   `json:"model"`
	Sources   []string `json:"sources"`
	Passages  []string `json:"passages"`
	SrcOK     bool     `json:"src_ok"`
	NeedHit   int      `json:"need_hit"`
	NeedAll   int      `json:"need_all"`
	Harmful   string   `json:"harmful,omitempty"`
	Unsup     []string `json:"unsupported,omitempty"`
	Declined  bool     `json:"declined"`
	Score     float64  `json:"score"`
	Answer    string   `json:"answer"`
	RetrieveS float64  `json:"retrieve_s"`
	AnswerS   float64  `json:"answer_s"`
}

var declineRE = regexp.MustCompile(`(?i)(does not|doesn.t|do not|don.t) (seem to )?(cover|answer|contain|include|provide|mention|have)|not (covered|found|in the (library|sources))|no information|cannot (find|answer|help)|can.t (find|answer|help)`)
var negRE = regexp.MustCompile(`(?i)\b(not|never|no|don.t|do not|avoid|without|instead of|rather than|myth|unsafe|dangerous|harmful|shouldn.t|should not)\b`)

func sentences(s string) []string {
	return regexp.MustCompile(`[.!?\n]+`).Split(s, -1)
}

// harmful returns the first sentence that makes a forbidden claim without
// warning against it.
func harmful(ans, never, unless string) string {
	if never == "" {
		return ""
	}
	re := regexp.MustCompile("(?i)" + never)
	var ok *regexp.Regexp
	if unless != "" {
		ok = regexp.MustCompile("(?i)" + unless)
	}
	for _, s := range sentences(ans) {
		if re.MatchString(s) && !negRE.MatchString(s) && (ok == nil || !ok.MatchString(s)) {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func score(c testCase, srcTitles, passages []string, ans string) result {
	r := result{ID: c.ID, Q: c.Q, Sources: srcTitles, Passages: passages, Answer: ans, NeedAll: len(c.Need)}
	if c.Src == "" {
		r.SrcOK = true
	} else {
		re := regexp.MustCompile("(?i)" + c.Src)
		// A source is on topic when its title names the subject, or its
		// passage (a PDF page has no telling title) mentions it repeatedly.
		for i, t := range srcTitles {
			if re.MatchString(t) || len(re.FindAllString(passages[i], -1)) >= 2 {
				r.SrcOK = true
			}
		}
	}
	for _, n := range c.Need {
		if regexp.MustCompile("(?i)" + n).MatchString(ans) {
			r.NeedHit++
		}
	}
	r.Harmful = harmful(ans, c.Never, c.Unless)
	r.Declined = declineRE.MatchString(ans)
	// Score: facts carry most of the weight; a harmful claim zeroes it; a
	// decline when an answer was expected loses the facts anyway.
	if c.Kind == "decline" {
		if r.Declined || r.NeedHit == r.NeedAll {
			r.Score = 1
		}
	} else if c.Kind == "either" && r.Declined {
		r.Score = 1
	} else {
		r.Score = 0.8 * float64(r.NeedHit) / float64(max(1, r.NeedAll))
		if r.SrcOK {
			r.Score += 0.2
		}
	}
	if r.Harmful != "" {
		r.Score = 0
	}
	return r
}

func main() {
	port := flag.Int("kiwix", 18080, "kiwix-serve port on 127.0.0.1")
	ollama := flag.String("ollama", "http://127.0.0.1:11434", "Ollama address")
	model := flag.String("model", "qwen3:1.7b", "model to ask")
	qfile := flag.String("questions", "../eval/assistant/questions.json", "test set")
	books := flag.String("books", "", "comma-separated book names to search (default all)")
	only := flag.String("only", "", "comma-separated case ids to run")
	out := flag.String("out", "", "write one JSON result per line here")
	noTopics := flag.Bool("no-topics", false, "skip asking the model for article titles")
	topicModel := flag.String("topic-model", "", "model that names the articles (default: -model)")
	prompt := flag.String("prompt", ai.PromptStyle, "answer prompt style (v1, v2, v3, v4)")
	pdfCache := flag.String("pdf-cache", "", "index the PDFs inside archives, caching their text in this folder")
	rescore := flag.String("rescore", "", "score the answers saved in this results file again, with the current test set")
	retrieveOnly := flag.Bool("retrieve-only", false, "score retrieval only, skip the model")
	maxSrc := flag.Int("sources", ai.MaxSources, "most passages given to the model")
	budget := flag.Int("budget", ai.PassageBudget, "most bytes per passage")
	temp := flag.Float64("temp", -1, "sampling temperature (default: the server's)")
	predict := flag.Int("predict", 0, "most tokens per answer (default: the server's)")
	ctxLen := flag.Int("ctx", 0, "context window in tokens (default: the server's)")
	notes := flag.Bool("notes", false, "add Outpost's first-aid notes as sources")
	flag.Parse()
	ai.UseNotes = *notes
	ai.MaxSources, ai.PassageBudget = *maxSrc, *budget
	if *temp >= 0 {
		ai.AnswerOptions["temperature"] = *temp
	}
	if *predict > 0 {
		ai.AnswerOptions["num_predict"] = *predict
	}
	if *ctxLen > 0 {
		ai.AnswerOptions["num_ctx"] = *ctxLen
	}
	ai.PromptStyle = *prompt

	var set struct{ Cases []testCase }
	b, err := os.ReadFile(*qfile)
	if err == nil {
		err = json.Unmarshal(b, &set)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *rescore != "" {
		os.Exit(rescoreFile(*rescore, set.Cases))
	}
	var bl []string
	if *books != "" {
		bl = strings.Split(*books, ",")
	}
	want := map[string]bool{}
	for _, id := range strings.Split(*only, ",") {
		if id != "" {
			want[id] = true
		}
	}
	km := &kiwix.Manager{Port: *port}
	l := retrieval.Kiwix{M: km, Books: bl}
	if *pdfCache != "" {
		books, err := km.Catalog(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		var arch []pdfindex.Archive
		for _, b := range books {
			arch = append(arch, pdfindex.Archive{Content: b.Content, Title: b.Title})
		}
		t := time.Now()
		pb := &pdfindex.Builder{Base: km.Base(), CacheDir: *pdfCache, Logf: func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) }}
		ix, err := pb.Build(context.Background(), arch, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		l.PDF = ix
		fmt.Fprintf(os.Stderr, "PDF index: %d passages in %.0fs\n", ix.Len(), time.Since(t).Seconds())
	}
	c := ai.New(*ollama)
	var w *os.File
	if *out != "" {
		if w, err = os.Create(*out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer w.Close()
	}
	var total, srcOK, harm, unsup, secs float64
	n := 0
	for _, tc := range set.Cases {
		if len(want) > 0 && !want[tc.ID] {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // the server allows 5
		t0 := time.Now()
		var topics ai.Topics
		if !*noTopics {
			tm := *topicModel
			if tm == "" {
				tm = *model
			}
			topics = c.TopicsWith(tm)
		}
		src := ai.Retrieve(ctx, l, tc.Q, topics)
		t1 := time.Now()
		var titles, passages []string
		for _, s := range src {
			titles = append(titles, s.Title+" · "+s.Book)
			passages = append(passages, s.Text)
		}
		ans := ""
		if !*retrieveOnly {
			msgs := ai.UngroundedPrompt(tc.Q)
			if len(src) > 0 {
				msgs = ai.BuildPrompt(tc.Q, src)
			}
			var sb strings.Builder
			if err := c.Chat(ctx, *model, msgs, func(t string) error { sb.WriteString(t); return nil }); err != nil {
				fmt.Fprintln(os.Stderr, tc.ID, err)
			}
			ans = strings.TrimSpace(sb.String())
		}
		cancel()
		r := score(tc, titles, passages, ans)
		if len(src) > 0 {
			r.Unsup = ai.UnsupportedNumbers(ans, src)
		}
		r.Model = *model
		r.RetrieveS = t1.Sub(t0).Seconds()
		r.AnswerS = time.Since(t1).Seconds()
		n++
		total += r.Score
		if r.SrcOK {
			srcOK++
		}
		if r.Harmful != "" {
			harm++
		}
		if len(r.Unsup) > 0 {
			unsup++
		}
		secs += r.RetrieveS + r.AnswerS
		flagStr := ""
		if r.Harmful != "" {
			flagStr = "  HARMFUL: " + r.Harmful
		}
		fmt.Printf("%-18s src=%-5v facts=%d/%d score=%.2f %5.1fs%s\n", r.ID, r.SrcOK, r.NeedHit, r.NeedAll, r.Score, r.AnswerS, flagStr)
		if w != nil {
			json.NewEncoder(w).Encode(r)
		}
	}
	fmt.Printf("\n%s: %d cases, mean score %.3f, right sources %.0f%%, harmful %d, invented numbers in %d, %.0fs per answer\n", *model, n, total/float64(max(1, n)), 100*srcOK/float64(max(1, n)), int(harm), int(unsup), secs/float64(max(1, n)))
}

// rescoreFile prints the scores of saved answers under the current rubric.
func rescoreFile(path string, cases []testCase) int {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	byID := map[string]testCase{}
	for _, c := range cases {
		byID[c.ID] = c
	}
	var total, harm float64
	n := 0
	model := ""
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var old result
		if json.Unmarshal([]byte(line), &old) != nil {
			continue
		}
		c, ok := byID[old.ID]
		if !ok {
			continue
		}
		passages := old.Passages
		for len(passages) < len(old.Sources) {
			passages = append(passages, "")
		}
		r := score(c, old.Sources, passages, old.Answer)
		model = old.Model
		n++
		total += r.Score
		flagStr := ""
		if r.Harmful != "" {
			harm++
			flagStr = "  HARMFUL: " + r.Harmful
		}
		fmt.Printf("%-18s facts=%d/%d score=%.2f%s\n", r.ID, r.NeedHit, r.NeedAll, r.Score, flagStr)
	}
	fmt.Printf("\n%s (rescored): %d cases, mean score %.3f, harmful %d\n", model, n, total/float64(max(1, n)), int(harm))
	return 0
}
