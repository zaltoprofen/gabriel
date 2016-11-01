package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
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
}

type IFTTTPostForm struct {
	Value1 string `json:"value1"`
	Value2 string `json:"value2"`
	Value3 string `json:"value3"`
}

func main() {
	os.Exit(_main())
}

func _main() int {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s config.json\n", os.Args[0])
		return 1
	}
	c, err := readConfig(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "readConfig:", err)
		return 1
	}
	sum, err := getCurrentSum(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		onError(c, err.Error())
		return 1
	}
	prevSum, err := readPrevSum(c)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		onError(c, err.Error())
		return 1
	}
	if !bytes.Equal(sum, prevSum) {
		if err := onChange(c); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
	}
	if err := writeBackSum(c, sum); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		onError(c, err.Error())
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
	reqUrl := fmt.Sprintf("http://maker.ifttt.com/trigger/%s/with/key/%s", c.EventName, c.Token)
	r, err := http.Post(reqUrl, "application/json", body)
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
	s, err := scrape(c.Url, c.Selector)
	if err != nil {
		return nil, err
	}
	sum := sha1.Sum([]byte(s))
	return sum[:], nil
}

func scrape(url, selector string) (string, error) {
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return "", err
	}
	return goquery.OuterHtml(doc.Find(selector))
}
