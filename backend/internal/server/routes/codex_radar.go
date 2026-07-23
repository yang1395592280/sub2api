package routes

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

const (
	codexRadarURL              = "https://codexradar.com/"
	codexRadarMaxResponseBytes = 2 << 20
	codexRadarCacheTTL         = 5 * time.Minute
)

var codexRadarReadOnlyAPIPaths = map[string]struct{}{
	"/model-ratings":    {},
	"/subscriber-count": {},
}

type codexRadarProxy struct {
	client      *http.Client
	upstreamURL *url.URL
	cacheTTL    time.Duration

	mu         sync.Mutex
	cachedHTML []byte
	cachedAt   time.Time
}

func newCodexRadarProxy(client *http.Client, upstreamURL string) (*codexRadarProxy, error) {
	parsedURL, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("parse Codex Radar URL: %w", err)
	}
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return nil, fmt.Errorf("unsupported Codex Radar URL scheme %q", parsedURL.Scheme)
	}
	return &codexRadarProxy{
		client:      client,
		upstreamURL: parsedURL,
		cacheTTL:    codexRadarCacheTTL,
	}, nil
}

func newDefaultCodexRadarProxy() *codexRadarProxy {
	upstream, err := url.Parse(codexRadarURL)
	if err != nil {
		panic(err)
	}
	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if req.URL.Scheme != upstream.Scheme || !strings.EqualFold(req.URL.Host, upstream.Host) {
				return fmt.Errorf("Codex Radar redirected to an unexpected origin")
			}
			return nil
		},
	}
	proxy, err := newCodexRadarProxy(client, codexRadarURL)
	if err != nil {
		panic(err)
	}
	return proxy
}

func (p *codexRadarProxy) serveEmbed(c *gin.Context) {
	htmlBody, err := p.embeddedHTML(c.Request)
	if err != nil {
		log.Printf("[CodexRadar] failed to load upstream page: %v", err)
		p.setEmbedHeaders(c)
		c.Data(http.StatusBadGateway, "text/html; charset=utf-8", []byte(codexRadarErrorPage))
		return
	}

	p.setEmbedHeaders(c)
	c.Header("Cache-Control", "private, max-age=60")
	c.Data(http.StatusOK, "text/html; charset=utf-8", htmlBody)
}

func (p *codexRadarProxy) embeddedHTML(request *http.Request) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.cachedHTML) > 0 && time.Since(p.cachedAt) < p.cacheTTL {
		return p.cachedHTML, nil
	}

	upstreamRequest, err := http.NewRequestWithContext(request.Context(), http.MethodGet, p.upstreamURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}
	upstreamRequest.Header.Set("Accept", "text/html,application/xhtml+xml")
	upstreamRequest.Header.Set("User-Agent", "sub2api-codex-radar/1.0")

	response, err := p.client.Do(upstreamRequest)
	if err != nil {
		if len(p.cachedHTML) > 0 {
			return p.cachedHTML, nil
		}
		return nil, fmt.Errorf("request upstream: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned HTTP %d", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(strings.ToLower(contentType), "text/html") {
		return nil, fmt.Errorf("upstream returned unexpected content type %q", contentType)
	}

	body, err := readLimitedBody(response.Body)
	if err != nil {
		return nil, err
	}
	transformed, err := transformCodexRadarHTML(body)
	if err != nil {
		return nil, err
	}

	p.cachedHTML = transformed
	p.cachedAt = time.Now()
	return p.cachedHTML, nil
}

func (p *codexRadarProxy) serveReadOnlyAPI(c *gin.Context) {
	path := c.Param("path")
	if _, ok := codexRadarReadOnlyAPIPaths[path]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Codex Radar resource not found"})
		return
	}

	upstreamURL := p.upstreamURL.ResolveReference(&url.URL{
		Path:     "/api" + path,
		RawQuery: c.Request.URL.RawQuery,
	})
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, upstreamURL.String(), nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to load Codex Radar data"})
		return
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "sub2api-codex-radar/1.0")

	response, err := p.client.Do(request)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to load Codex Radar data"})
		return
	}
	defer response.Body.Close()

	body, err := readLimitedBody(response.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Invalid Codex Radar response"})
		return
	}

	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Cache-Control", "private, max-age=60")
	c.Data(response.StatusCode, "application/json; charset=utf-8", body)
}

func (p *codexRadarProxy) setEmbedHeaders(c *gin.Context) {
	// The iframe is same-site at the transport layer, but its sandbox gives the
	// remote scripts an opaque origin so they cannot access the parent app.
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Content-Security-Policy", strings.Join([]string{
		"default-src https://codexradar.com data: blob:",
		"script-src 'unsafe-inline' https://codexradar.com https://static.cloudflareinsights.com",
		"style-src 'unsafe-inline' https://codexradar.com",
		"img-src https: data: blob:",
		"connect-src https://codexradar.com 'self'",
		"font-src https://codexradar.com data:",
		"base-uri https://codexradar.com",
		"form-action https://codexradar.com",
		"frame-ancestors 'self'",
	}, "; "))
}

func readLimitedBody(reader io.Reader) ([]byte, error) {
	limited := io.LimitReader(reader, codexRadarMaxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read Codex Radar response: %w", err)
	}
	if len(body) > codexRadarMaxResponseBytes {
		return nil, fmt.Errorf("Codex Radar response exceeds %d bytes", codexRadarMaxResponseBytes)
	}
	return body, nil
}

func transformCodexRadarHTML(source []byte) ([]byte, error) {
	document, err := html.Parse(bytes.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("parse Codex Radar HTML: %w", err)
	}

	var head *html.Node
	var removableNodes []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			if node.Data == "head" {
				head = node
			}
			shouldRemove := false
			for _, attribute := range node.Attr {
				if attribute.Key == "id" && attribute.Val == "codex-community" {
					shouldRemove = true
				}
				if attribute.Key == "class" {
					for _, className := range strings.Fields(attribute.Val) {
						if className == "site-announcement-hint" || className == "site-announcement-actions" {
							shouldRemove = true
							break
						}
					}
				}
			}
			if shouldRemove {
				removableNodes = append(removableNodes, node)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)

	if head == nil {
		return nil, fmt.Errorf("Codex Radar HTML has no head element")
	}
	for _, node := range removableNodes {
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
		}
	}

	base := &html.Node{
		Type: html.ElementNode,
		Data: "base",
		Attr: []html.Attribute{{Key: "href", Val: codexRadarURL}},
	}
	head.InsertBefore(base, head.FirstChild)

	script := &html.Node{Type: html.ElementNode, Data: "script"}
	script.AppendChild(&html.Node{Type: html.TextNode, Data: codexRadarFetchBridge})
	head.InsertBefore(script, base.NextSibling)

	style := &html.Node{Type: html.ElementNode, Data: "style"}
	style.AppendChild(&html.Node{Type: html.TextNode, Data: "#codex-community,.site-announcement-hint,.site-announcement-actions{display:none!important}"})
	head.AppendChild(style)

	var output bytes.Buffer
	if err := html.Render(&output, document); err != nil {
		return nil, fmt.Errorf("render Codex Radar HTML: %w", err)
	}
	return output.Bytes(), nil
}

const codexRadarFetchBridge = `(function(){
	try{window.localStorage.getItem("__codex_radar_probe__");}catch(error){
	    var memory={};
	    try{Object.defineProperty(window,"localStorage",{value:{
	      getItem:function(key){return Object.prototype.hasOwnProperty.call(memory,key)?memory[key]:null;},
	      setItem:function(key,value){memory[key]=String(value);},
	      removeItem:function(key){delete memory[key];},
	      clear:function(){memory={};}
	    }});}catch(storageError){}
  }
  try{
    if(!window.localStorage.getItem("codex_radar_theme")){
      window.localStorage.setItem("codex_radar_theme","light");
    }
  }catch(themeStorageError){}
  var originalFetch=window.fetch.bind(window);
  var localProxy=new URL("/api/v1/codex-radar/upstream",window.location.href).href;
  function rewrite(value){
    var parsed=new URL(String(value),document.baseURI);
    if(parsed.pathname==="/api/model-ratings"||parsed.pathname==="/api/subscriber-count"){
      return localProxy+parsed.pathname.slice(4)+parsed.search;
    }
    return value;
  }
  window.fetch=function(input,init){
    if(typeof input==="string"||input instanceof URL){
      return originalFetch(rewrite(input),init);
    }
    if(input instanceof Request){
      return originalFetch(new Request(rewrite(input.url),input),init);
    }
    return originalFetch(input,init);
  };
})();`

const codexRadarErrorPage = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Codex 雷达</title><style>html,body{height:100%;margin:0}body{display:grid;place-items:center;background:#0d1420;color:#e5edf7;font:15px system-ui,sans-serif}.message{text-align:center;padding:24px}.message strong{display:block;font-size:18px;margin-bottom:8px}.message span{color:#a7b2c3}</style></head><body><div class="message"><strong>Codex 雷达暂时无法加载</strong><span>请稍后刷新页面重试</span></div></body></html>`
