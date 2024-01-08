package area

import (
	"context"
	_ "embed"
	"os"
	"sort"
	"strings"

	"github.com/gospider007/gson"
	"github.com/gospider007/re"
	"github.com/gospider007/requests"
	"github.com/gospider007/tree"
)

func subProvince(province string) string {
	province = re.Sub(`市|省|\s`, "", province)
	if strings.Contains(province, "香港") {
		province = "香港"
	}
	if strings.Contains(province, "蒙古") {
		province = "蒙古"
	}
	if strings.Contains(province, "新疆") {
		province = "新疆"
	}
	if strings.Contains(province, "广西") {
		province = "广西"
	}
	if strings.Contains(province, "西藏") {
		province = "西藏"
	}
	if strings.Contains(province, "宁夏") {
		province = "宁夏"
	}
	if strings.Contains(province, "澳门") {
		province = "澳门"
	}
	return province
}
func subCity(city string) string {
	city = re.Sub(`\s`, "", city)
	city = re.Sub("(.{2,})[市县州旗镇乡岛]$", "${1}", city)
	return city
}
func subCounty(county string) string {
	county = re.Sub(`\s`, "", county)
	county = re.Sub("(.{2,})新区$", "${1}", county)
	county = re.Sub("(.{2,})[区市县州旗镇乡岛]$", "${1}", county)
	county = re.Sub("(.{2,})?自治.*", "${1}", county)
	county = re.Sub(`[\(（].+?[\)）]$`, "", county)
	return county
}

type Province struct {
	Name    string `json:"name"`
	Value   any    `json:"value"`
	subName string
	Citys   []City `json:"citys"`
}
type City struct {
	Name    string `json:"name"`
	subName string
	Value   any      `json:"value"`
	Countys []County `json:"countys"`
}
type County struct {
	Name    string `json:"name"`
	subName string
	Value   any `json:"value"`
}

func SaveAreaData(pre_ctx context.Context, file_name string) error {
	resp, err := requests.Get(pre_ctx, "https://geo.datav.aliyun.com/areas_v3/bound/all.json")
	if err != nil {
		return err
	}
	if _, err = resp.Json(); err != nil {
		return err
	}
	return os.WriteFile(file_name, resp.Content(), 0777)
}

type Client struct {
	tree   *tree.Client
	option ClientOption
}

//go:embed areaCode.json
var areaContent []byte

type ClientOption struct {
	Datas       []Province
	SubProvince bool
	SubCity     bool
	SubCounty   bool
}

// 根据映射表创建客户端
func newClient(option ClientOption) *Client {
	city_tree := tree.NewClient()
	for provinceIndex, province := range option.Datas {
		city_tree.Add(province.Name)
		if option.SubProvince {
			province2 := subProvince(province.Name)
			if province2 != province.Name && province2 != "" {
				city_tree.Add(province2)
				option.Datas[provinceIndex].subName = province2
			}
		}
		for cityIndex, city := range province.Citys {
			city_tree.Add(city.Name)
			if option.SubCity {
				city2 := subCity(city.Name)
				if city2 != city.Name && city2 != "" {
					city_tree.Add(city2)
					option.Datas[provinceIndex].Citys[cityIndex].subName = city2
				}
			}
			for countyIndex, county := range city.Countys {
				city_tree.Add(county.Name)
				if option.SubCounty {
					county2 := subCounty(county.Name)
					if county2 != county.Name && county2 != "" {
						city_tree.Add(county2)
						option.Datas[provinceIndex].Citys[cityIndex].Countys[countyIndex].subName = county2
					}
				}
			}
		}
	}
	return &Client{option: option, tree: city_tree}
}

type adcode struct {
	name   string
	adcode int64
	parent int64
}

func getDefaultArea() []Province {
	jsonData, err := gson.Decode(areaContent)
	if err != nil {
		return nil
	}
	provinces := []adcode{}
	citys := []adcode{}
	xians := []adcode{}
	for _, ll := range jsonData.Array() {
		level := ll.Get("level").String()
		switch level {
		case "province":
			provinces = append(provinces, adcode{
				name:   ll.Get("name").String(),
				adcode: ll.Get("adcode").Int(),
				parent: ll.Get("parent").Int(),
			})
		case "city":
			citys = append(citys, adcode{
				name:   ll.Get("name").String(),
				adcode: ll.Get("adcode").Int(),
				parent: ll.Get("parent").Int(),
			})
		case "district":
			xians = append(xians, adcode{
				name:   ll.Get("name").String(),
				adcode: ll.Get("adcode").Int(),
				parent: ll.Get("parent").Int(),
			})

		}
	}
	results := []Province{}
	for _, province := range provinces {
		p := Province{
			Name:  province.name,
			Value: province.adcode,
			Citys: []City{},
		}
		for _, city := range citys {
			if city.parent == province.adcode {
				c := City{
					Name:    city.name,
					Value:   city.adcode,
					Countys: []County{},
				}
				for _, xian := range xians {
					if xian.parent == city.adcode {
						c.Countys = append(c.Countys, County{
							Name:  xian.name,
							Value: xian.adcode,
						})
					}
				}
				p.Citys = append(p.Citys, c)
			}
		}
		results = append(results, p)
	}
	return results
}

// 创建根据映射关系创建默认的客户端
func NewClient(options ...ClientOption) *Client {
	if len(options) > 0 {
		return newClient(options[0])
	}
	data := getDefaultArea()
	if data == nil {
		return nil
	}
	return newClient(ClientOption{
		Datas:       data,
		SubProvince: true,
		SubCity:     true,
		SubCounty:   true,
	})
}

func (obj *Client) getSearchData(searchData map[string]int) []*Node {
	if searchData == nil {
		return nil
	}
	results := []*Node{}
	for _, province := range obj.option.Datas {
		provinceCount := searchData[province.Name]
		provinceCount2 := searchData[province.subName]
		var haveCity bool
		var haveCounty2 bool
		for _, city := range province.Citys {
			cityCount := searchData[city.Name]
			cityCount2 := searchData[city.subName]
			if city.Name == province.Name {
				cityCount = 0
				cityCount, provinceCount = provinceCount, cityCount
			}
			if city.subName == province.subName {
				cityCount2 = 0
				cityCount2, provinceCount2 = provinceCount2, cityCount2
			}
			var haveCounty bool
			for _, county := range city.Countys {
				countyCount := searchData[county.Name]
				countyCount2 := searchData[county.subName]
				if county.Name == city.Name {
					countyCount = 0
					cityCount, countyCount = countyCount, cityCount
				}
				if county.subName == city.subName {
					countyCount2 = 0
					cityCount2, countyCount2 = countyCount2, cityCount2
				}
				if countyCount+countyCount2 > 0 {
					haveCounty = true
					haveCounty2 = true
					results = append(results, &Node{
						Province: province.Name,
						City:     city.Name,
						County:   county.Name,

						subProvince: province.subName,
						subCity:     city.subName,
						subCounty:   county.subName,

						ProvinceValue: province.Value,
						CityValue:     city.Value,
						CountyValue:   county.Value,

						provinceSize: provinceCount,
						citySize:     cityCount,
						countySize:   countyCount,

						subProvinceSize: provinceCount2,
						subCitySize:     cityCount2,
						subCountySize:   countyCount2,
					})
				}
			}
			if !haveCounty && cityCount+cityCount2 > 0 {
				haveCity = true
				results = append(results, &Node{
					Province: province.Name,
					City:     city.Name,

					subProvince: province.subName,
					subCity:     city.subName,

					ProvinceValue: province.Value,
					CityValue:     city.Value,

					provinceSize: provinceCount,
					citySize:     cityCount,

					subProvinceSize: provinceCount2,
					subCitySize:     cityCount2,
				})

			}
		}
		if !haveCity && !haveCounty2 && provinceCount+provinceCount2 > 0 {
			haveCity = true
			results = append(results, &Node{
				subProvince: province.subName,
				Province:    province.Name,

				ProvinceValue:   province.Value,
				provinceSize:    provinceCount,
				subProvinceSize: provinceCount2,
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		node := results[i]
		node2 := results[j]

		socre := node.score1()
		socre2 := node2.score1()
		if socre == socre2 {
			socre = node.score2()
			socre2 = node2.score2()
			if socre == socre2 {
				socre = node.score3()
				socre2 = node2.score3()
				if socre == socre2 {
					socre = node.score4()
					socre2 = node2.score4()
				}
			}
		}
		return socre > socre2
	})
	return results
}

type Node struct {
	Province string //省
	City     string //市
	County   string //县

	ProvinceValue any
	CityValue     any
	CountyValue   any

	subProvince string //省
	subCity     string //市
	subCounty   string //县

	provinceSize int
	citySize     int
	countySize   int

	subProvinceSize int
	subCitySize     int
	subCountySize   int
}

func (obj Node) score1() int {
	if obj.provinceSize > 0 && obj.citySize > 0 && obj.countySize > 0 {
		return 10
	}
	if obj.subProvinceSize > 0 && obj.subCitySize > 0 && obj.subCountySize > 0 {
		return 9
	}
	if obj.provinceSize > 0 && obj.citySize > 0 {
		return 8
	}
	if obj.subProvinceSize > 0 && obj.subCitySize > 0 {
		return 7
	}
	if obj.citySize > 0 && obj.countySize > 0 {
		return 7
	}
	if obj.subCitySize > 0 && obj.subCountySize > 0 {
		return 6
	}

	if obj.provinceSize > 0 && obj.countySize > 0 {
		return 6
	}
	if obj.subProvinceSize > 0 && obj.subCountySize > 0 {
		return 5
	}

	if obj.provinceSize > 0 {
		return 5
	}
	if obj.citySize > 0 {
		return 4
	}
	if obj.countySize > 0 {
		return 3
	}

	if obj.subProvinceSize > 0 {
		return 4
	}
	if obj.subCitySize > 0 {
		return 3
	}
	if obj.subCountySize > 0 {
		return 2
	}
	return 0
}
func (obj Node) score2() int {
	return obj.provinceSize*7 + obj.citySize*3 + obj.countySize
}
func (obj Node) score3() int {
	return obj.subProvinceSize*7 + obj.subCitySize*3 + obj.subCountySize
}
func (obj Node) score4() int {
	score := 0
	if strings.HasSuffix(obj.Province, "省") {
		score += 24
	} else if strings.HasSuffix(obj.Province, "市") {
		score += 12
	} else if strings.HasSuffix(obj.Province, "县") {
		score += 6
	} else if strings.HasSuffix(obj.Province, "区") {
		score += 2
	}
	if strings.HasSuffix(obj.City, "省") {
		score += 24
	} else if strings.HasSuffix(obj.City, "市") {
		score += 12
	} else if strings.HasSuffix(obj.City, "县") {
		score += 6
	} else if strings.HasSuffix(obj.City, "区") {
		score += 2
	}
	if strings.HasSuffix(obj.County, "省") {
		score += 24
	} else if strings.HasSuffix(obj.County, "市") {
		score += 12
	} else if strings.HasSuffix(obj.County, "县") {
		score += 6
	} else if strings.HasSuffix(obj.County, "区") {
		score += 2
	}
	return score
}

// 返回所有可能
func (obj *Client) Searchs(txt string) []*Node {
	return obj.getSearchData(obj.tree.Search(re.Sub(`\s|北京时间`, "", txt)))
}

// 返回分数最大的结果
func (obj *Client) Search(txts ...string) *Node {
	if len(txts) == 0 {
		return nil
	}
	if len(txts) == 1 {
		rs := obj.Searchs(txts[0])
		if len(rs) > 0 {
			return rs[0]
		} else {
			return nil
		}
	}
	var mustNode *Node
	allTxt := ""
	for _, txt := range txts {
		allTxt += txt
		nodes := obj.Searchs(allTxt)
		if len(nodes) > 0 {
			if mustNode == nil {
				mustNode = nodes[0]
			} else {
				for _, node := range nodes {
					if node.Province == mustNode.Province {
						mustNode = node
						break
					}
				}
			}
			if mustNode.County != "" {
				return mustNode
			}
		}
	}
	return mustNode
}
