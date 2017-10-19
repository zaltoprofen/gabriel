package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Config struct {
	SumFilePath string
	Token       string
	EventName   string
	Url         string
	Selector    string
	UserAgent   string
}

type IFTTTPostForm struct {
	Value1 string `json:"value1"`
	Value2 string `json:"value2"`
	Value3 string `json:"value3"`
}

var (
	client  = &http.Client{}
	verbose = flag.Bool("verbose", false, "show log message")
)

func printLog(msg ...interface{}) {
	if *verbose {
		log.Println(msg...)
	}
}

func main() {
	flag.Parse()
	exitCode := _main()
	printLog("exit gabriel:", exitCode)
	os.Exit(exitCode)
}

func _main() int {
	if len(flag.Args()) != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s config.json\n", os.Args[0])
		return 1
	}
	printLog("start gabriel")
	c, err := readConfig(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "readConfig:", err)
		return 1
	}
	printLog("read configuration:", flag.Arg(0))
	sum, err := getCurrentSum(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		onError(c, err.Error())
		printLog("notified onError event:", err)
		return 1
	}
	printLog("got current target sum:", hex.EncodeToString(sum))
	prevSum, err := readPrevSum(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		onError(c, err.Error())
		printLog("notified onError event:", err)
		return 1
	}
	printLog("loaded previous target sum:", hex.EncodeToString(sum))
	if !bytes.Equal(sum, prevSum) {
		if err := onChange(c); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		printLog("notified onChange event")
	}
	if err := writeBackSum(c, sum); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		onError(c, err.Error())
		printLog("notified onError event:", err)
		return 1
	}
	return 0
}

func readConfig(path string) (*Config, error) {
	conf := new(Config)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, conf); err != nil {
		return nil, err
	}
	return conf, nil
}

func onChange(c *Config) error {
	if err := fireIFTTT(c, &IFTTTPostForm{
		"Change",
		c.Url,
		"",
	}); err != nil {
		return err
	}
	return nil
}

func onError(c *Config, errMsg string) error {
	if err := fireIFTTT(c, &IFTTTPostForm{
		"Error",
		"",
		errMsg,
	}); err != nil {
		return err
	}
	return nil
}

func fireIFTTT(c *Config, form *IFTTTPostForm) error {
	data, err := json.Marshal(form)
	if err != nil {
		return err
	}
	body := strings.NewReader(string(data))
	reqURL := fmt.Sprintf("http://maker.ifttt.com/trigger/%s/with/key/%s", c.EventName, c.Token)
	r, err := client.Post(reqURL, "application/json", body)
	if err != nil {
		return err
	}
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("return status code is not ok: %d", r.StatusCode)
	}
	return nil
}

func readPrevSum(c *Config) ([]byte, error) {
	if _, err := os.Stat(c.SumFilePath); err != nil {
		return []byte{}, nil
	}
	sumStr, err := ioutil.ReadFile(c.SumFilePath)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(string(sumStr))
}

func writeBackSum(c *Config, sum []byte) error {
	sumStr := hex.EncodeToString(sum)
	return ioutil.WriteFile(c.SumFilePath, []byte(sumStr), 0644)
}

func getCurrentSum(c *Config) ([]byte, error) {
	s, err := scrape(c.Url, c.Selector, c.UserAgent)
	if err != nil {
		return nil, err
	}
	printLog("scrape:", s)
	sum := sha1.Sum([]byte(s))
	return sum[:], nil
}

func scrape(url, selector, userAgent string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}
	return goquery.OuterHtml(doc.Find(selector))
}
