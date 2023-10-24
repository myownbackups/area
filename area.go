package area

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/gospider007/kinds"
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
	main_url := "http://www.tcmap.com.cn/list/jiancheng_list.html"
	session, err := requests.NewClient(pre_ctx)
	if err != nil {
		return err
	}
	main_a, _ := session.Request(pre_ctx, "GET", main_url)
	main_html := main_a.Html()
	main_trs := main_html.Find("div[id=page_left] table").Finds("tr")
	var valueNum int64
	qcData := kinds.NewSet[string]()
	Provinces := []Province{}
	for _, main_tr := range main_trs {
		province := re.Sub(`\s`, "", main_tr.Finds("td")[1].Text())
		if province == "" {
			continue
		}
		valueNum++
		var provinceData Province
		provinceData.Name = province
		provinceData.Value = valueNum
		if qcData.Has(provinceData.Name) {
			continue
		} else {
			qcData.Add(provinceData.Name)
		}
		province_url := "http://www.tcmap.com.cn" + main_tr.Find("a").Get("href")
		province_a, _ := session.Request(pre_ctx, "GET", province_url)
		province_html := province_a.Html()
		province_trs := province_html.Find("div[id=page_left]>table").Finds("tr")
		Citys := []City{}
		for _, province_tr := range province_trs {
			province_tds := province_tr.Finds("td")
			city := re.Sub(`\s`, "", province_tds[0].Text())
			if city == "" {
				continue
			}
			valueNum++
			var cityData City
			cityData.Name = city
			cityData.Value = valueNum
			if qcData.Has(provinceData.Name + cityData.Name) {
				continue
			} else {
				qcData.Add(provinceData.Name + cityData.Name)
			}
			Countys := []County{}
			for _, county := range strings.Split(province_tds[len(province_tds)-1].Text(), " ") {
				county = re.Sub(`\s`, "", county)
				if county == "" {
					continue
				}
				valueNum++
				var countyData County
				countyData.Name = county
				countyData.Value = valueNum
				if qcData.Has(provinceData.Name + cityData.Name + countyData.Name) {
					continue
				} else {
					qcData.Add(provinceData.Name + cityData.Name + countyData.Name)
				}
				Countys = append(Countys, countyData)
			}
			cityData.Countys = Countys
			Citys = append(Citys, cityData)
		}
		provinceData.Citys = Citys
		Provinces = append(Provinces, provinceData)
	}
	content, err := json.Marshal(Provinces)
	if err != nil {
		return err
	}
	return os.WriteFile(file_name, content, 0777)
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

// 创建根据映射关系创建默认的客户端
func NewClient(options ...ClientOption) *Client {
	if len(options) > 0 {
		return newClient(options[0])
	}
	var data []Province
	err := json.Unmarshal(areaContent, &data)
	if err != nil {
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
						Countysize:   countyCount,

						subProvinceSize: provinceCount2,
						subCitySize:     cityCount2,
						subCountysize:   countyCount2,
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

	subProvince string //省
	subCity     string //市
	subCounty   string //县

	ProvinceValue any
	CityValue     any
	CountyValue   any

	provinceSize int
	citySize     int
	Countysize   int

	subProvinceSize int
	subCitySize     int
	subCountysize   int
}

func (obj Node) score1() int {
	if obj.provinceSize > 0 && obj.citySize > 0 && obj.Countysize > 0 {
		return 10
	}
	if obj.subProvinceSize > 0 && obj.subCitySize > 0 && obj.subCountysize > 0 {
		return 9
	}
	if obj.provinceSize > 0 && obj.citySize > 0 {
		return 8
	}
	if obj.subProvinceSize > 0 && obj.subCitySize > 0 {
		return 7
	}
	if obj.citySize > 0 && obj.Countysize > 0 {
		return 7
	}
	if obj.subCitySize > 0 && obj.subCountysize > 0 {
		return 6
	}

	if obj.provinceSize > 0 && obj.Countysize > 0 {
		return 6
	}
	if obj.subProvinceSize > 0 && obj.subCountysize > 0 {
		return 5
	}

	if obj.provinceSize > 0 {
		return 5
	}
	if obj.citySize > 0 {
		return 4
	}
	if obj.Countysize > 0 {
		return 3
	}

	if obj.subProvinceSize > 0 {
		return 4
	}
	if obj.subCitySize > 0 {
		return 3
	}
	if obj.subCountysize > 0 {
		return 2
	}
	return 0
}
func (obj Node) score2() int {
	return obj.provinceSize*7 + obj.citySize*3 + obj.Countysize
}
func (obj Node) score3() int {
	return obj.subProvinceSize*7 + obj.subCitySize*3 + obj.subCountysize
}

// 返回所有可能
func (obj *Client) Searchs(txt string) []*Node {
	return obj.getSearchData(obj.tree.Search(re.Sub(`\s|北京时间`, "", txt)))
}

// 返回分数最大的结果
func (obj *Client) Search(txt string) *Node {
	rs := obj.Searchs(txt)
	if len(rs) > 0 {
		return rs[0]
	} else {
		return nil
	}
}
