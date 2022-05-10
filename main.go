package main

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/robGoods/sams/config"
	"github.com/robGoods/sams/dd"
)

var (
	confName = flag.String("f", "config.json", "configuration file to load")
)

func main() {
	flag.Parse()

	conf, err := config.LoadFile(*confName)
	if err != nil {
		fmt.Printf("read config file error [%v]\n", err)
		return
	}

	if err = conf.Validate(); err != nil {
		fmt.Printf("validate config file error [%v]\n", err)
		return
	}

	run(conf)
}

func run(conf config.Config) {
	session := dd.SamsSession{
		SettleDeliveryInfo: map[int]dd.SettleDeliveryInfo{},
		StoreList:          map[string]dd.Store{},
	}
	err := session.InitSession(conf)
	if err != nil {
		fmt.Println(err)
		return
	}

	loopCount := int64(0)
	for true {
	SaveDeliveryAddress:
		fmt.Println("########## 切换购物车收货地址 ###########")
		err = session.SaveDeliveryAddress()
		if err != nil {
			goto SaveDeliveryAddress
		} else {
			fmt.Println("切换成功!")
			fmt.Printf("%s %s %s %s %s \n", session.Address.Name, session.Address.DistrictName, session.Address.ReceiverAddress, session.Address.DetailAddress, session.Address.Mobile)
		}
	StoreLoop:
		fmt.Println("########## 获取地址附近可用商店 ###########")
		stores, err := session.CheckStore()
		if err != nil {
			fmt.Printf("%s", err)
			goto StoreLoop
		}

		session.StoreList = make(map[string]dd.Store)
		for index, store := range stores {
			if _, ok := session.StoreList[store.StoreId]; !ok {
				session.StoreList[store.StoreId] = store
				fmt.Printf("[%v] Id：%s 名称：%s, 类型 ：%s\n", index, store.StoreId, store.StoreName, store.StoreType)
			}
		}
	CartLoop:
		fmt.Printf("########## 获取购物车中有效商品【%s】 ###########\n", time.Now().Format("15:04:05"))
		err = session.CheckCart()
		if err != nil {
			fmt.Printf("%s", err)
			goto StoreLoop
		}
		for _, v := range session.Cart.FloorInfoList {
			if v.FloorId == session.Conf.FloorId {
				session.GoodsList = make([]dd.Goods, 0)
				for _, goods := range v.NormalGoodsList {
					if goods.StockQuantity > 0 && goods.StockStatus && goods.IsPutOnSale && goods.IsAvailable {
						if goods.StockQuantity <= goods.Quantity {
							goods.Quantity = goods.StockQuantity
						}
						if session.Conf.CheckGoods && goods.LimitNum > 0 && goods.Quantity > goods.LimitNum {
							goods.Quantity = goods.LimitNum
						}
						session.GoodsList = append(session.GoodsList, goods.ToGoods())
					}
				}

				for _, goods := range v.ShortageStockGoodsList {
					if goods.StockQuantity > 0 && goods.StockStatus && goods.IsPutOnSale && goods.IsAvailable {
						if goods.StockQuantity <= goods.Quantity {
							goods.Quantity = goods.StockQuantity
						}
						if session.Conf.CheckGoods && goods.LimitNum > 0 && goods.Quantity > goods.LimitNum {
							goods.Quantity = goods.LimitNum
						}
						session.GoodsList = append(session.GoodsList, goods.ToGoods())
					}
				}

				for _, goods := range v.AllOutOfStockGoodsList {
					if goods.StockQuantity > 0 && goods.StockStatus && goods.IsPutOnSale && goods.IsAvailable {
						if goods.StockQuantity <= goods.Quantity {
							goods.Quantity = goods.StockQuantity
						}

						if session.Conf.CheckGoods && goods.LimitNum > 0 && goods.Quantity > goods.LimitNum {
							goods.Quantity = goods.LimitNum
						}

						session.GoodsList = append(session.GoodsList, goods.ToGoods())
					}
				}

				session.FloorInfo = v
				session.DeliveryInfoVO = dd.DeliveryInfoVO{
					StoreDeliveryTemplateId: v.StoreInfo.StoreDeliveryTemplateId,
					DeliveryModeId:          v.StoreInfo.DeliveryModeId,
					StoreType:               v.StoreInfo.StoreType,
				}
			} else {
				//无效商品
				//for index, goods := range v.NormalGoodsList {
				//	fmt.Printf("----[%v] %s 数量：%v 总价：%d\n", index, goods.SpuId, goods.StoreId, goods.Price/100.0)
				//}
			}
		}

		for index, goods := range session.GoodsList {
			fmt.Printf("[%v] %s 数量：%v 总价：%d\n", index, goods.GoodsName, goods.Quantity, goods.Price/100.0)
		}

		if len(session.GoodsList) == 0 {
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println("当前购物车中无有效商品 1s 后重试")
				time.Sleep(1 * time.Second)
			}
			if errors.Is(err, dd.LimitedErr1) {
				time.Sleep(1 * time.Second)
			}
			goto StoreLoop
		}
	GoodsLoop:
		fmt.Printf("########## 开始校验当前商品【%s】 ###########\n", time.Now().Format("15:04:05"))
		if session.Conf.CheckGoods {
			if err = session.CheckGoods(); err != nil {
				fmt.Println(err)
				time.Sleep(1 * time.Second)
				switch err {
				case dd.OOSErr:
					goto CartLoop
				default:
					goto GoodsLoop
				}
			}
		}
		if err = session.CheckSettleInfo(); err != nil {
			fmt.Printf("校验商品失败：%s\n", err)
			time.Sleep(1 * time.Second)
			switch err {
			case dd.CartGoodChangeErr:
				goto CartLoop
			case dd.LimitedErr:
				goto GoodsLoop
			case dd.NoMatchDeliverMode:
				goto SaveDeliveryAddress
			default:
				goto GoodsLoop
			}
		} else {
			fmt.Printf("运费： %s\n", session.SettleInfo.DeliveryFee)
			if session.Conf.DeliveryFee && session.SettleInfo.DeliveryFee != "0" {
				goto CartLoop
			}
		}
	CapacityLoop:
		fmt.Printf("########## 获取当前可用配送时间【%s】 ###########\n", time.Now().Format("15:04:05"))
		err = session.CheckCapacity()
		if err != nil {
			fmt.Printf("获取配送时间出错: %v 500ms 后重试...\n", err)
			time.Sleep(500 * time.Millisecond)
			//刷新可用配送时间， 会出现“服务器正忙,请稍后再试”， 可以忽略。
			goto CapacityLoop
		}

		session.SettleDeliveryInfo = map[int]dd.SettleDeliveryInfo{}
		for _, caps := range session.Capacity.CapCityResponseList {
			for _, v := range caps.List {
				fmt.Printf("配送时间： %s %s - %s, 是否可用：%v\n", caps.StrDate, v.StartTime, v.EndTime, !v.TimeISFull && !v.Disabled)
				if v.TimeISFull == false && v.Disabled == false {
					session.SettleDeliveryInfo[len(session.SettleDeliveryInfo)] = dd.SettleDeliveryInfo{
						ArrivalTimeStr:       fmt.Sprintf("%s %s - %s", caps.StrDate, v.StartTime, v.EndTime),
						ExpectArrivalTime:    v.StartRealTime,
						ExpectArrivalEndTime: v.EndRealTime,
					}
				}
			}
		}

		if len(session.SettleDeliveryInfo) > 0 {
			for _, v := range session.SettleDeliveryInfo {
				fmt.Printf("发现可用的配送时段::%s!\n", v.ArrivalTimeStr)
			}
		} else {
			loopCount++
			if loopCount%600 == 0 {
				fmt.Println("当前无可用配送时间段, 刷新商店信息后重试...")
				goto StoreLoop
			}
			fmt.Println("当前无可用配送时间段 1s 后重试...")
			time.Sleep(1 * time.Second)
			goto CapacityLoop
		}
	OrderLoop:
		for len(session.SettleDeliveryInfo) > 0 {
			for k, v := range session.SettleDeliveryInfo {
				fmt.Printf("########## 提交订单中【%s】 ###########\n", time.Now().Format("15:04:05"))
				fmt.Printf("配送时段: %s!\n", v.ArrivalTimeStr)
				err = session.CommitPay(v)
				if err == nil {
					fmt.Println("抢购成功，请前往app付款！")
					if session.Conf.BarkId != "" {
						for true {
							err = session.PushSuccess(fmt.Sprintf("Smas抢单成功，订单号：%s", session.OrderInfo.OrderNo))
							if err == nil {
								break
							} else {
								fmt.Println(err)
							}
							time.Sleep(1 * time.Second)
						}
					}
					return
				} else {
					fmt.Printf("下单失败：%s\n", err)
					switch err {
					case dd.LimitedErr1:
						fmt.Println("立即重试...")
						goto OrderLoop
					case dd.OOSErr, dd.PreGoodNotStartSellErr, dd.CartGoodChangeErr, dd.GoodsExceedLimitErr:
						goto CartLoop
					case dd.StoreHasClosedError:
						goto StoreLoop
					case dd.CloseOrderTimeExceptionErr, dd.DecreaseCapacityCountError, dd.NotDeliverCapCityErr:
						delete(session.SettleDeliveryInfo, k)
					default:
						goto StoreLoop
					}
				}
			}
		}
		goto CapacityLoop
	}
}
