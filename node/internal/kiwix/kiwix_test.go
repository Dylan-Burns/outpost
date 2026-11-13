package kiwix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const feed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/">
  <channel>
    <opensearch:totalResults>1,553</opensearch:totalResults>
    <item>
      <title>Organic &amp; <b>acid</b></title>
      <link>/content/chem_2026-10/Organic_acid</link>
      <description>An organic <b>acid</b> is an organic compound with &lt;acidic&gt; properties.</description>
      <book><title>Chemistry by Wikipedia</title></book>
    </item>
    <item><title>Outside</title><link>http://evil.example/x</link><description>x</description></item>
  </channel>
</rss>`

func TestSearchParsesHighlights(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.URL.Query().Get("pattern") != "acid" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(feed))
	}))
	defer srv.Close()
	port, _ := strconv.Atoi(srv.URL[strings.LastIndexByte(srv.URL, ':')+1:])
	m := &Manager{Port: port}
	res, total, err := m.Search(context.Background(), "acid", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1553 || len(res) != 1 {
		t.Fatalf("total %d, %d results", total, len(res))
	}
	if res[0].Title != "Organic & acid" || !strings.Contains(res[0].Snippet, "organic acid is") || !strings.Contains(res[0].Snippet, "<acidic>") {
		t.Fatalf("got %+v", res[0])
	}
	if res[0].Book != "Chemistry by Wikipedia" {
		t.Fatalf("book %q", res[0].Book)
	}
}

func TestHTMLToText(t *testing.T) {
	got := HTMLToText(`<html><head><title>x</title><script>alert(1)</script></head><body><h1>Water</h1><p>Boil &amp; cool.</p><style>p{}</style><p>ok</p></body></html>`, 1000)
	if strings.Contains(got, "alert") || !strings.Contains(got, "Boil & cool.") || !strings.Contains(got, "Water") {
		t.Fatalf("got %q", got)
	}
}
