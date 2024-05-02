package minnano

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/antchfx/htmlquery"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"golang.org/x/net/html"
)

type actressCrawlerRepo struct {
	baseURL                       string
	actressCrawlerDataProviderMap map[string]domain.ActressCrawlerProvider
}

func CreateActressCrawlerRepo(baseURL string) domain.ActressCrawlerRepo {
	a := &actressCrawlerRepo{
		baseURL:                       baseURL,
		actressCrawlerDataProviderMap: make(map[string]domain.ActressCrawlerProvider),
	}

	return a
}

// GetActresses
// limit is not work, because always get 30 actresses per page in mininano web
func (a *actressCrawlerRepo) GetActresses(page, limit int) (*domain.ActressCrawlerDataPagination, error) {
	latPageNum, err := getLastPageNum(a.baseURL + "/actress_list.php")
	if err != nil {
		return nil, errors.Wrap(err, "get last page num failed")
	}

	if page > latPageNum {
		return nil, errors.Wrap(domain.ErrNoData, "page greater last page")
	}

	pageCrawler := createPageCrawler(a.baseURL)

	path := "actress_list.php"
	if page > 0 {
		path = path + "?page=" + strconv.Itoa(page)
	}

	actresses, err := pageCrawler.crawlerActress(path)
	if err != nil {
		return nil, errors.Wrap(err, "crawler actress failed")
	}

	return &domain.ActressCrawlerDataPagination{
		Count:       len(actresses),
		CurrentPage: page,
		TotalPages:  latPageNum,
		Items:       actresses,
	}, nil
}

type actress struct {
	Name           string
	ImageURL       string
	ImageType      domain.ImageType
	Tags           []string
	VideoCount     int
	FreeVideoCount int
}

type actressImage struct {
	rawData []byte
}

func createActressImage(rawData []byte) domain.ImageGetter {
	return &actressImage{
		rawData: rawData,
	}
}

func (a *actressImage) GetImageArea() (float64, error) {
	m, _, err := image.Decode(bytes.NewBuffer(a.rawData))
	if err != nil {
		return 0, errors.Wrap(err, "image decode failed")
	}
	g := m.Bounds()

	height := g.Dy()
	width := g.Dx()

	resolution := height * width

	return float64(resolution), nil
}

func (a *actressImage) GetRawData() []byte {
	return a.rawData
}

func (a *actress) GetWithValid() (*domain.ActressCrawlerData, error) {
	if a.ImageType != domain.ImageTypePNG && a.ImageType != domain.ImageTypeJPG {
		return nil, errors.New("error image type: " + string(a.ImageType))
	} else if strings.Contains(a.Name, "(") || strings.Contains(a.Name, ")") ||
		strings.Contains(a.Name, "（") || strings.Contains(a.Name, "）") ||
		strings.Contains(a.Name, "【") || strings.Contains(a.Name, "】") {
		return nil, errors.New("error name: " + string(a.ImageType))
	}

	return &domain.ActressCrawlerData{
		ActressName:       a.Name,
		ActressPreviewURL: a.ImageURL,
		PreviewImageType:  a.ImageType,
	}, nil
}

func (a *actress) GetImage() (domain.ImageGetter, error) {
	url := a.ImageURL
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request failed")
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do request failed")
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "read body failed")
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("read body failed, body: %s", string(body)))
	}

	return createActressImage(body), nil
}

type page struct {
	baseURL string
}

func createPageCrawler(baseURL string) *page {
	return &page{
		baseURL: baseURL,
	}
}

func (p *page) crawlerActress(path string) ([]domain.ActressCrawlerProvider, error) {
	htmlNode, err := htmlquery.LoadURL(p.baseURL + "/" + path)
	if err != nil {
		return nil, errors.Wrap(err, "load html failed")
	}
	nodes, err := htmlquery.QueryAll(htmlNode, `//*[@id="main-area"]/section/table/tbody/tr[*]`)
	if err != nil {
		return nil, errors.Wrap(err, "query html fail")
	}
	// first tr is title, no need
	actresses := make([]domain.ActressCrawlerProvider, len(nodes)-1)
	for idx, node := range nodes[1:] {
		var (
			actressItem actress
			elementNum  int
		)
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode {
				switch elementNum {
				case 0:
					imageURL, err := p.getImageURL(child)
					if err != nil {
						return nil, errors.Wrap(err, "get image url failed")
					}
					actressItem.ImageURL = imageURL
					actressItem.ImageType = domain.ImageTypeUnknown
					imageType := imageURL[strings.LastIndex(imageURL, ".")+1:]
					if imageType == "jpeg" || imageType == "jpg" {
						actressItem.ImageType = domain.ImageTypeJPG
					} else if imageType == "png" {
						actressItem.ImageType = domain.ImageTypePNG
					} else if imageType == "gif" {
						actressItem.ImageType = domain.ImageTypeGIF
					}
				case 1:
					name, err := p.getName(child)
					if err != nil {
						return nil, errors.Wrap(err, "get name failed")
					}
					actressItem.Name = name
					tags, err := p.getTags(child)
					if err != nil {
						return nil, errors.Wrap(err, "get tags failed")
					}
					actressItem.Tags = tags
				case 2:
					videoCount, err := p.getVideoCount(child)
					if err != nil {
						return nil, errors.Wrap(err, "get video count failed")
					}
					actressItem.VideoCount = videoCount
				case 3:
					freeVideoCount, err := p.getFreeVideoCount(child)
					if err != nil {
						return nil, errors.Wrap(err, "get free video count failed")
					}
					actressItem.FreeVideoCount = freeVideoCount
				}
				elementNum++
			}
		}
		actresses[idx] = &actressItem
	}
	return actresses, nil
}

func (p *page) getImageURL(htmlNode *html.Node) (string, error) {
	if htmlNode.FirstChild != nil && htmlNode.FirstChild.Type == html.ElementNode &&
		htmlNode.FirstChild.FirstChild != nil && htmlNode.FirstChild.FirstChild.Type == html.ElementNode {
		return p.baseURL + "/" + htmlNode.FirstChild.FirstChild.Attr[0].Val, nil
	}
	return "", errors.New("get image url failed")
}

func (p *page) getName(htmlNode *html.Node) (string, error) {
	for child := htmlNode.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "h2" &&
			child.FirstChild != nil && child.FirstChild.FirstChild != nil {
			return child.FirstChild.FirstChild.Data, nil
		}
	}
	return "", errors.New("get name failed")
}

func (p *page) getTags(htmlNode *html.Node) ([]string, error) {
	for child := htmlNode.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "div" {
			var tags []string
			for child2 := child.FirstChild; child2 != nil; child2 = child2.NextSibling {
				if child2.Type == html.ElementNode && child2.FirstChild != nil {
					tags = append(tags, child2.FirstChild.Data)
				}
			}
			return tags, nil
		}
	}
	return nil, errors.New("get tags failed")
}

func (p *page) getVideoCount(htmlNode *html.Node) (int, error) {
	if htmlNode.FirstChild != nil {
		count, err := strconv.Atoi(htmlNode.FirstChild.Data)
		if err != nil {
			return 0, errors.Wrap(err, "cover string to int failed")
		}
		return count, nil
	}
	return 0, errors.New("get count failed")
}

func (p *page) getFreeVideoCount(htmlNode *html.Node) (int, error) {
	if htmlNode.FirstChild != nil {
		count, err := strconv.Atoi(htmlNode.FirstChild.Data)
		if err != nil {
			return 0, errors.Wrap(err, "cover string to int failed")
		}
		return count, nil
	}
	return 0, errors.New("get count failed")
}

func getLastPageNum(url string) (int, error) {
	htmlNode, err := htmlquery.LoadURL(url)
	if err != nil {
		return 0, errors.Wrap(err, "load html failed")
	}
	node, err := htmlquery.Query(htmlNode, `//*[@id="main-area"]/section/div/b`)
	if err != nil {
		return 0, errors.Wrap(err, "query html fail")
	}
	lastPageNumRawData := node.LastChild.FirstChild.Data
	lastPageNumRawData = strings.Replace(lastPageNumRawData, "[", "", 1)
	lastPageNumRawData = strings.Replace(lastPageNumRawData, "]", "", 1)
	lastPageNum, err := strconv.Atoi(lastPageNumRawData)
	if err != nil {
		return 0, errors.Wrap(err, "covert string to int failed")
	}
	return lastPageNum, nil
}
