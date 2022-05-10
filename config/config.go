package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
)

var (
	NL  = []byte{'\n'}
	ANT = []byte{'#'}
)

type Config struct {
	AuthToken    string   `json:"authToken"`    // 必选, Sam's App HTTP头部auth-token
	BarkId       string   `json:"barkId"`       // 可选，通知用的`bark` id, 可选参数
	FloorId      int      `json:"floorId"`      // 可选，1,普通商品 2,全球购保税 3,特殊订购自提 4,大件商品 5,厂家直供商品 6,特殊订购商品 7,失效商品
	DeliveryType int      `json:"deliveryType"` // 可选，1 急速达，2， 全程配送
	Longitude    string   `json:"longitude"`    // 可选，HTTP头部longitude
	Latitude     string   `json:"latitude"`     // 可选，HTTP头部latitude
	DeviceId     string   `json:"deviceId"`     // 可选，HTTP头部device-id
	TrackInfo    string   `json:"trackInfo"`    // 可选，HTTP头部track-info
	PromotionId  []string `json:"promotionId"`  // 可选，优惠券id,多个用逗号隔开，山姆app优惠券列表接口中的 'ruleId' 字段
	AddressId    string   `json:"addressId"`    // 可选，地址id
	PayMethod    int      `json:"payMethod"`    // 可选，1,微信 2,支付宝
	DeliveryFee  bool     `json:"deliveryFee"`  // 可选，是否免运费下单
	CheckGoods   bool     `json:"checkGoods"`   // 可选，是否校验商品限购
}

func (c *Config) Validate() error {
	if c.AuthToken == "" {
		return fmt.Errorf("authToken can not be empty")
	}
	return nil
}

func trimComments(data []byte) (data1 []byte) {
	confLines := bytes.Split(data, NL)
	for k, line := range confLines {
		confLines[k] = trimCommentsLine(line)
	}
	return bytes.Join(confLines, NL)
}

func trimCommentsLine(line []byte) []byte {
	var newLine []byte
	var i, quoteCount int
	lastIdx := len(line) - 1
	for i = 0; i <= lastIdx; i++ {
		if line[i] == '\\' {
			if i != lastIdx && (line[i+1] == '\\' || line[i+1] == '"') {
				newLine = append(newLine, line[i], line[i+1])
				i++
				continue
			}
		}
		if line[i] == '"' {
			quoteCount++
		}
		if line[i] == '#' {
			if quoteCount%2 == 0 {
				break
			}
		}
		newLine = append(newLine, line[i])
	}
	return newLine
}

func LoadFile(confName string) (conf Config, err error) {
	data, err := ioutil.ReadFile(confName)
	if err != nil {
		return
	}
	data = trimComments(data)

	conf = Config{}
	err = json.Unmarshal(data, &conf)
	return conf, err
}
